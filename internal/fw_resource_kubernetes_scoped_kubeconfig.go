package internal

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

const scopedKubeconfigIDFormat = "<environment_id>:<scoped_kubeconfig_id>"

// scopedKubeconfigMaxExpiry is the longest lifetime Portainer issues a scoped
// kubeconfig for: 7 days.
const scopedKubeconfigMaxExpiry = 7 * 24 * 60 * 60

// scopedKubeconfigStatus is kubernetes.scopedKubeconfigStatusResponse, and
// with UserID/Username/EndpointID also kubernetes.scopedKubeconfigListItem.
type scopedKubeconfigStatus struct {
	ID             int      `json:"id"`
	EndpointID     int      `json:"endpointId"`
	UserID         int      `json:"userId"`
	Username       string   `json:"username"`
	Description    string   `json:"description"`
	Namespaces     []string `json:"namespaces"`
	IncludeSecrets bool     `json:"includeSecrets"`
	ServiceAccount string   `json:"serviceAccount"`
	Active         bool     `json:"active"`
	CreatedAt      int      `json:"createdAt"`
	ExpiresAt      int      `json:"expiresAt"`
}

var (
	_ resource.Resource                = (*kubernetesScopedKubeconfigResource)(nil)
	_ resource.ResourceWithConfigure   = (*kubernetesScopedKubeconfigResource)(nil)
	_ resource.ResourceWithImportState = (*kubernetesScopedKubeconfigResource)(nil)
)

// kubernetesScopedKubeconfigResource issues a read-only kubeconfig limited to
// some namespaces of one environment. Portainer has no update for it, so
// every argument forces a new one; a scoped kubeconfig that expired is dropped
// from state on refresh, so the next apply issues a fresh one.
//
// The credential is returned once, by the issuing call, and never again: it is
// kept in state as a sensitive attribute, since a Terraform output is the only
// way to hand it on. A write-only attribute would not help, as it is the
// provider, not the user, that receives the value.
func newKubernetesScopedKubeconfigResource() resource.Resource {
	return &kubernetesScopedKubeconfigResource{}
}

type kubernetesScopedKubeconfigResource struct{ k8sfwBase }

type kubernetesScopedKubeconfigModel struct {
	ID                 types.String `tfsdk:"id"`
	EnvironmentID      types.Int64  `tfsdk:"environment_id"`
	ExpirySeconds      types.Int64  `tfsdk:"expiry_seconds"`
	Namespaces         types.List   `tfsdk:"namespaces"`
	IncludeSecrets     types.Bool   `tfsdk:"include_secrets"`
	Description        types.String `tfsdk:"description"`
	ScopedKubeconfigID types.Int64  `tfsdk:"scoped_kubeconfig_id"`
	Kubeconfig         types.String `tfsdk:"kubeconfig"`
	ServiceAccount     types.String `tfsdk:"service_account"`
	CoveredNamespaces  types.List   `tfsdk:"covered_namespaces"`
	Active             types.Bool   `tfsdk:"active"`
	CreatedAt          types.Int64  `tfsdk:"created_at"`
	ExpiresAt          types.Int64  `tfsdk:"expires_at"`
}

func (r *kubernetesScopedKubeconfigResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_scoped_kubeconfig"
}

func (r *kubernetesScopedKubeconfigResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keepString := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	keepInt := []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the scoped kubeconfig, `" + scopedKubeconfigIDFormat + "`.",
				PlanModifiers: keepString,
			},
			"environment_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the Portainer Kubernetes environment the kubeconfig gives access to.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"expiry_seconds": schema.Int64Attribute{
				Required:      true,
				Description:   "Lifetime of the kubeconfig in seconds, at most 604800 (7 days). Once it has expired, the next plan issues a new one.",
				Validators:    []validator.Int64{int64validator.Between(1, scopedKubeconfigMaxExpiry)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"namespaces": schema.ListAttribute{
				ElementType:   types.StringType,
				Optional:      true,
				Description:   "Namespaces the kubeconfig can read, within the caller's own access. Leave unset for every namespace the caller can view; the `portainer` and `portainer-addon-*` namespaces are covered only when named.",
				PlanModifiers: []planmodifier.List{scopedKubeconfigNamespacesReplace()},
			},
			"include_secrets": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(false),
				Description:   "Whether the kubeconfig can read Secrets. Granted only where the caller can view secrets, and never in the `portainer` and `portainer-addon-*` namespaces.",
				PlanModifiers: []planmodifier.Bool{k8sfwKeepNull{}, boolplanmodifier.RequiresReplace()},
			},
			"description": schema.StringAttribute{
				Optional:      true,
				Description:   "Free-text description shown in Portainer's list of scoped kubeconfigs.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"scoped_kubeconfig_id": schema.Int64Attribute{
				Computed:      true,
				Description:   "Portainer identifier of the scoped kubeconfig.",
				PlanModifiers: keepInt,
			},
			"kubeconfig": schema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				Description:   "The kubeconfig document. Portainer returns it only when it is issued, so it is empty after an import.",
				PlanModifiers: keepString,
			},
			"service_account": schema.StringAttribute{
				Computed:      true,
				Description:   "Kubernetes service account the kubeconfig authenticates as.",
				PlanModifiers: keepString,
			},
			"covered_namespaces": schema.ListAttribute{
				ElementType:   types.StringType,
				Computed:      true,
				Description:   "Namespaces the kubeconfig actually covers, as reported by Portainer.",
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
			},
			"active": schema.BoolAttribute{
				Computed:      true,
				Description:   "Whether the kubeconfig still works. Always true in state: one that expired or was revoked is dropped on refresh.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"created_at": schema.Int64Attribute{
				Computed:      true,
				Description:   "Time the kubeconfig was issued, as a Unix timestamp.",
				PlanModifiers: keepInt,
			},
			"expires_at": schema.Int64Attribute{
				Computed:      true,
				Description:   "Time the kubeconfig stops working, as a Unix timestamp.",
				PlanModifiers: keepInt,
			},
		},
	}
}

