package internal

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// addonAccessResource owns the whole access configuration of one addon.
// PUT /addons/{id}/access replaces the stored policies wholesale, so a
// per-grant resource would have each instance revoking the others' grants;
// one resource per addon is the only shape that matches the endpoint.
var (
	_ resource.Resource                = (*addonAccessResource)(nil)
	_ resource.ResourceWithConfigure   = (*addonAccessResource)(nil)
	_ resource.ResourceWithImportState = (*addonAccessResource)(nil)
)

func newAddonAccessResource() resource.Resource { return &addonAccessResource{} }

type addonAccessResource struct{ client *APIClient }

type addonAccessModel struct {
	ID         types.String `tfsdk:"id"`
	AddonID    types.String `tfsdk:"addon_id"`
	Enabled    types.Bool   `tfsdk:"enabled"`
	UserAccess types.Set    `tfsdk:"user_access"`
	TeamAccess types.Set    `tfsdk:"team_access"`
}

func addonAccessGrantType(idField string) types.ObjectType {
	return types.ObjectType{AttrTypes: map[string]attr.Type{idField: types.Int64Type, "role_id": types.Int64Type}}
}

func (r *addonAccessResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_addon_access"
}

func (r *addonAccessResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	grantBlock := func(desc, idField, idDesc, roleDesc string) schema.SetNestedBlock {
		return schema.SetNestedBlock{
			Description: desc,
			NestedObject: schema.NestedBlockObject{
				Attributes: map[string]schema.Attribute{
					idField:   schema.Int64Attribute{Required: true, Description: idDesc},
					"role_id": schema.Int64Attribute{Required: true, Description: roleDesc},
				},
			},
		}
	}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"addon_id": schema.StringAttribute{
				Required:      true,
				Description:   "Catalog identifier of the addon whose access configuration this resource owns, for example `portal-template`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether the addon is switched on. A disabled addon stays installed but is hidden from users.",
			},
		},
		Blocks: map[string]schema.Block{
			"user_access": grantBlock("Users granted access to the addon. The set is authoritative: a user missing from it has no grant.",
				"user_id", "Identifier of the user granted access.", "Identifier of the role the user is granted on the addon."),
			"team_access": grantBlock("Teams granted access to the addon. The set is authoritative: a team missing from it has no grant.",
				"team_id", "Identifier of the team granted access.", "Identifier of the role the team is granted on the addon."),
		},
	}
}

