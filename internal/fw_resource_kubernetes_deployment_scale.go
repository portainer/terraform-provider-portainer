package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*kubernetesDeploymentScaleResource)(nil)
	_ resource.ResourceWithConfigure = (*kubernetesDeploymentScaleResource)(nil)
)

func newKubernetesDeploymentScaleResource() resource.Resource {
	return &kubernetesDeploymentScaleResource{}
}

type kubernetesDeploymentScaleResource struct{ k8sfwBase }

type kubernetesDeploymentScaleModel struct {
	ID                types.String `tfsdk:"id"`
	EnvironmentID     types.Int64  `tfsdk:"environment_id"`
	Namespace         types.String `tfsdk:"namespace"`
	DeploymentName    types.String `tfsdk:"deployment_name"`
	Replicas          types.Int64  `tfsdk:"replicas"`
	ReadyReplicas     types.Int64  `tfsdk:"ready_replicas"`
	AvailableReplicas types.Int64  `tfsdk:"available_replicas"`
}

// k8sDeployment is the subset of the appsv1.Deployment response the scale
// and rollback resources read back.
type k8sDeployment struct {
	Spec struct {
		Replicas *int `json:"replicas"`
	} `json:"spec"`
	Status struct {
		ReadyReplicas     int `json:"readyReplicas"`
		AvailableReplicas int `json:"availableReplicas"`
	} `json:"status"`
}

func (r *kubernetesDeploymentScaleResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_deployment_scale"
}

func (r *kubernetesDeploymentScaleResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the scale, `<environment_id>/<namespace>/<deployment_name>/scale`.",
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
				Description:   "Name of the deployment to scale. The deployment must already exist; this resource manages only its replica count.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"replicas": schema.Int64Attribute{
				Required:    true,
				Description: "Desired number of replicas. Changing this scales the deployment in place.",
				Validators:  []validator.Int64{int64validator.AtLeast(0)},
			},
			"ready_replicas": schema.Int64Attribute{
				Computed:    true,
				Description: "Number of replicas passing their readiness checks, as reported after the scale request.",
			},
			"available_replicas": schema.Int64Attribute{
				Computed:    true,
				Description: "Number of replicas available for at least the configured minimum ready seconds, as reported after the scale request.",
			},
		},
	}
}

func (r *kubernetesDeploymentScaleResource) scale(ctx context.Context, plan *kubernetesDeploymentScaleModel) error {
	return kubernetesDeploymentScaleRun(ctx, r.client, plan)
}

// kubernetesDeploymentScaleRun scales the deployment and records what
// Kubernetes reports; it is the operation of the resource (create and update)
// and of the portainer_kubernetes_deployment_scale action.
func kubernetesDeploymentScaleRun(ctx context.Context, client *APIClient, plan *kubernetesDeploymentScaleModel) error {
	envID := plan.EnvironmentID.ValueInt64()
	namespace := plan.Namespace.ValueString()
	name := plan.DeploymentName.ValueString()
	replicas := int(plan.Replicas.ValueInt64())

	var deployment k8sDeployment
	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/deployments/%s/scale", client.Endpoint, envID, namespace, name)
	if err := doJSON(ctx, client, http.MethodPut, url, map[string]interface{}{"replicas": replicas}, &deployment); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_kubernetes_deployment_scale", requires245)
		return fmt.Errorf("failed to scale deployment %q in namespace %q to %d replicas: %w", name, namespace, replicas, err)
	}
	plan.ReadyReplicas = types.Int64Value(int64(deployment.Status.ReadyReplicas))
	plan.AvailableReplicas = types.Int64Value(int64(deployment.Status.AvailableReplicas))
	plan.ID = types.StringValue(fmt.Sprintf("%d/%s/%s/scale", envID, namespace, name))
	return nil
}

func (r *kubernetesDeploymentScaleResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubernetesDeploymentScaleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.scale(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Failed to scale the deployment", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *kubernetesDeploymentScaleResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state kubernetesDeploymentScaleModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	envID := state.EnvironmentID.ValueInt64()
	namespace := state.Namespace.ValueString()
	name := state.DeploymentName.ValueString()

	var deployment k8sDeployment
	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/deployments/%s", r.client.Endpoint, envID, namespace, name)
	if err := doJSON(ctx, r.client, http.MethodGet, url, nil, &deployment); err != nil {
		// A deleted deployment has no replica count to manage any more, so the
		// resource drops out of state instead of failing every plan.
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read the deployment", fmt.Errorf("failed to read deployment %q in namespace %q: %w", name, namespace, err).Error())
		return
	}

	// spec.replicas is a pointer in the Kubernetes API: absent means the default
	// of 1, which is not the same as an explicit 0.
	replicas := 1
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}
	state.Replicas = types.Int64Value(int64(replicas))
	state.ReadyReplicas = types.Int64Value(int64(deployment.Status.ReadyReplicas))
	state.AvailableReplicas = types.Int64Value(int64(deployment.Status.AvailableReplicas))
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *kubernetesDeploymentScaleResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan kubernetesDeploymentScaleModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := r.scale(ctx, &plan); err != nil {
		resp.Diagnostics.AddError("Failed to scale the deployment", err.Error())
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete only forgets the replica count. Scaling has no inverse: dropping the
// resource stops managing the replica count and leaves the deployment
// running at its current scale. Scale to 0 explicitly to stop the workload.
func (r *kubernetesDeploymentScaleResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
