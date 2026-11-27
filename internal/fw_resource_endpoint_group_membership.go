package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*endpointGroupMembershipResource)(nil)
	_ resource.ResourceWithConfigure = (*endpointGroupMembershipResource)(nil)
)

func newEndpointGroupMembershipResource() resource.Resource {
	return &endpointGroupMembershipResource{}
}

type endpointGroupMembershipResource struct{ client *APIClient }

type endpointGroupMembershipModel struct {
	ID              types.String `tfsdk:"id"`
	EndpointGroupID types.Int64  `tfsdk:"endpoint_group_id"`
	EndpointID      types.Int64  `tfsdk:"endpoint_id"`
}

func (r *endpointGroupMembershipResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint_group_membership"
}

func (r *endpointGroupMembershipResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the membership: <endpoint_group_id>/<endpoint_id>.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"endpoint_group_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the environment group the environment is added to.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"endpoint_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the environment to place in the group.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *endpointGroupMembershipResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *endpointGroupMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan endpointGroupMembershipModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	groupID, endpointID := plan.EndpointGroupID.ValueInt64(), plan.EndpointID.ValueInt64()

	url := fmt.Sprintf("%s/endpoint_groups/%d/endpoints/%d", r.client.Endpoint, groupID, endpointID)
	if err := doJSON(ctx, r.client, http.MethodPut, url, nil, nil); err != nil {
		resp.Diagnostics.AddError("Failed to create endpoint group membership", fmt.Sprintf("failed to add environment %d to group %d: %s", endpointID, groupID, err))
		return
	}
	plan.ID = types.StringValue(fmt.Sprintf("%d/%d", groupID, endpointID))

	// The SDKv2 version re-read the membership here.
	found, err := r.member(ctx, plan)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read endpoint group membership", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to create endpoint group membership", fmt.Sprintf("environment %d is not in group %d right after it was added", endpointID, groupID))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// member reports whether the environment is in the group. Membership is a
// property of the environment, not a resource of its own: endpoint.GroupId is
// the single source of truth, and moving an environment to another group
// silently removes it from this one.
func (r *endpointGroupMembershipResource) member(ctx context.Context, m endpointGroupMembershipModel) (bool, error) {
	endpointID := m.EndpointID.ValueInt64()
	var endpoint struct {
		GroupID int64 `json:"GroupId"`
	}
	url := fmt.Sprintf("%s/endpoints/%d", r.client.Endpoint, endpointID)
	if err := doJSON(ctx, r.client, http.MethodGet, url, nil, &endpoint); err != nil {
		if isAPINotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read environment %d: %w", endpointID, err)
	}
	return endpoint.GroupID == m.EndpointGroupID.ValueInt64(), nil
}

func (r *endpointGroupMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state endpointGroupMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := r.member(ctx, state)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read endpoint group membership", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
	}
}

// Update is never called: every attribute requires replacement.
func (r *endpointGroupMembershipResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Endpoint group memberships cannot be updated", "Every attribute of portainer_endpoint_group_membership forces a new resource.")
}

func (r *endpointGroupMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state endpointGroupMembershipModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	groupID, endpointID := state.EndpointGroupID.ValueInt64(), state.EndpointID.ValueInt64()

	// Portainer moves the environment back to the Unassigned group (ID 1);
	// there is no state in which an environment belongs to no group at all.
	url := fmt.Sprintf("%s/endpoint_groups/%d/endpoints/%d", r.client.Endpoint, groupID, endpointID)
	if err := doJSON(ctx, r.client, http.MethodDelete, url, nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete endpoint group membership", fmt.Sprintf("failed to remove environment %d from group %d: %s", endpointID, groupID, err))
	}
}