// scopedKubeconfigNamespacesReplace makes a change of namespaces issue a new
// kubeconfig, except right after an import. Portainer reports only the
// namespaces a kubeconfig covers, not the ones it was issued for, so the
// import stores the covered ones: for a kubeconfig issued without namespaces
// (every namespace the caller can view) they are the list the caller could
// view then. A configuration that leaves namespaces unset, or lists the same
// namespaces in another order, then updates the state in place instead of
// replacing a credential that may well match; an unset one with a warning,
// as the import cannot tell.
func scopedKubeconfigNamespacesReplace() planmodifier.List {
	return listplanmodifier.RequiresReplaceIf(
		func(ctx context.Context, req planmodifier.ListRequest, resp *listplanmodifier.RequiresReplaceIfFuncResponse) {
			resp.RequiresReplace = true
			if req.Private == nil || !k8sImported(ctx, req.Private, &resp.Diagnostics) {
				return
			}
			if req.ConfigValue.IsNull() {
				resp.RequiresReplace = false
				resp.Diagnostics.AddAttributeWarning(req.Path, "Namespaces of an imported scoped kubeconfig not verified",
					"Portainer reports the namespaces a scoped kubeconfig covers, not whether it was issued for every namespace, so "+
						"leaving namespaces unset only updates the state. To be sure the kubeconfig covers every namespace, replace it: "+
						"terraform apply -replace=<address>.")
				return
			}
			var planned, imported []string
			resp.Diagnostics.Append(req.ConfigValue.ElementsAs(ctx, &planned, false)...)
			resp.Diagnostics.Append(req.StateValue.ElementsAs(ctx, &imported, false)...)
			resp.RequiresReplace = !scopedKubeconfigSameNamespaces(planned, imported)
		},
		"Issues a new kubeconfig when the namespaces change, except when they cannot be verified after an import.",
		"Issues a new kubeconfig when the namespaces change, except when they cannot be verified after an import.",
	)
}

// scopedKubeconfigSameNamespaces compares two namespace lists in any order.
func scopedKubeconfigSameNamespaces(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	count := map[string]int{}
	for _, ns := range a {
		count[ns]++
	}
	for _, ns := range b {
		if count[ns] == 0 {
			return false
		}
		count[ns]--
	}
	return true
}

func parseScopedKubeconfigID(id string) (int, int, error) {
	parts, err := importIDParts(id, ":", 2, scopedKubeconfigIDFormat)
	if err != nil {
		return 0, 0, err
	}
	envID, err1 := strconv.Atoi(parts[0])
	kcID, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return 0, 0, fmt.Errorf("invalid ID %q: expected %s", id, scopedKubeconfigIDFormat)
	}
	return envID, kcID, nil
}

func scopedKubeconfigURL(client *APIClient, envID, kcID int) string {
	return fmt.Sprintf("%s/kubernetes/%d/scoped_kubeconfig/%d", client.Endpoint, envID, kcID)
}

