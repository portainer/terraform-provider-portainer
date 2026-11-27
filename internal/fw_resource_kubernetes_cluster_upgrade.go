package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*kubernetesClusterUpgradeResource)(nil)
	_ resource.ResourceWithConfigure = (*kubernetesClusterUpgradeResource)(nil)
)

// kubernetesClusterUpgradeResource upgrades a provisioned Kubernetes cluster
// to the next stable version. It is a one-shot action with no state of its own
// to read back, so `triggers` is how another upgrade is asked for.
func newKubernetesClusterUpgradeResource() resource.Resource {
	return &kubernetesClusterUpgradeResource{}
}

type kubernetesClusterUpgradeResource struct{ k8sfwBase }

type kubernetesClusterUpgradeModel struct {
	ID         types.String `tfsdk:"id"`
	EndpointID types.Int64  `tfsdk:"endpoint_id"`
	Triggers   types.Map    `tfsdk:"triggers"`
}

func (r *kubernetesClusterUpgradeResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_kubernetes_cluster_upgrade"
}

func (r *kubernetesClusterUpgradeResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the upgrade run, `<endpoint_id>-upgrade-<timestamp>`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"endpoint_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the provisioned Kubernetes environment to upgrade.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"triggers": schema.MapAttribute{
				ElementType:   types.StringType,
				Optional:      true,
				Description:   "Arbitrary values that force another upgrade when they change. An upgrade is a one-shot action, so without a trigger the resource runs once and then stays put.",
				PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *kubernetesClusterUpgradeResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan kubernetesClusterUpgradeModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	kubernetesClusterUpgradeRun(ctx, r.client, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// kubernetesClusterUpgradeRun starts the cluster upgrade; it is the operation
// of both the resource and the portainer_kubernetes_cluster_upgrade action.
func kubernetesClusterUpgradeRun(ctx context.Context, client *APIClient, plan *kubernetesClusterUpgradeModel, diags *diag.Diagnostics) {
	endpointID := plan.EndpointID.ValueInt64()

	upgradeURL := fmt.Sprintf("%s/cloud/endpoints/%d/upgrade", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodPost, upgradeURL, nil, nil); err != nil {
		diags.AddError("Failed to upgrade the Kubernetes cluster", fmt.Errorf("failed to upgrade the Kubernetes cluster of environment %d: %w", endpointID, err).Error())
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%d-upgrade-%d", endpointID, makeTimestamp()))
}

// Read has nothing to refresh: the upgrade is a one-shot action.
func (r *kubernetesClusterUpgradeResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

// Update is never called: every argument forces a new upgrade.
func (r *kubernetesClusterUpgradeResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	k8sfwKeepState(ctx, req, resp)
}

// Delete only forgets the upgrade, which cannot be undone.
func (r *kubernetesClusterUpgradeResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
