package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*kubernetesHelmResource)(nil)
	_ resource.ResourceWithConfigure   = (*kubernetesHelmResource)(nil)
	_ resource.ResourceWithImportState = (*kubernetesHelmResource)(nil)
)

func newKubernetesHelmResource() resource.Resource {
	return &kubernetesHelmResource{}
}

type kubernetesHelmResource struct{ k8sfwBase }

type kubernetesHelmModel struct {
	ID            types.String   `tfsdk:"id"`
	EnvironmentID types.Int64    `tfsdk:"environment_id"`
	Chart         types.String   `tfsdk:"chart"`
	Name          types.String   `tfsdk:"name"`
	Namespace     types.String   `tfsdk:"namespace"`
	Repo          types.String   `tfsdk:"repo"`
	Values        types.String   `tfsdk:"values"`
	Timeouts      timeouts.Value `tfsdk:"timeouts"`
}

func (r *kubernetesHelmResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_helm"
}

func (r *kubernetesHelmResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the release, `<environment_id>:<namespace>:<name>`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"environment_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the Portainer Kubernetes environment where the Helm chart is installed.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"chart": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the Helm chart to install.",
				PlanModifiers: replace,
			},
			"name": schema.StringAttribute{
				Required:      true,
				Description:   "Helm release name used to install the chart.",
				PlanModifiers: replace,
			},
			"namespace": schema.StringAttribute{
				Required:      true,
				Description:   "Kubernetes namespace in which the Helm release is created.",
				PlanModifiers: replace,
			},
			"repo": schema.StringAttribute{
				Required:      true,
				Description:   "URL of the Helm chart repository hosting the chart.",
				PlanModifiers: replace,
			},
			"values": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Default:       stringdefault.StaticString(""),
				Description:   "Optional Helm values document (YAML) used to customise the release.",
				PlanModifiers: []planmodifier.String{k8sfwKeepNull{}, stringplanmodifier.RequiresReplace()},
			},
		},
		Blocks: map[string]schema.Block{
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Create: true, Delete: true}),
		},
	}
}

func (r *kubernetesHelmResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubernetesHelmModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	timeout, d := plan.Timeouts.Create(ctx, 15*time.Minute)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	id := plan.EnvironmentID.ValueInt64()
	body := map[string]interface{}{
		"chart":     plan.Chart.ValueString(),
		"name":      plan.Name.ValueString(),
		"namespace": plan.Namespace.ValueString(),
		"repo":      plan.Repo.ValueString(),
		"values":    plan.Values.ValueString(),
	}

	url := fmt.Sprintf("%s/endpoints/%d/kubernetes/helm", r.client.Endpoint, id)
	if err := doJSON(ctx, r.client, http.MethodPost, url, body, nil); err != nil {
		resp.Diagnostics.AddError("Failed to install the Helm chart", fmt.Errorf("failed to install helm chart: %w", err).Error())
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%d:%s:%s", id, plan.Namespace.ValueString(), plan.Name.ValueString()))
	found, err := r.read(ctx, &plan)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the Helm release", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to read the Helm release", fmt.Sprintf("helm release %s not found after the install", plan.Name.ValueString()))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// read confirms the release exists and restores the identity fields from the
// ID, reporting false when it is gone.
//
// chart/repo/values are intentionally not refreshed — the release detail API
// returns a server-normalised chartReference (chartPath/repoURL) that may not
// byte-match the user's authored config, and chart/repo force a new release:
// any mismatch would trigger a destroy+recreate instead of a benign diff.
// After `terraform import`, chart/repo/values must be set in config to match
// the live release or the next apply recreates it.
func (r *kubernetesHelmResource) read(ctx context.Context, m *kubernetesHelmModel) (bool, error) {
	id := m.ID.ValueString()
	idParts := strings.SplitN(id, ":", 3)
	if len(idParts) != 3 {
		return false, fmt.Errorf("invalid ID format, expected 'envID:namespace:release': %s", id)
	}
	var envID int
	_, _ = fmt.Sscanf(idParts[0], "%d", &envID)
	namespace := idParts[1]
	release := idParts[2]
	if envID == 0 || release == "" {
		return false, fmt.Errorf("invalid ID format, expected 'envID:namespace:release': %s", id)
	}

	url := fmt.Sprintf("%s/endpoints/%d/kubernetes/helm/%s?namespace=%s", r.client.Endpoint, envID, release, namespace)
	if err := doJSON(ctx, r.client, http.MethodGet, url, nil, nil); err != nil {
		if isAPINotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read helm release %s: %w", release, err)
	}

	m.EnvironmentID = types.Int64Value(int64(envID))
	m.Namespace = types.StringValue(namespace)
	m.Name = types.StringValue(release)
	return true, nil
}

func (r *kubernetesHelmResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state kubernetesHelmModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := r.read(ctx, &state)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the Helm release", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update only changes the timeouts: every other argument forces a new
// release.
func (r *kubernetesHelmResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	k8sfwKeepState(ctx, req, resp)
}

func (r *kubernetesHelmResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state kubernetesHelmModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	timeout, d := state.Timeouts.Delete(ctx, 10*time.Minute)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	id := state.ID.ValueString()
	idParts := strings.SplitN(id, ":", 3)
	if len(idParts) != 3 {
		resp.Diagnostics.AddError("Invalid resource ID", fmt.Sprintf("invalid ID format, expected 'envID:namespace:release': %s", id))
		return
	}
	envID, namespace, release := idParts[0], idParts[1], idParts[2]

	url := fmt.Sprintf("%s/endpoints/%s/kubernetes/helm/%s?namespace=%s", r.client.Endpoint, envID, release, namespace)
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete the Helm release", err.Error())
		return
	}
	if err := setAuthHeader(httpReq, r.client); err != nil {
		resp.Diagnostics.AddError("Failed to delete the Helm release", err.Error())
		return
	}

	httpResp, err := r.client.HTTPClient.Do(httpReq)
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete the Helm release", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusNoContent {
		data, _ := io.ReadAll(httpResp.Body)
		resp.Diagnostics.AddError("Failed to delete the Helm release", fmt.Sprintf("failed to delete helm release: %s", string(data)))
	}
}

// ImportState takes "<environment_id>:<namespace>:<name>".
func (r *kubernetesHelmResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	k8sfwImportDefaults(ctx, k8sfwSchemaOf(ctx, r), &resp.State, &resp.Diagnostics)
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
