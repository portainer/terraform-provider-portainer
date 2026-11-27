package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
)

var (
	_ resource.Resource                = (*kubernetesNamespaceIngressControllersResource)(nil)
	_ resource.ResourceWithConfigure   = (*kubernetesNamespaceIngressControllersResource)(nil)
	_ resource.ResourceWithImportState = (*kubernetesNamespaceIngressControllersResource)(nil)
)

func newKubernetesNamespaceIngressControllersResource() resource.Resource {
	return &kubernetesNamespaceIngressControllersResource{}
}

type kubernetesNamespaceIngressControllersResource struct{ k8sfwBase }

func (r *kubernetesNamespaceIngressControllersResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_namespace_ingresscontrollers"
}

func (r *kubernetesNamespaceIngressControllersResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the namespace, `<environment_id>:<namespace>`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"environment_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the Portainer Kubernetes environment hosting the namespace.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"namespace": schema.StringAttribute{
				Required:      true,
				Description:   "Kubernetes namespace whose ingress controller allow-list is being configured.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
		Blocks: map[string]schema.Block{
			// The PUT replaces the namespace's list, so a change applies in place.
			"controllers": ingressControllersBlock("List of ingress controllers allowed for the namespace."),
		},
	}
}

func (r *kubernetesNamespaceIngressControllersResource) url(data map[string]interface{}) (int, string) {
	id, _ := data["environment_id"].(int)
	namespace, _ := data["namespace"].(string)
	return id, fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/ingresscontrollers", r.client.Endpoint, id, namespace)
}

func (r *kubernetesNamespaceIngressControllersResource) write(ctx context.Context, data map[string]interface{}, diags *diag.Diagnostics) {
	id, url := r.url(data)
	if err := putIngressControllers(ctx, r.client, id, url, ingressControllersFromData(data, false), "namespace ingress controllers"); err != nil {
		diags.AddError("Failed to update the namespace ingress controllers", err.Error())
		return
	}
	data["id"] = fmt.Sprintf("%d:%s", id, data["namespace"])
	if _, err := readIngressControllers(ctx, r.client, url, data, "namespace ingress controllers"); err != nil {
		diags.AddError("Failed to read the namespace ingress controllers", err.Error())
	}
}

func (r *kubernetesNamespaceIngressControllersResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	data, plan := k8sfwData(ctx, req.Plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	k8sfwSetApplied(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, plan, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

func (r *kubernetesNamespaceIngressControllersResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	data, prior := k8sfwData(ctx, req.State, &resp.Diagnostics)
	fromSDK := k8sfwFromSDK(ctx, req.Private, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	_, url := r.url(data)
	found, err := readIngressControllers(ctx, r.client, url, data, "namespace ingress controllers")
	if err != nil {
		resp.Diagnostics.AddError("Failed to read the namespace ingress controllers", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	k8sfwSetState(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, prior, fromSDK, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

func (r *kubernetesNamespaceIngressControllersResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	data, plan := k8sfwData(ctx, req.Plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, data, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	k8sfwSetApplied(ctx, k8sfwSchemaOf(ctx, r), &resp.State, data, plan, &resp.Diagnostics)
	k8sfwMark(ctx, resp.Private, &resp.Diagnostics)
}

// Delete makes the controllers available in the namespace again, which is
// Portainer's default, rather than leaving them blocked once the resource is
// gone: there is no DELETE endpoint.
func (r *kubernetesNamespaceIngressControllersResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	data, _ := k8sfwData(ctx, req.State, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	id, url := r.url(data)
	unlock := lockKubernetesConfig(id)
	defer unlock()
	if err := doJSON(ctx, r.client, http.MethodPut, url, ingressControllersFromData(data, true), nil); err != nil {
		resp.Diagnostics.AddError("Failed to restore the namespace ingress controllers", fmt.Errorf("failed to restore namespace ingress controllers: %w", err).Error())
	}
}

// ImportState takes "<environment_id>:<namespace>".
func (r *kubernetesNamespaceIngressControllersResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	const format = "<environment_id>:<namespace>"
	parts, err := importIDParts(req.ID, ":", 2, format)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	id, err := strconv.Atoi(parts[0])
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("invalid import ID %q: expected %s", req.ID, format))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("environment_id"), int64(id))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("namespace"), parts[1])...)
}
