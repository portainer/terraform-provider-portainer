package internal

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*kubernetesDeleteObjectResource)(nil)
	_ resource.ResourceWithConfigure = (*kubernetesDeleteObjectResource)(nil)
)

func newKubernetesDeleteObjectResource() resource.Resource {
	return &kubernetesDeleteObjectResource{}
}

type kubernetesDeleteObjectResource struct{ k8sfwBase }

type kubernetesDeleteObjectModel struct {
	ID            types.String `tfsdk:"id"`
	EnvironmentID types.Int64  `tfsdk:"environment_id"`
	ResourceType  types.String `tfsdk:"resource_type"`
	Namespace     types.String `tfsdk:"namespace"`
	Names         types.List   `tfsdk:"names"`
}

func (r *kubernetesDeleteObjectResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_delete_object"
}

func (r *kubernetesDeleteObjectResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the deletion, `<environment_id>:<resource_type>:<names>`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"environment_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the Portainer Kubernetes environment from which the resources are deleted.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"resource_type": schema.StringAttribute{
				Required:    true,
				Description: "Type of Kubernetes resource to delete. One of `cron_jobs`, `ingresses`, `jobs`, `role_bindings`, `roles`, `service_accounts`, `services`, `cluster_role_bindings`, `cluster_roles`.",
				Validators: []validator.String{stringvalidator.OneOf(
					"cron_jobs", "ingresses", "jobs", "role_bindings",
					"roles", "service_accounts", "services", "cluster_role_bindings",
					"cluster_roles",
				)},
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"namespace": schema.StringAttribute{
				Required:      true,
				Description:   "Kubernetes namespace containing the resources to delete (ignored for cluster-scoped types).",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"names": schema.ListAttribute{
				ElementType:   types.StringType,
				Required:      true,
				Description:   "List of resource names to delete within the target namespace or cluster scope.",
				PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *kubernetesDeleteObjectResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubernetesDeleteObjectModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	kubernetesDeleteObjectRun(ctx, r.client, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// kubernetesDeleteObjectRun deletes the objects; it is the operation of both
// the resource and the portainer_kubernetes_delete_object action.
func kubernetesDeleteObjectRun(ctx context.Context, client *APIClient, plan *kubernetesDeleteObjectModel, diags *diag.Diagnostics) {
	envID := plan.EnvironmentID.ValueInt64()
	typePath := plan.ResourceType.ValueString()
	namespace := plan.Namespace.ValueString()
	names := []string{}
	diags.Append(plan.Names.ElementsAs(ctx, &names, false)...)
	if diags.HasError() {
		return
	}

	body := map[string][]string{
		namespace: names,
	}

	url := fmt.Sprintf("%s/kubernetes/%d/%s/delete", client.Endpoint, envID, typePath)
	if err := doJSON(ctx, client, http.MethodPost, url, body, nil); err != nil {
		diags.AddError("Failed to delete the Kubernetes objects", fmt.Errorf("failed to delete %s: %w", typePath, err).Error())
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%d:%s:%s", envID, typePath, strings.Join(names, ",")))
}

// Read has nothing to refresh: the deletion is a one-shot action.
func (r *kubernetesDeleteObjectResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

// Update is never called: every argument forces a new deletion.
func (r *kubernetesDeleteObjectResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	k8sfwKeepState(ctx, req, resp)
}

// Delete only forgets the deletion.
func (r *kubernetesDeleteObjectResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