func (r *addonAccessResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// addonAccessPoliciesFromSet turns a user_access/team_access block into the
// {"<id>": {"RoleId": n}} object Portainer stores. The result is never nil:
// an empty object is how the API is told to revoke every grant, which is
// exactly what an empty set in the configuration means.
func addonAccessPoliciesFromSet(set types.Set, idField string) map[string]map[string]int {
	policies := map[string]map[string]int{}
	if set.IsNull() || set.IsUnknown() {
		return policies
	}
	for _, elem := range set.Elements() {
		obj, ok := elem.(types.Object)
		if !ok {
			continue
		}
		attrs := obj.Attributes()
		id, _ := attrs[idField].(types.Int64)
		role, _ := attrs["role_id"].(types.Int64)
		policies[strconv.FormatInt(id.ValueInt64(), 10)] = map[string]int{"RoleId": int(role.ValueInt64())}
	}
	return policies
}

// addonAccessPoliciesToSet is the inverse, for reading state back.
func addonAccessPoliciesToSet(policies map[string]map[string]int, idField string) types.Set {
	objType := addonAccessGrantType(idField)
	keys := make([]string, 0, len(policies))
	for k := range policies {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	elems := make([]attr.Value, 0, len(policies))
	for _, idStr := range keys {
		id, err := strconv.Atoi(idStr)
		if err != nil {
			continue
		}
		elems = append(elems, types.ObjectValueMust(objType.AttrTypes, map[string]attr.Value{
			idField:   types.Int64Value(int64(id)),
			"role_id": types.Int64Value(int64(policies[idStr]["RoleId"])),
		}))
	}
	return types.SetValueMust(objType, elems)
}

func (r *addonAccessResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.write(ctx, req.Plan, &resp.State, &resp.Diagnostics)
}

func (r *addonAccessResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.write(ctx, req.Plan, &resp.State, &resp.Diagnostics)
}

func (r *addonAccessResource) write(ctx context.Context, plan tfsdk.Plan, state *tfsdk.State, diags *diag.Diagnostics) {
	var m addonAccessModel
	diags.Append(plan.Get(ctx, &m)...)
	if diags.HasError() {
		return
	}
	addonID := m.AddonID.ValueString()

	payload := map[string]interface{}{
		"enabled":            m.Enabled.ValueBool(),
		"userAccessPolicies": addonAccessPoliciesFromSet(m.UserAccess, "user_id"),
		"teamAccessPolicies": addonAccessPoliciesFromSet(m.TeamAccess, "team_id"),
	}

	endpoint := addonURL(r.client, addonID) + "/access"
	if err := doJSON(ctx, r.client, http.MethodPut, endpoint, payload, nil); err != nil {
		addonDiagError(diags, fmt.Errorf("failed to update the access configuration of addon %s: %w", addonID, err))
		return
	}
	m.ID = types.StringValue(addonID)

	// Read back as the SDKv2 version did; what was just written is what the
	// state records.
	if _, err := readAddon(ctx, r.client, addonID); err != nil {
		addonDiagError(diags, fmt.Errorf("failed to read addon %s: %w", addonID, err))
		return
	}
	m.UserAccess = addonAccessNormalizeSet(m.UserAccess, "user_id")
	m.TeamAccess = addonAccessNormalizeSet(m.TeamAccess, "team_id")
	diags.Append(state.Set(ctx, &m)...)
}

// addonAccessNormalizeSet gives a null block set the empty set SDKv2 stored.
func addonAccessNormalizeSet(set types.Set, idField string) types.Set {
	if set.IsNull() || set.IsUnknown() {
		return types.SetValueMust(addonAccessGrantType(idField), nil)
	}
	return set
}

func (r *addonAccessResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m addonAccessModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := m.ID.ValueString()

	addon, err := readAddon(ctx, r.client, id)
	if err != nil {
		addonDiagError(&resp.Diagnostics, fmt.Errorf("failed to read addon %s: %w", id, err))
		return
	}
	if addon == nil {
		resp.State.RemoveResource(ctx)
		return
	}

	m.AddonID = types.StringValue(addon.ID)
	m.Enabled = types.BoolValue(addon.Enabled)
	// The policies live on the admin-only record. Read by a token that cannot
	// see it, the grants are simply left as configured rather than wiped from
	// state on the strength of a response that never carried them.
	if rec := addon.Record; rec != nil {
		m.UserAccess = addonAccessPoliciesToSet(rec.UserAccessPolicies, "user_id")
		m.TeamAccess = addonAccessPoliciesToSet(rec.TeamAccessPolicies, "team_id")
	}
	m.UserAccess = addonAccessNormalizeSet(m.UserAccess, "user_id")
	m.TeamAccess = addonAccessNormalizeSet(m.TeamAccess, "team_id")
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *addonAccessResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m addonAccessModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := m.ID.ValueString()

	// Destroying the resource revokes the grants it created. `enabled` is
	// deliberately left out of the payload: turning the addon off is a
	// different decision than giving up managing who may use it.
	payload := map[string]interface{}{
		"userAccessPolicies": map[string]map[string]int{},
		"teamAccessPolicies": map[string]map[string]int{},
	}

	endpoint := addonURL(r.client, id) + "/access"
	if err := doJSON(ctx, r.client, http.MethodPut, endpoint, payload, nil); err != nil && !isAPINotFound(err) {
		addonDiagError(&resp.Diagnostics, fmt.Errorf("failed to revoke the access grants of addon %s: %w", id, err))
	}
}

func (r *addonAccessResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