// readScopedKubeconfig returns the status of a scoped kubeconfig, or nil when
// Portainer no longer knows it.
func readScopedKubeconfig(ctx context.Context, client *APIClient, envID, kcID int) (*scopedKubeconfigStatus, error) {
	var status scopedKubeconfigStatus
	if err := doJSON(ctx, client, http.MethodGet, scopedKubeconfigURL(client, envID, kcID), nil, &status); err != nil {
		if isAPINotFound(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read scoped kubeconfig %d: %w", kcID, err)
	}
	return &status, nil
}

func scopedKubeconfigStrings(ctx context.Context, values []string) types.List {
	l, _ := types.ListValueFrom(ctx, types.StringType, append([]string{}, values...))
	return l
}

func (r *kubernetesScopedKubeconfigResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubernetesScopedKubeconfigModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	envID := int(plan.EnvironmentID.ValueInt64())

	payload := map[string]interface{}{
		"expirySeconds":  int(plan.ExpirySeconds.ValueInt64()),
		"includeSecrets": plan.IncludeSecrets.ValueBool(),
		"description":    plan.Description.ValueString(),
	}
	var namespaces []string
	if !plan.Namespaces.IsNull() && !plan.Namespaces.IsUnknown() {
		resp.Diagnostics.Append(plan.Namespaces.ElementsAs(ctx, &namespaces, false)...)
	}
	if len(namespaces) > 0 {
		payload["namespaces"] = namespaces
	}

	var result struct {
		ID             int      `json:"id"`
		Kubeconfig     string   `json:"kubeconfig"`
		ServiceAccount string   `json:"serviceAccount"`
		Namespaces     []string `json:"namespaces"`
		ExpiresAt      int      `json:"expiresAt"`
	}
	url := fmt.Sprintf("%s/kubernetes/%d/scoped_kubeconfig", r.client.Endpoint, envID)
	if err := doJSON(ctx, r.client, http.MethodPost, url, payload, &result); err != nil {
		err = explainUnsupported(ctx, r.client, err, "portainer_kubernetes_scoped_kubeconfig", requiresBE2452)
		resp.Diagnostics.AddError("Failed to issue a scoped kubeconfig", fmt.Errorf("failed to issue a scoped kubeconfig: %w", err).Error())
		return
	}
	if result.ID == 0 {
		resp.Diagnostics.AddError("Failed to issue a scoped kubeconfig", "failed to issue a scoped kubeconfig: Portainer returned no identifier")
		return
	}

	// The credential is never returned again, so it is stored before the read.
	plan.ID = types.StringValue(fmt.Sprintf("%d:%d", envID, result.ID))
	plan.ScopedKubeconfigID = types.Int64Value(int64(result.ID))
	plan.Kubeconfig = types.StringValue(result.Kubeconfig)
	plan.ServiceAccount = types.StringValue(result.ServiceAccount)
	plan.CoveredNamespaces = scopedKubeconfigStrings(ctx, result.Namespaces)
	plan.ExpiresAt = types.Int64Value(int64(result.ExpiresAt))
	plan.Active = types.BoolValue(true)
	plan.CreatedAt = types.Int64Value(0)
	// Saved first, so the credential is not lost when the read fails.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The configured arguments are what the apply stores; a refresh reports
	// any difference.
	includeSecrets, description := plan.IncludeSecrets, plan.Description
	found, err := r.read(ctx, &plan, false)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the scoped kubeconfig", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to read the scoped kubeconfig", fmt.Sprintf("scoped kubeconfig %d is gone or no longer active right after it was issued", result.ID))
		return
	}
	plan.IncludeSecrets, plan.Description = includeSecrets, description
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

// read refreshes m, reporting false when the kubeconfig is gone or no longer
// works. fromSDK says whether m is a state SDKv2 wrote.
func (r *kubernetesScopedKubeconfigResource) read(ctx context.Context, m *kubernetesScopedKubeconfigModel, fromSDK bool) (bool, error) {
	envID, kcID, err := parseScopedKubeconfigID(m.ID.ValueString())
	if err != nil {
		return false, err
	}
	status, err := readScopedKubeconfig(ctx, r.client, envID, kcID)
	if err != nil {
		return false, err
	}
	// An expired or revoked kubeconfig no longer works; dropping it lets the
	// next apply issue a replacement.
	if status == nil || !status.Active {
		log.Printf("[INFO] scoped kubeconfig %d on environment %d is gone or no longer active, removing it from state", kcID, envID)
		return false, nil
	}

	m.EnvironmentID = types.Int64Value(int64(envID))
	m.ScopedKubeconfigID = types.Int64Value(int64(status.ID))
	m.IncludeSecrets = types.BoolValue(status.IncludeSecrets)
	// description is optional: an empty one stays unset when it was. SDKv2
	// stored it as "" when unset, so in a state it wrote "" counts as unset.
	switch {
	case status.Description != "":
		m.Description = types.StringValue(status.Description)
	case m.Description.IsNull() || (fromSDK && m.Description.ValueString() == ""):
		m.Description = types.StringNull()
	default:
		m.Description = types.StringValue("")
	}
	m.ServiceAccount = types.StringValue(status.ServiceAccount)
	m.CoveredNamespaces = scopedKubeconfigStrings(ctx, status.Namespaces)
	m.Active = types.BoolValue(status.Active)
	m.CreatedAt = types.Int64Value(int64(status.CreatedAt))
	m.ExpiresAt = types.Int64Value(int64(status.ExpiresAt))
	if m.Kubeconfig.IsNull() || m.Kubeconfig.IsUnknown() {
		m.Kubeconfig = types.StringValue("")
	}
	return true, nil
}

func (r *kubernetesScopedKubeconfigResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state kubernetesScopedKubeconfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	fromSDK := k8sfwFromSDK(ctx, req.Private, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	// ImportState leaves kubeconfig unset and the first Read fills it in, so
	// an unset kubeconfig tells the Read that completes an import.
	importRead := state.Kubeconfig.IsNull()
	found, err := r.read(ctx, &state, fromSDK)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the scoped kubeconfig", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
	scopedKubeconfigAgeImportMark(ctx, req.Private, resp.Private, importRead, &resp.Diagnostics)
}

// The import mark (k8sImportedKey) lets the first plan after an import treat
// a namespaces change as an in-place update (scopedKubeconfigNamespacesReplace).
// An update drops it, but a plan without changes never reaches the provider
// again, so Read ages it instead: the Read completing the import keeps it, the
// next Read - the refresh of the first plan after the import - marks it
// expiring, which that plan still honours, and the Read after that drops it.
// A `terraform import` followed by a plan and apply that change nothing thus
// leaves no mark behind, and a later change of namespaces replaces the
// kubeconfig as usual. (With an import block, whose own run already plans,
// the mark lasts for one more run.)
const scopedKubeconfigImportExpiring = `"expiring"`

func scopedKubeconfigAgeImportMark(ctx context.Context, prior k8sfwPrivateReader, private k8sfwPrivateWriter, importRead bool, diags *diag.Diagnostics) {
	v, d := prior.GetKey(ctx, k8sImportedKey)
	diags.Append(d...)
	switch {
	case len(v) == 0 || importRead:
	case string(v) == scopedKubeconfigImportExpiring:
		k8sClearImported(ctx, private, diags)
	default:
		diags.Append(private.SetKey(ctx, k8sImportedKey, []byte(scopedKubeconfigImportExpiring))...)
	}
}

// Update only stores the plan: every argument forces a new kubeconfig, but
// for namespaces right after an import (see
// scopedKubeconfigNamespacesReplace). The state then holds the configuration,
// so the import mark is dropped.
func (r *kubernetesScopedKubeconfigResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	k8sfwKeepState(ctx, req, resp)
	k8sClearImported(ctx, resp.Private, &resp.Diagnostics)
}

// Delete revokes the kubeconfig. Portainer answers 500 when the credential
// was revoked but its service account and role bindings could not be removed
// from the cluster: the credential already stopped working, so that is
// reported as a warning, not a failed destroy.
func (r *kubernetesScopedKubeconfigResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state kubernetesScopedKubeconfigModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	envID, kcID, err := parseScopedKubeconfigID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid resource ID", err.Error())
		return
	}

	err = doJSON(ctx, r.client, http.MethodDelete, scopedKubeconfigURL(r.client, envID, kcID), nil, nil)
	switch {
	case err == nil || isAPINotFound(err):
	case isAPIStatus(err, http.StatusInternalServerError):
		resp.Diagnostics.AddWarning(
			fmt.Sprintf("Scoped kubeconfig %d was revoked, but its Kubernetes objects could not be removed", kcID),
			fmt.Sprintf("The credential no longer works. Its service account and role bindings stay in the cluster until removed by hand: %v", err),
		)
	default:
		resp.Diagnostics.AddError("Failed to revoke the scoped kubeconfig", fmt.Errorf("failed to revoke scoped kubeconfig %d: %w", kcID, err).Error())
	}
}

// ImportState takes "<environment_id>:<scoped_kubeconfig_id>" and fills the
// arguments Read does not refresh from the issued kubeconfig, so the plan
// after an import matches a configuration written to match it. The credential
// itself cannot be recovered.
func (r *kubernetesScopedKubeconfigResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	envID, kcID, err := parseScopedKubeconfigID(req.ID)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	status, err := readScopedKubeconfig(ctx, r.client, envID, kcID)
	if err != nil {
		resp.Diagnostics.AddError("Failed to import the scoped kubeconfig", err.Error())
		return
	}
	if status == nil {
		resp.Diagnostics.AddError("Failed to import the scoped kubeconfig", fmt.Sprintf("scoped kubeconfig %d not found on environment %d", kcID, envID))
		return
	}
	namespaces := types.ListNull(types.StringType)
	if len(status.Namespaces) > 0 {
		namespaces = scopedKubeconfigStrings(ctx, status.Namespaces)
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), int64(envID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("namespaces"), namespaces)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("expiry_seconds"), int64(status.ExpiresAt-status.CreatedAt))...)
	k8sMarkImported(ctx, resp.Private, &resp.Diagnostics)
}
