package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*kubernetesNodeDrainResource)(nil)
	_ resource.ResourceWithConfigure = (*kubernetesNodeDrainResource)(nil)
)

func newKubernetesNodeDrainResource() resource.Resource {
	return &kubernetesNodeDrainResource{}
}

type kubernetesNodeDrainResource struct{ k8sfwBase }

type kubernetesNodeDrainModel struct {
	ID                 types.String `tfsdk:"id"`
	EnvironmentID      types.Int64  `tfsdk:"environment_id"`
	NodeName           types.String `tfsdk:"node_name"`
	Force              types.Bool   `tfsdk:"force"`
	TimeoutSeconds     types.Int64  `tfsdk:"timeout_seconds"`
	GracePeriodSeconds types.Int64  `tfsdk:"grace_period_seconds"`
	IgnoreDaemonSets   types.Bool   `tfsdk:"ignore_daemon_sets"`
	DeleteEmptyDirData types.Bool   `tfsdk:"delete_empty_dir_data"`
	DisableEviction    types.Bool   `tfsdk:"disable_eviction"`
}

func (r *kubernetesNodeDrainResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_node_drain"
}

func (r *kubernetesNodeDrainResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.Bool{k8sfwKeepNull{}, boolplanmodifier.RequiresReplace()}
	replaceInt := []planmodifier.Int64{k8sfwKeepNull{}, int64planmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the drain, `<environment_id>/<node_name>`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"environment_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the Portainer Kubernetes environment the node belongs to.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"node_name": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the Kubernetes node to drain.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			// The defaults below mirror Portainer's own libkubectl.DefaultDrainOptions,
			// so an unset field behaves exactly as omitting it from the API payload.
			"force": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(false),
				Description:   "Whether to allow deletion of standalone pods that are not managed by a controller. Such pods are lost for good, so the drain refuses them unless this is enabled. Mirrors `kubectl drain --force`.",
				PlanModifiers: replace,
			},
			"timeout_seconds": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Default:       int64default.StaticInt64(60),
				Description:   "Overall time in seconds to wait for the drain to complete. Mirrors `kubectl drain --timeout`.",
				Validators:    []validator.Int64{int64validator.AtLeast(0)},
				PlanModifiers: replaceInt,
			},
			"grace_period_seconds": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Default:       int64default.StaticInt64(-1),
				Description:   "Termination grace period in seconds to apply to every evicted pod. `-1` keeps each pod's own grace period. Mirrors `kubectl drain --grace-period`.",
				Validators:    []validator.Int64{int64validator.AtLeast(-1)},
				PlanModifiers: replaceInt,
			},
			"ignore_daemon_sets": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(true),
				Description:   "Whether to skip DaemonSet-managed pods, which would otherwise block the drain because their controller recreates them immediately. Mirrors `kubectl drain --ignore-daemonsets`.",
				PlanModifiers: replace,
			},
			"delete_empty_dir_data": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(true),
				Description:   "Whether to evict pods that use an emptyDir volume, whose data is lost once the pod is deleted. Mirrors `kubectl drain --delete-emptydir-data`.",
				PlanModifiers: replace,
			},
			"disable_eviction": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(false),
				Description:   "Whether to delete pods directly instead of going through the eviction API, which ignores any configured PodDisruptionBudget. Mirrors `kubectl drain --disable-eviction`.",
				PlanModifiers: replace,
			},
		},
	}
}

func (r *kubernetesNodeDrainResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubernetesNodeDrainModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	kubernetesNodeDrainRun(ctx, r.client, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// kubernetesNodeDrainRun drains the node; it is the operation of both the
// resource and the portainer_kubernetes_node_drain action.
func kubernetesNodeDrainRun(ctx context.Context, client *APIClient, plan *kubernetesNodeDrainModel, diags *diag.Diagnostics) {
	envID := plan.EnvironmentID.ValueInt64()
	nodeName := plan.NodeName.ValueString()

	// Every field is sent explicitly. The schema defaults match Portainer's own
	// drain defaults, so this is equivalent to omitting them while keeping the
	// effective options visible in state and in the plan.
	//
	// The keys are PascalCase on purpose: Portainer's drainNodePayload carries no
	// json tags, so the Go field names are the wire names. camelCase would also
	// decode (encoding/json falls back to a case-insensitive match), but the
	// exact names keep this independent of that fallback.
	body := map[string]interface{}{
		"Force":              plan.Force.ValueBool(),
		"TimeoutSeconds":     int(plan.TimeoutSeconds.ValueInt64()),
		"GracePeriodSeconds": int(plan.GracePeriodSeconds.ValueInt64()),
		"IgnoreDaemonSets":   plan.IgnoreDaemonSets.ValueBool(),
		"DeleteEmptyDirData": plan.DeleteEmptyDirData.ValueBool(),
		"DisableEviction":    plan.DisableEviction.ValueBool(),
	}

	url := fmt.Sprintf("%s/kubernetes/%d/nodes/%s/drain", client.Endpoint, envID, nodeName)
	if err := doJSON(ctx, client, http.MethodPost, url, body, nil); err != nil {
		diags.AddError("Failed to drain the Kubernetes node", fmt.Errorf("failed to drain Kubernetes node %q: %w", nodeName, err).Error())
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%d/%s", envID, nodeName))
}

// Read has nothing to refresh: the drain is a one-shot action.
func (r *kubernetesNodeDrainResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

// Update is never called: every argument forces a new drain.
func (r *kubernetesNodeDrainResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	k8sfwKeepState(ctx, req, resp)
}

// Delete only forgets the drain; the node stays cordoned.
func (r *kubernetesNodeDrainResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
