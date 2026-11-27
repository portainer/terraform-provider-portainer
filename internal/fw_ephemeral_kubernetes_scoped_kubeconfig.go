package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral"
	"github.com/hashicorp/terraform-plugin-framework/ephemeral/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ ephemeral.EphemeralResource              = (*scopedKubeconfigEphemeral)(nil)
	_ ephemeral.EphemeralResourceWithConfigure = (*scopedKubeconfigEphemeral)(nil)
	_ ephemeral.EphemeralResourceWithClose     = (*scopedKubeconfigEphemeral)(nil)
)

// scopedKubeconfigEphemeral issues a scoped kubeconfig for one Terraform run
// and revokes it in Close, which also removes the service account and role
// bindings Portainer created for it. The credential reaches neither the plan
// nor the state. Renew is not implemented: the lifetime is chosen up front
// with expiry_seconds and Portainer cannot extend it.
func newScopedKubeconfigEphemeral() ephemeral.EphemeralResource {
	return &scopedKubeconfigEphemeral{}
}

type scopedKubeconfigEphemeral struct{ ephemeralBase }

type scopedKubeconfigEphemeralModel struct {
	EnvironmentID      types.Int64  `tfsdk:"environment_id"`
	ExpirySeconds      types.Int64  `tfsdk:"expiry_seconds"`
	Namespaces         types.List   `tfsdk:"namespaces"`
	IncludeSecrets     types.Bool   `tfsdk:"include_secrets"`
	Description        types.String `tfsdk:"description"`
	ScopedKubeconfigID types.Int64  `tfsdk:"scoped_kubeconfig_id"`
	Kubeconfig         types.String `tfsdk:"kubeconfig"`
	ServiceAccount     types.String `tfsdk:"service_account"`
	CoveredNamespaces  types.List   `tfsdk:"covered_namespaces"`
	ExpiresAt          types.Int64  `tfsdk:"expires_at"`
}

// scopedKubeconfigClose is what Close needs to revoke the kubeconfig.
type scopedKubeconfigClose struct {
	EnvironmentID int `json:"environment_id"`
	KubeconfigID  int `json:"kubeconfig_id"`
}

func (e *scopedKubeconfigEphemeral) Metadata(_ context.Context, req ephemeral.MetadataRequest, resp *ephemeral.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_scoped_kubeconfig"
}

func (e *scopedKubeconfigEphemeral) Schema(_ context.Context, _ ephemeral.SchemaRequest, resp *ephemeral.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Issues a read-only kubeconfig scoped to some namespaces of one environment for the duration of one Terraform run, and revokes it when the run ends. Requires Portainer Business Edition 2.45.2 or later.",
		Attributes: map[string]schema.Attribute{
			"environment_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the Portainer Kubernetes environment the kubeconfig gives access to.",
			},
			"expiry_seconds": schema.Int64Attribute{
				Required:    true,
				Description: "Lifetime of the kubeconfig in seconds, at most 604800 (7 days). It is revoked at the end of the run anyway; the expiry bounds its life should the revocation fail.",
				Validators:  []validator.Int64{int64validator.Between(1, scopedKubeconfigMaxExpiry)},
			},
			"namespaces": schema.ListAttribute{
				ElementType: types.StringType,
				Optional:    true,
				Description: "Namespaces the kubeconfig can read, within the caller's own access. Leave unset for every namespace the caller can view; the `portainer` and `portainer-addon-*` namespaces are covered only when named.",
			},
			"include_secrets": schema.BoolAttribute{
				Optional:    true,
				Description: "Whether the kubeconfig can read Secrets. Defaults to false. Granted only where the caller can view secrets, and never in the `portainer` and `portainer-addon-*` namespaces.",
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Free-text description shown in Portainer's list of scoped kubeconfigs while the kubeconfig exists.",
			},
			"scoped_kubeconfig_id": schema.Int64Attribute{
				Computed:    true,
				Description: "Portainer identifier of the scoped kubeconfig.",
			},
			"kubeconfig": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "The kubeconfig document.",
			},
			"service_account": schema.StringAttribute{
				Computed:    true,
				Description: "Kubernetes service account the kubeconfig authenticates as.",
			},
			"covered_namespaces": schema.ListAttribute{
				ElementType: types.StringType,
				Computed:    true,
				Description: "Namespaces the kubeconfig actually covers, as reported by Portainer.",
			},
			"expires_at": schema.Int64Attribute{
				Computed:    true,
				Description: "Time the kubeconfig stops working, as a Unix timestamp.",
			},
		},
	}
}

