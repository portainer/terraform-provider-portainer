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

var ErrRegistryNotFound = errors.New("registry not found")

var (
	_ resource.Resource                = (*registryAccessResource)(nil)
	_ resource.ResourceWithConfigure   = (*registryAccessResource)(nil)
	_ resource.ResourceWithImportState = (*registryAccessResource)(nil)
)

func newRegistryAccessResource() resource.Resource { return &registryAccessResource{} }

type registryAccessResource struct{ client *APIClient }

type registryAccessModel struct {
	ID         types.String `tfsdk:"id"`
	RegistryID types.Int64  `tfsdk:"registry_id"`
	EndpointID types.Int64  `tfsdk:"endpoint_id"`
	TeamID     types.Int64  `tfsdk:"team_id"`
	UserID     types.Int64  `tfsdk:"user_id"`
	RoleID     types.Int64  `tfsdk:"role_id"`
}

func (r *registryAccessResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_registry_access"
}

func (r *registryAccessResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the access policy: <registry_id>:<endpoint_id>:team:<team_id> or <registry_id>:<endpoint_id>:user:<user_id>.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"registry_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the Portainer registry whose access policy is being managed.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"endpoint_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Identifier of the Portainer endpoint to which the registry access policy applies.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"team_id": schema.Int64Attribute{
				Optional:      true,
				Description:   "Identifier of the team granted access to the registry. Mutually exclusive with `user_id`.",
				PlanModifiers: []planmodifier.Int64{egRequiresReplaceUnlessZero()},
			},
			"user_id": schema.Int64Attribute{
				Optional:      true,
				Description:   "Identifier of the user granted access to the registry. Mutually exclusive with `team_id`.",
				PlanModifiers: []planmodifier.Int64{egRequiresReplaceUnlessZero()},
			},
			"role_id": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
				Description: "Role identifier assigned to the user or team for the registry on the given endpoint.",
			},
		},
	}
}

func (r *registryAccessResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// ImportState accepts <registry_id>:<endpoint_id>:team:<team_id> or
// <registry_id>:<endpoint_id>:user:<user_id>, or the same with "/" from before
// v3.0.0.
func (r *registryAccessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	const format = "<registry_id>:<endpoint_id>:team:<team_id> or <registry_id>:<endpoint_id>:user:<user_id>"
	parts, err := importIDPartsCompat(req.ID, "/", 4, format)
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", err.Error())
		return
	}
	invalid := fmt.Sprintf("invalid import ID %q: expected %s", req.ID, format)
	var ids [3]int64
	for i, raw := range []string{parts[0], parts[1], parts[3]} {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			resp.Diagnostics.AddError("Invalid import ID", invalid)
			return
		}
		ids[i] = n
	}
	var principal string
	switch parts[2] {
	case "team":
		principal = "team_id"
	case "user":
		principal = "user_id"
	default:
		resp.Diagnostics.AddError("Invalid import ID", invalid)
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), strings.Join(parts, ":"))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("registry_id"), ids[0])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("endpoint_id"), ids[1])...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root(principal), ids[2])...)
}

func (r *registryAccessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan registryAccessModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, &plan, &resp.State, &resp.Diagnostics)
}

// Update sends the same request as Create.
func (r *registryAccessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan registryAccessModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.apply(ctx, &plan, &resp.State, &resp.Diagnostics)
}

type registryAccessStateSetter interface {
	Set(ctx context.Context, val interface{}) diag.Diagnostics
}

func (r *registryAccessResource) apply(ctx context.Context, plan *registryAccessModel, state registryAccessStateSetter, diags *diag.Diagnostics) {
	registryID := int(plan.RegistryID.ValueInt64())
	endpointID := int(plan.EndpointID.ValueInt64())
	hasTeam, hasUser := egInt64Set(plan.TeamID), egInt64Set(plan.UserID)
	roleID := plan.RoleID.ValueInt64()

	if !hasTeam && !hasUser {
		diags.AddError("Failed to update registry access", "either team_id or user_id must be provided")
		return
	}

	policies, err := getRegistryPolicies(ctx, r.client, registryID, endpointID)
	if err != nil {
		diags.AddError("Failed to update registry access", err.Error())
		return
	}
	if hasTeam {
		policies.TeamAccessPolicies[strconv.FormatInt(plan.TeamID.ValueInt64(), 10)] = accessPolicy{RoleID: roleID}
	}
	if hasUser {
		policies.UserAccessPolicies[strconv.FormatInt(plan.UserID.ValueInt64(), 10)] = accessPolicy{RoleID: roleID}
	}
	if err := putRegistryAccess(ctx, r.client, registryID, endpointID, policies); err != nil {
		diags.AddError("Failed to update registry access", fmt.Errorf("failed to update registry access: %w", err).Error())
		return
	}

	id := fmt.Sprintf("%d:%d:", registryID, endpointID)
	if hasTeam {
		id += fmt.Sprintf("team:%d", plan.TeamID.ValueInt64())
	} else {
		id += fmt.Sprintf("user:%d", plan.UserID.ValueInt64())
	}
	plan.ID = types.StringValue(id)

	m := *plan
	found, err := registryAccessRead(ctx, r.client, &m)
	if err != nil {
		diags.AddError("Failed to read registry access", err.Error())
		return
	}
	if !found {
		diags.AddError("Failed to read registry access", fmt.Sprintf("the access policy %s was not found after it was written", id))
		return
	}
	diags.Append(state.Set(ctx, plan)...)
}

