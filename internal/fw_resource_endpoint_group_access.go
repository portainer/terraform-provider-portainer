package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*endpointGroupAccessResource)(nil)
	_ resource.ResourceWithConfigure   = (*endpointGroupAccessResource)(nil)
	_ resource.ResourceWithImportState = (*endpointGroupAccessResource)(nil)
)

var ErrEndpointGroupNotFound = errors.New("endpoint group not found")

func newEndpointGroupAccessResource() resource.Resource { return &endpointGroupAccessResource{} }

type endpointGroupAccessResource struct{ client *APIClient }

type endpointGroupAccessModel struct {
	ID              types.String `tfsdk:"id"`
	EndpointGroupID types.Int64  `tfsdk:"endpoint_group_id"`
	TeamID          types.Int64  `tfsdk:"team_id"`
	UserID          types.Int64  `tfsdk:"user_id"`
	RoleID          types.Int64  `tfsdk:"role_id"`
}

func (r *endpointGroupAccessResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint_group_access"
}

func (r *endpointGroupAccessResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the access policy: <endpoint_group_id>:team:<team_id> or <endpoint_group_id>:user:<user_id>.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"endpoint_group_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the Portainer endpoint group to which the access policy applies.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"team_id": schema.Int64Attribute{
				Optional:      true,
				Description:   "Identifier of the team granted access to the endpoint group. Mutually exclusive with `user_id`.",
				PlanModifiers: []planmodifier.Int64{egRequiresReplaceUnlessZero()},
			},
			"user_id": schema.Int64Attribute{
				Optional:      true,
				Description:   "Identifier of the user granted access to the endpoint group. Mutually exclusive with `team_id`.",
				PlanModifiers: []planmodifier.Int64{egRequiresReplaceUnlessZero()},
			},
			"role_id": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
				Description: "Role identifier assigned to the user or team for this endpoint group.",
			},
		},
	}
}

func (r *endpointGroupAccessResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *endpointGroupAccessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan endpointGroupAccessModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Update re-runs Create: the role is written into the group's policy map.
func (r *endpointGroupAccessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan endpointGroupAccessModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// apply merges the team or user policy into the group and writes the whole
// group back, then re-reads it.
func (r *endpointGroupAccessResource) apply(ctx context.Context, m *endpointGroupAccessModel, diags *diag.Diagnostics) {
	groupID := int(m.EndpointGroupID.ValueInt64())
	hasTeam, hasUser := egInt64Set(m.TeamID), egInt64Set(m.UserID)
	teamID, userID := int(m.TeamID.ValueInt64()), int(m.UserID.ValueInt64())
	roleID := int(m.RoleID.ValueInt64())

	if !hasTeam && !hasUser {
		diags.AddError("Failed to apply endpoint group access", "either team_id or user_id must be provided")
		return
	}

	if _, err := getEndpointGroupPolicies(ctx, r.client, groupID); err != nil {
		diags.AddError("Failed to apply endpoint group access", err.Error())
		return
	}

	// PUT /endpoint_groups/{id} replaces the group, so the full object is
	// read and written back with the policy merged in.
	fullObject, err := getEndpointGroupMap(ctx, r.client, groupID)
	if err != nil {
		diags.AddError("Failed to apply endpoint group access", err.Error())
		return
	}
	userPolicies, teamPolicies := endpointGroupPolicyMaps(fullObject)
	if hasTeam {
		teamPolicies[strconv.Itoa(teamID)] = map[string]int{"RoleId": roleID}
	}
	if hasUser {
		userPolicies[strconv.Itoa(userID)] = map[string]int{"RoleId": roleID}
	}
	fullObject["UserAccessPolicies"] = userPolicies
	fullObject["TeamAccessPolicies"] = teamPolicies

	if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/endpoint_groups/%d", r.client.Endpoint, groupID), fullObject, nil); err != nil {
		diags.AddError("Failed to apply endpoint group access", fmt.Sprintf("failed to update endpoint group access: %s", err))
		return
	}

	id := fmt.Sprintf("%d:", groupID)
	if hasTeam {
		id += fmt.Sprintf("team:%d", teamID)
	} else {
		id += fmt.Sprintf("user:%d", userID)
	}
	m.ID = types.StringValue(id)

	// The SDKv2 version re-read the policy here.
	check := *m
	found, err := readEndpointGroupAccess(ctx, r.client, &check)
	if err != nil {
		diags.AddError("Failed to apply endpoint group access", err.Error())
		return
	}
	if !found {
		diags.AddError("Failed to apply endpoint group access", fmt.Sprintf("the access policy %s is missing right after it was written", id))
	}
}

// readEndpointGroupAccess fills role_id from the group's policies. It reports
// false when the group or the policy is gone.
func readEndpointGroupAccess(ctx context.Context, client *APIClient, m *endpointGroupAccessModel) (bool, error) {
	policies, err := getEndpointGroupPolicies(ctx, client, int(m.EndpointGroupID.ValueInt64()))
	if err != nil {
		if errors.Is(err, ErrEndpointGroupNotFound) {
			return false, nil
		}
		return false, err
	}
	var p map[string]int
	var ok bool
	switch {
	case egInt64Set(m.TeamID):
		p, ok = policies.TeamAccessPolicies[strconv.FormatInt(m.TeamID.ValueInt64(), 10)]
	case egInt64Set(m.UserID):
		p, ok = policies.UserAccessPolicies[strconv.FormatInt(m.UserID.ValueInt64(), 10)]
	}
	if !ok {
		return false, nil
	}
	m.RoleID = types.Int64Value(int64(p["RoleId"]))
	return true, nil
}

func (r *endpointGroupAccessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state endpointGroupAccessModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := readEndpointGroupAccess(ctx, r.client, &state)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read endpoint group access", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *endpointGroupAccessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state endpointGroupAccessModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	groupID := int(state.EndpointGroupID.ValueInt64())

	fullObject, err := getEndpointGroupMap(ctx, r.client, groupID)
	if err != nil {
		if errors.Is(err, ErrEndpointGroupNotFound) {
			return
		}
		resp.Diagnostics.AddError("Failed to delete endpoint group access", err.Error())
		return
	}
	userPolicies, teamPolicies := endpointGroupPolicyMaps(fullObject)
	if egInt64Set(state.TeamID) {
		delete(teamPolicies, strconv.FormatInt(state.TeamID.ValueInt64(), 10))
	}
	if egInt64Set(state.UserID) {
		delete(userPolicies, strconv.FormatInt(state.UserID.ValueInt64(), 10))
	}
	fullObject["UserAccessPolicies"] = userPolicies
	fullObject["TeamAccessPolicies"] = teamPolicies

	if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/endpoint_groups/%d", r.client.Endpoint, groupID), fullObject, nil); err != nil {
		if isAPINotFound(err) {
			return
		}
		resp.Diagnostics.AddError("Failed to delete endpoint group access", fmt.Sprintf("failed to delete endpoint group access: %s", err))
	}
}

