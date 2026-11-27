package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*endpointSnapshotResource)(nil)
	_ resource.ResourceWithConfigure = (*endpointSnapshotResource)(nil)
)

// endpointSnapshotResource triggers a snapshot of one environment or of all
// of them. It is a one-time action: nothing is read back or deleted.
func newEndpointSnapshotResource() resource.Resource { return &endpointSnapshotResource{} }

type endpointSnapshotResource struct{ client *APIClient }

type endpointSnapshotModel struct {
	ID         types.String `tfsdk:"id"`
	EndpointID types.Int64  `tfsdk:"endpoint_id"`
}

func (r *endpointSnapshotResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint_snapshot"
}

func (r *endpointSnapshotResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"endpoint_id": schema.Int64Attribute{
				Optional:      true,
				Description:   "ID of the endpoint to snapshot. If omitted, all endpoints will be snapshotted.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *endpointSnapshotResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *endpointSnapshotResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan endpointSnapshotModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpointSnapshotRun(ctx, r.client, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// endpointSnapshotRun takes the snapshot(s) and sets the ID; it is the
// operation of both the resource and the portainer_endpoint_snapshot action.
func endpointSnapshotRun(ctx context.Context, client *APIClient, plan *endpointSnapshotModel, diags *diag.Diagnostics) {
	// An endpoint_id of 0 counts as unset, as it did under SDKv2.
	url := fmt.Sprintf("%s/endpoints/snapshot", client.Endpoint)
	id := "all"
	if v := plan.EndpointID.ValueInt64(); v != 0 {
		url = fmt.Sprintf("%s/endpoints/%d/snapshot", client.Endpoint, v)
		id = strconv.FormatInt(v, 10)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		diags.AddError("Failed to snapshot endpoint(s)", err.Error())
		return
	}
	if err := setAuthHeader(httpReq, client); err != nil {
		diags.AddError("Failed to snapshot endpoint(s)", err.Error())
		return
	}
	httpResp, err := client.HTTPClient.Do(httpReq)
	if err != nil {
		diags.AddError("Failed to snapshot endpoint(s)", err.Error())
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode != http.StatusNoContent {
		diags.AddError("Failed to snapshot endpoint(s)", fmt.Sprintf("failed to snapshot endpoint(s): HTTP %d", httpResp.StatusCode))
		return
	}

	plan.ID = types.StringValue(id)
}

// Read has no meaningful state to refresh; this is a one-time action resource.
func (r *endpointSnapshotResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}

// Update is never called: endpoint_id forces a new resource.
func (r *endpointSnapshotResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Endpoint snapshots cannot be updated", "Every attribute of portainer_endpoint_snapshot forces a new resource.")
}

// Delete has nothing to delete in Portainer; the resource just leaves state.
func (r *endpointSnapshotResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