func (r *registryAccessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state registryAccessModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := registryAccessRead(ctx, r.client, &state)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read registry access", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// registryAccessRead refreshes role_id; false when the registry or the
// policy is gone.
func registryAccessRead(ctx context.Context, client *APIClient, m *registryAccessModel) (bool, error) {
	policies, err := getRegistryPolicies(ctx, client, int(m.RegistryID.ValueInt64()), int(m.EndpointID.ValueInt64()))
	if err != nil {
		if errors.Is(err, ErrRegistryNotFound) {
			return false, nil
		}
		return false, err
	}
	var p accessPolicy
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
	m.RoleID = types.Int64Value(p.RoleID)
	return true, nil
}

func (r *registryAccessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state registryAccessModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	registryID := int(state.RegistryID.ValueInt64())
	endpointID := int(state.EndpointID.ValueInt64())

	policies, err := getRegistryPolicies(ctx, r.client, registryID, endpointID)
	if err != nil {
		if errors.Is(err, ErrRegistryNotFound) {
			return
		}
		resp.Diagnostics.AddError("Failed to delete registry access", err.Error())
		return
	}
	if egInt64Set(state.TeamID) {
		delete(policies.TeamAccessPolicies, strconv.FormatInt(state.TeamID.ValueInt64(), 10))
	}
	if egInt64Set(state.UserID) {
		delete(policies.UserAccessPolicies, strconv.FormatInt(state.UserID.ValueInt64(), 10))
	}
	if err := putRegistryAccess(ctx, r.client, registryID, endpointID, policies); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete registry access", fmt.Errorf("failed to delete registry access: %w", err).Error())
	}
}

// registryAccessPolicies is a registry's access on one environment, as
// Portainer reports it in Registry.RegistryAccesses.
type registryAccessPolicies struct {
	UserAccessPolicies map[string]accessPolicy `json:"UserAccessPolicies"`
	TeamAccessPolicies map[string]accessPolicy `json:"TeamAccessPolicies"`
	Namespaces         []string                `json:"Namespaces"`
}

type accessPolicy struct {
	RoleID int64 `json:"RoleId,omitempty"`
}

// registryAccessPayload is the PUT /endpoints/{id}/registries/{registryId}
// body.
type registryAccessPayload struct {
	UserAccessPolicies map[string]accessPolicy `json:"userAccessPolicies,omitempty"`
	TeamAccessPolicies map[string]accessPolicy `json:"teamAccessPolicies,omitempty"`
	Namespaces         []string                `json:"namespaces"`
}

func getRegistryPolicies(ctx context.Context, client *APIClient, registryID int, endpointID int) (*registryAccessPolicies, error) {
	registry, err := getRegistry(ctx, client, int64(registryID))
	if err != nil {
		if isAPINotFound(err) {
			return nil, ErrRegistryNotFound
		}
		return nil, fmt.Errorf("failed to fetch registry: %w", err)
	}

	policies, ok := registry.RegistryAccesses[strconv.Itoa(endpointID)]
	if !ok {
		return &registryAccessPolicies{
			UserAccessPolicies: make(map[string]accessPolicy),
			TeamAccessPolicies: make(map[string]accessPolicy),
		}, nil
	}
	if policies.UserAccessPolicies == nil {
		policies.UserAccessPolicies = make(map[string]accessPolicy)
	}
	if policies.TeamAccessPolicies == nil {
		policies.TeamAccessPolicies = make(map[string]accessPolicy)
	}
	return &policies, nil
}

// putRegistryAccess replaces the registry's access on the environment.
func putRegistryAccess(ctx context.Context, client *APIClient, registryID, endpointID int, policies *registryAccessPolicies) error {
	body := registryAccessPayload{
		UserAccessPolicies: policies.UserAccessPolicies,
		TeamAccessPolicies: policies.TeamAccessPolicies,
		Namespaces:         policies.Namespaces,
	}
	return doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/endpoints/%d/registries/%d", client.Endpoint, endpointID, registryID), body, nil)
}