// ImportState takes "<endpoint_group_id>:team:<team_id>" or
// "<endpoint_group_id>:user:<user_id>", or the "/"-separated form from before
// v3.0.0, and stores the ID in the ":" form.
func (r *endpointGroupAccessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	const format = "<endpoint_group_id>:team:<team_id> or <endpoint_group_id>:user:<user_id>"
	invalid := func() {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("invalid import ID %q: expected %s", req.ID, format))
	}
	parts, err := importIDPartsCompat(req.ID, "/", 3, format)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	groupID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		invalid()
		return
	}
	principalID, err := strconv.ParseInt(parts[2], 10, 64)
	if err != nil {
		invalid()
		return
	}
	var principal string
	switch parts[1] {
	case "team":
		principal = "team_id"
	case "user":
		principal = "user_id"
	default:
		invalid()
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), strings.Join(parts, ":"))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("endpoint_group_id"), groupID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(principal), principalID)...)
}

type EndpointGroupAccessPolicies struct {
	UserAccessPolicies map[string]map[string]int `json:"UserAccessPolicies"`
	TeamAccessPolicies map[string]map[string]int `json:"TeamAccessPolicies"`
}

func getEndpointGroupPolicies(ctx context.Context, client *APIClient, endpointGroupID int) (*EndpointGroupAccessPolicies, error) {
	var policies EndpointGroupAccessPolicies
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/endpoint_groups/%d", client.Endpoint, endpointGroupID), nil, &policies); err != nil {
		if isAPINotFound(err) {
			return nil, ErrEndpointGroupNotFound
		}
		return nil, fmt.Errorf("failed to fetch endpoint group: %w", err)
	}
	if policies.UserAccessPolicies == nil {
		policies.UserAccessPolicies = make(map[string]map[string]int)
	}
	if policies.TeamAccessPolicies == nil {
		policies.TeamAccessPolicies = make(map[string]map[string]int)
	}
	return &policies, nil
}

func getEndpointGroupMap(ctx context.Context, client *APIClient, endpointGroupID int) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/endpoint_groups/%d", client.Endpoint, endpointGroupID), nil, &result); err != nil {
		if isAPINotFound(err) {
			return nil, ErrEndpointGroupNotFound
		}
		return nil, fmt.Errorf("failed to fetch endpoint group: %w", err)
	}
	return result, nil
}

// endpointGroupPolicyMaps returns the group's user and team policy maps,
// empty ones when the group has none.
func endpointGroupPolicyMaps(group map[string]interface{}) (users, teams map[string]interface{}) {
	users, teams = map[string]interface{}{}, map[string]interface{}{}
	if up, ok := group["UserAccessPolicies"].(map[string]interface{}); ok {
		users = up
	}
	if tp, ok := group["TeamAccessPolicies"].(map[string]interface{}); ok {
		teams = tp
	}
	return users, teams
}