func (e *scopedKubeconfigEphemeral) Open(ctx context.Context, req ephemeral.OpenRequest, resp *ephemeral.OpenResponse) {
	var cfg scopedKubeconfigEphemeralModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &cfg)...)
	client := e.ephemeralClient(&resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	envID := int(cfg.EnvironmentID.ValueInt64())

	// The same request portainer_kubernetes_scoped_kubeconfig sends.
	payload := map[string]interface{}{
		"expirySeconds":  int(cfg.ExpirySeconds.ValueInt64()),
		"includeSecrets": cfg.IncludeSecrets.ValueBool(),
		"description":    cfg.Description.ValueString(),
	}
	var namespaces []string
	if !cfg.Namespaces.IsNull() && !cfg.Namespaces.IsUnknown() {
		resp.Diagnostics.Append(cfg.Namespaces.ElementsAs(ctx, &namespaces, false)...)
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
	url := fmt.Sprintf("%s/kubernetes/%d/scoped_kubeconfig", client.Endpoint, envID)
	if err := doJSON(ctx, client, http.MethodPost, url, payload, &result); err != nil {
		err = explainUnsupported(ctx, client, err, "ephemeral portainer_kubernetes_scoped_kubeconfig", requiresBE2452)
		resp.Diagnostics.AddError("Failed to issue a scoped kubeconfig", fmt.Errorf("failed to issue a scoped kubeconfig: %w", err).Error())
		return
	}
	if result.ID == 0 {
		resp.Diagnostics.AddError("Failed to issue a scoped kubeconfig", "failed to issue a scoped kubeconfig: Portainer returned no identifier")
		return
	}

	setEphemeralPrivate(ctx, resp.Private, scopedKubeconfigClose{EnvironmentID: envID, KubeconfigID: result.ID}, &resp.Diagnostics)
	cfg.ScopedKubeconfigID = types.Int64Value(int64(result.ID))
	cfg.Kubeconfig = types.StringValue(result.Kubeconfig)
	cfg.ServiceAccount = types.StringValue(result.ServiceAccount)
	cfg.CoveredNamespaces = scopedKubeconfigStrings(ctx, result.Namespaces)
	cfg.ExpiresAt = types.Int64Value(int64(result.ExpiresAt))
	resp.Diagnostics.Append(resp.Result.Set(ctx, &cfg)...)
}

// Close revokes the kubeconfig. As in the resource's Delete, a 500 means the
// credential was revoked but its Kubernetes objects stayed behind, which is a
// warning rather than a failed run.
func (e *scopedKubeconfigEphemeral) Close(ctx context.Context, req ephemeral.CloseRequest, resp *ephemeral.CloseResponse) {
	var p scopedKubeconfigClose
	if !getEphemeralPrivate(ctx, req.Private, &p, &resp.Diagnostics) {
		return
	}
	client := e.ephemeralClient(&resp.Diagnostics)
	if client == nil {
		return
	}
	err := doJSON(ctx, client, http.MethodDelete, scopedKubeconfigURL(client, p.EnvironmentID, p.KubeconfigID), nil, nil)
	switch {
	case err == nil || isAPINotFound(err):
	case isAPIStatus(err, http.StatusInternalServerError):
		resp.Diagnostics.AddWarning(
			fmt.Sprintf("Scoped kubeconfig %d was revoked, but its Kubernetes objects could not be removed", p.KubeconfigID),
			fmt.Sprintf("The credential no longer works. Its service account and role bindings stay in the cluster until removed by hand: %v", err),
		)
	default:
		resp.Diagnostics.AddError("Failed to revoke the scoped kubeconfig", fmt.Sprintf("failed to revoke scoped kubeconfig %d, which stays valid until it expires or is revoked by hand: %s", p.KubeconfigID, err))
	}
}
