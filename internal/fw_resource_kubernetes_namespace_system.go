package internal

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                   = (*kubernetesNamespaceSystemResource)(nil)
	_ resource.ResourceWithConfigure      = (*kubernetesNamespaceSystemResource)(nil)
	_ resource.ResourceWithImportState    = (*kubernetesNamespaceSystemResource)(nil)
	_ resource.ResourceWithValidateConfig = (*kubernetesNamespaceSystemResource)(nil)
)

func newKubernetesNamespaceSystemResource() resource.Resource {
	return &kubernetesNamespaceSystemResource{}
}

type kubernetesNamespaceSystemResource struct{ k8sfwBase }

type kubernetesNamespaceSystemModel struct {
	ID            types.String `tfsdk:"id"`
	EnvironmentID types.Int64  `tfsdk:"environment_id"`
	Namespace     types.String `tfsdk:"namespace"`
	System        types.Bool   `tfsdk:"system"`
}

func (r *kubernetesNamespaceSystemResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_namespace_system"
}

func (r *kubernetesNamespaceSystemResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the namespace, `<environment_id>:<namespace>`.",
			},
			"environment_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the Portainer Kubernetes environment owning the namespace.",
			},
			"namespace": schema.StringAttribute{
				Required:    true,
				Description: "Name of the Kubernetes namespace whose system flag is being toggled.",
			},
			"system": schema.BoolAttribute{
				Required:    true,
				Description: "Whether the namespace should be marked as a Portainer system namespace.",
			},
		},
	}
}

// ValidateConfig rejects marking the default namespace as a system
// namespace: Portainer answers 204 but never changes it, so the plan asked
// for it again on every run.
func (r *kubernetesNamespaceSystemResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var config kubernetesNamespaceSystemModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !config.Namespace.IsUnknown() && config.Namespace.ValueString() == "default" && config.System.ValueBool() {
		resp.Diagnostics.AddAttributeError(path.Root("system"), "Invalid system namespace",
			"the default namespace cannot be a system namespace: Portainer never marks it as one; use another namespace")
	}
}

func (r *kubernetesNamespaceSystemResource) toggle(ctx context.Context, m *kubernetesNamespaceSystemModel, system bool) error {
	id := m.EnvironmentID.ValueInt64()
	namespace := m.Namespace.ValueString()

	body := map[string]interface{}{
		"system": system,
	}
	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/system", r.client.Endpoint, id, namespace)
	if err := doJSON(ctx, r.client, http.MethodPut, url, body, nil); err != nil {
		return fmt.Errorf("failed to toggle namespace system state: %w", err)
	}
	m.ID = types.StringValue(fmt.Sprintf("%d:%s", id, namespace))
	return nil
}

func (r *kubernetesNamespaceSystemResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubernetesNamespaceSystemModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.toggle(ctx, &plan, plan.System.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Failed to toggle the namespace system state", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *kubernetesNamespaceSystemResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state kubernetesNamespaceSystemModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := state.ID.ValueString()
	invalid := fmt.Errorf("invalid ID format, expected 'envID:namespace': %s", id)
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid resource ID", invalid.Error())
		return
	}
	var envID int
	_, _ = fmt.Sscanf(parts[0], "%d", &envID)
	namespace := parts[1]
	if envID == 0 || namespace == "" {
		resp.Diagnostics.AddError("Invalid resource ID", invalid.Error())
		return
	}

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s", r.client.Endpoint, envID, namespace)
	var ns struct {
		IsSystem bool `json:"IsSystem"`
	}
	if err := doJSON(ctx, r.client, http.MethodGet, url, nil, &ns); err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read the namespace", fmt.Errorf("failed to read namespace %q: %w", namespace, err).Error())
		return
	}

	state.EnvironmentID = types.Int64Value(int64(envID))
	state.Namespace = types.StringValue(namespace)
	state.System = types.BoolValue(ns.IsSystem)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *kubernetesNamespaceSystemResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan kubernetesNamespaceSystemModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.toggle(ctx, &plan, plan.System.ValueBool()); err != nil {
		resp.Diagnostics.AddError("Failed to toggle the namespace system state", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete unmarks the namespace as a system namespace.
func (r *kubernetesNamespaceSystemResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state kubernetesNamespaceSystemModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.toggle(ctx, &state, false); err != nil {
		resp.Diagnostics.AddError("Failed to toggle the namespace system state", err.Error())
	}
}

func (r *kubernetesNamespaceSystemResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
