package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

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
	_ resource.Resource              = (*omniNodeRebootResource)(nil)
	_ resource.ResourceWithConfigure = (*omniNodeRebootResource)(nil)
)

// omniNodeRebootResource reboots a node of an Omni cluster. A reboot is a
// one-shot action with no state to read back, so the resource records that it
// ran and `triggers` is how another one is asked for.
func newOmniNodeRebootResource() resource.Resource { return &omniNodeRebootResource{} }

type omniNodeRebootResource struct{ client *APIClient }

type omniNodeRebootModel struct {
	ID           types.String `tfsdk:"id"`
	CredentialID types.Int64  `tfsdk:"credential_id"`
	Cluster      types.String `tfsdk:"cluster"`
	Node         types.String `tfsdk:"node"`
	Triggers     types.Map    `tfsdk:"triggers"`
}

func (r *omniNodeRebootResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_omni_node_reboot"
}

func (r *omniNodeRebootResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"credential_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the Portainer cloud credential holding the Omni endpoint and service account key.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"cluster": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the cluster the node belongs to.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"node": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the node to reboot.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"triggers": schema.MapAttribute{
				Optional:      true,
				ElementType:   types.StringType,
				Description:   "Arbitrary values that force another reboot when they change. A reboot is a one-shot action, so without a trigger the resource runs once and then stays put.",
				PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *omniNodeRebootResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *omniNodeRebootResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan omniNodeRebootModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	omniNodeRebootRun(ctx, r.client, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// omniNodeRebootRun reboots the node; it is the operation of both the
// resource and the portainer_omni_node_reboot action.
func omniNodeRebootRun(ctx context.Context, client *APIClient, plan *omniNodeRebootModel, diags *diag.Diagnostics) {
	credentialID := plan.CredentialID.ValueInt64()
	cluster := plan.Cluster.ValueString()
	node := plan.Node.ValueString()

	rebootURL := fmt.Sprintf("%s/omni/%d/node/%s?cluster=%s",
		client.Endpoint, credentialID, url.PathEscape(node), url.QueryEscape(cluster))
	if err := doJSON(ctx, client, http.MethodPost, rebootURL, nil, nil); err != nil {
		omniAddError(diags, fmt.Errorf("failed to reboot node %s of Omni cluster %s: %w", node, cluster, err))
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%d/%s/%s/reboot-%d", credentialID, cluster, node, makeTimestamp()))
}

// Read keeps the state: there is nothing to read back after a reboot.
func (r *omniNodeRebootResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

// Update is never called: every attribute requires replacement.
func (r *omniNodeRebootResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Node reboots cannot be updated", "Every attribute of portainer_omni_node_reboot forces a new resource.")
}

// Delete only forgets the reboot.
func (r *omniNodeRebootResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
