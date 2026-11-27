package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*kubernetesDeploymentRollbackResource)(nil)
	_ resource.ResourceWithConfigure = (*kubernetesDeploymentRollbackResource)(nil)
)

func newKubernetesDeploymentRollbackResource() resource.Resource {
	return &kubernetesDeploymentRollbackResource{}
}

type kubernetesDeploymentRollbackResource struct{ k8sfwBase }

type kubernetesDeploymentRollbackModel struct {
	ID                   types.String `tfsdk:"id"`
	EnvironmentID        types.Int64  `tfsdk:"environment_id"`
	Namespace            types.String `tfsdk:"namespace"`
	DeploymentName       types.String `tfsdk:"deployment_name"`
	Revision             types.Int64  `tfsdk:"revision"`
	RolledBackToReplicas types.Int64  `tfsdk:"rolled_back_to_replicas"`
}

func (r *kubernetesDeploymentRollbackResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_deployment_rollback"
}

func (r *kubernetesDeploymentRollbackResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the rollback, `<environment_id>/<namespace>/<deployment_name>/rollback/<timestamp>`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"environment_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the Portainer Kubernetes environment the deployment belongs to.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"namespace": schema.StringAttribute{
				Required:      true,
				Description:   "Namespace the deployment lives in.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"deployment_name": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the deployment to roll back.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"revision": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Default:       int64default.StaticInt64(0),
				Description:   "Rollout revision to roll back to. `0` (the default) rolls back to the previous revision, matching `kubectl rollout undo`. Use the `portainer_kubernetes_replicasets` data source to discover the available revisions.",
				Validators:    []validator.Int64{int64validator.AtLeast(0)},
				PlanModifiers: []planmodifier.Int64{k8sfwKeepNull{}, int64planmodifier.RequiresReplace()},
			},
			"rolled_back_to_replicas": schema.Int64Attribute{
				Computed:      true,
				Description:   "Desired replica count of the deployment after the rollback.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *kubernetesDeploymentRollbackResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubernetesDeploymentRollbackModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	kubernetesDeploymentRollbackRun(ctx, r.client, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// kubernetesDeploymentRollbackRun rolls the deployment back; it is the
// operation of both the resource and the
// portainer_kubernetes_deployment_rollback action.
func kubernetesDeploymentRollbackRun(ctx context.Context, client *APIClient, plan *kubernetesDeploymentRollbackModel, diags *diag.Diagnostics) {
	envID := plan.EnvironmentID.ValueInt64()
	namespace := plan.Namespace.ValueString()
	name := plan.DeploymentName.ValueString()
	revision := int(plan.Revision.ValueInt64())

	var deployment k8sDeployment
	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/deployments/%s/rollback", client.Endpoint, envID, namespace, name)
	if err := doJSON(ctx, client, http.MethodPost, url, map[string]interface{}{"revision": revision}, &deployment); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_kubernetes_deployment_rollback", requires245)
		diags.AddError("Failed to roll the deployment back", fmt.Errorf("failed to roll deployment %q in namespace %q back to revision %d: %w", name, namespace, revision, err).Error())
		return
	}

	replicas := 1
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}
	plan.RolledBackToReplicas = types.Int64Value(int64(replicas))
	// A rollback is a one-shot action, so the ID carries a timestamp: re-running
	// it means replacing the resource, the same pattern portainer_helm_rollback
	// uses.
	plan.ID = types.StringValue(fmt.Sprintf("%d/%s/%s/rollback/%d", envID, namespace, name, makeTimestamp()))
}

// Read has nothing to refresh: the rollback is a one-shot action.
func (r *kubernetesDeploymentRollbackResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

// Update is never called: every argument forces a new rollback.
func (r *kubernetesDeploymentRollbackResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	k8sfwKeepState(ctx, req, resp)
}

// Delete only forgets the rollback, which cannot be undone.
func (r *kubernetesDeploymentRollbackResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
