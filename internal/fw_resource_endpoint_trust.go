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
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*endpointTrustResource)(nil)
	_ resource.ResourceWithConfigure   = (*endpointTrustResource)(nil)
	_ resource.ResourceWithImportState = (*endpointTrustResource)(nil)
)

// endpointTrustResource grants trust to an edge environment sitting in
// Portainer's waiting room. The environment itself is created by the agent
// when it first calls home, not by Terraform, so this resource adopts an
// existing environment rather than creating one.
//
// Portainer's endpoint takes a batch, but one resource per environment is what
// lets Terraform track each one's trust state separately; the batch is sent as
// a list of one.
func newEndpointTrustResource() resource.Resource { return &endpointTrustResource{} }

type endpointTrustResource struct{ client *APIClient }

type endpointTrustModel struct {
	ID           types.String `tfsdk:"id"`
	EndpointID   types.Int64  `tfsdk:"endpoint_id"`
	GroupID      types.Int64  `tfsdk:"group_id"`
	EdgeGroupIDs types.List   `tfsdk:"edge_group_ids"`
	TagIDs       types.List   `tfsdk:"tag_ids"`
	Name         types.String `tfsdk:"name"`
	EdgeID       types.String `tfsdk:"edge_id"`
	Trusted      types.Bool   `tfsdk:"trusted"`
}

func (r *endpointTrustResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint_trust"
}

func (r *endpointTrustResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"endpoint_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the edge environment in the waiting room to trust. Use the `portainer_edge_waiting_room` data source to find environments awaiting trust.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			// The three relation fields below are only honoured by the call that
			// grants trust, so changing one has to re-run that call. They force
			// a new resource rather than being updatable in place: an update
			// that quietly did nothing would be worse than a plan that says
			// what it will do.
			"group_id": schema.Int64Attribute{
				Optional:      true,
				Description:   "Identifier of the endpoint group to place the environment in when trust is granted. Leave unset to keep the group the environment already has.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"edge_group_ids": schema.ListAttribute{
				Optional:      true,
				ElementType:   types.Int64Type,
				Description:   "Identifiers of the edge groups to add the environment to when trust is granted. Leave unset to keep the edge groups the environment already has.",
				PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplace()},
			},
			"tag_ids": schema.ListAttribute{
				Optional:      true,
				ElementType:   types.Int64Type,
				Description:   "Identifiers of the tags to apply to the environment when trust is granted. Leave unset to keep the tags the environment already has.",
				PlanModifiers: []planmodifier.List{listplanmodifier.RequiresReplace()},
			},
			"name": schema.StringAttribute{
				Computed:    true,
				Description: "Name of the trusted environment.",
			},
			"edge_id": schema.StringAttribute{
				Computed:    true,
				Description: "Edge identifier the agent reports for the environment.",
			},
			"trusted": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether Portainer reports the environment as trusted.",
			},
		},
	}
}

func (r *endpointTrustResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *endpointTrustResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan endpointTrustModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	endpointID := int(plan.EndpointID.ValueInt64())

	// Each relation field is omitted unless it was configured. Portainer reads
	// an empty array as "clear this", so sending one would strip the tags or
	// edge groups the environment already carries.
	relation := map[string]interface{}{}
	if !plan.GroupID.IsNull() && plan.GroupID.ValueInt64() != 0 {
		relation["Group"] = int(plan.GroupID.ValueInt64())
	}
	if ids := int64ListToInts(ctx, plan.EdgeGroupIDs, &resp.Diagnostics); len(ids) > 0 {
		relation["EdgeGroups"] = ids
	}
	if ids := int64ListToInts(ctx, plan.TagIDs, &resp.Diagnostics); len(ids) > 0 {
		relation["Tags"] = ids
	}
	if resp.Diagnostics.HasError() {
		return
	}

	payload := map[string]interface{}{"EndpointIDs": []int{endpointID}}
	if len(relation) > 0 {
		payload["Relations"] = map[string]interface{}{strconv.Itoa(endpointID): relation}
	}

	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/endpoints/edge/trust", payload, nil); err != nil {
		resp.Diagnostics.AddError("Failed to trust environment", fmt.Errorf("failed to trust environment %d: %w", endpointID, err).Error())
		return
	}

	plan.ID = types.StringValue(strconv.Itoa(endpointID))
	if !r.read(ctx, &plan, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.Diagnostics.AddError("Failed to trust environment", fmt.Sprintf("environment %d is not trusted after the trust call", endpointID))
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// read refreshes the computed attributes; false when the environment is gone
// or no longer trusted.
func (r *endpointTrustResource) read(ctx context.Context, m *endpointTrustModel, diags *diag.Diagnostics) bool {
	endpointID, err := strconv.Atoi(m.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid resource ID", fmt.Errorf("the resource ID must be an environment identifier, got %q: %w", m.ID.ValueString(), err).Error())
		return false
	}

	// The relation fields are deliberately not read back. They are only
	// honoured when trust is granted, and reading a merged result into state
	// would make Terraform plan a replacement against its own configuration.
	var endpoint struct {
		ID          int    `json:"Id"`
		Name        string `json:"Name"`
		EdgeID      string `json:"EdgeID"`
		UserTrusted bool   `json:"UserTrusted"`
	}
	url := fmt.Sprintf("%s/endpoints/%d", r.client.Endpoint, endpointID)
	if err := doJSON(ctx, r.client, http.MethodGet, url, nil, &endpoint); err != nil {
		if isAPINotFound(err) {
			return false
		}
		diags.AddError("Failed to read environment", fmt.Errorf("failed to read environment %d: %w", endpointID, err).Error())
		return false
	}

	// An environment put back in the waiting room outside Terraform is drift
	// the next apply can actually fix, so it leaves state rather than being
	// reported as trusted.
	if !endpoint.UserTrusted {
		return false
	}

	m.EndpointID = types.Int64Value(int64(endpoint.ID))
	m.Name = types.StringValue(endpoint.Name)
	m.EdgeID = types.StringValue(endpoint.EdgeID)
	m.Trusted = types.BoolValue(endpoint.UserTrusted)
	return true
}

func (r *endpointTrustResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state endpointTrustModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.read(ctx, &state, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// Update is never called: every configurable attribute forces a new resource.
func (r *endpointTrustResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Endpoint trust cannot be updated", "Every attribute of portainer_endpoint_trust forces a new resource.")
}

// Delete only forgets the resource. Portainer has no endpoint to revoke trust:
// destroying the resource leaves the environment trusted; untrusting is done
// by deleting the environment.
func (r *endpointTrustResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *endpointTrustResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
