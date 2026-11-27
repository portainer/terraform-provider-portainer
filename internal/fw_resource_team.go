package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*teamResource)(nil)
	_ resource.ResourceWithConfigure   = (*teamResource)(nil)
	_ resource.ResourceWithImportState = (*teamResource)(nil)
	_ resource.ResourceWithIdentity    = (*teamResource)(nil)
)

func newTeamResource() resource.Resource { return &teamResource{} }

type teamResource struct {
	fwIDIdentity
	client *APIClient
}

type teamResourceModel struct {
	ID                  types.String `tfsdk:"id"`
	Name                types.String `tfsdk:"name"`
	DenyPortainerAccess types.Bool   `tfsdk:"deny_portainer_access"`
}

func (r *teamResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team"
}

func (r *teamResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
				Description: "Name of the Portainer team. Must be unique within the Portainer instance.",
			},
			"deny_portainer_access": schema.BoolAttribute{
				Optional: true,
				Description: "Whether members of this team are denied access to Portainer itself (they keep the access to environments the team grants). " +
					"Business Edition 2.45 or later. Left out, Portainer keeps its current value.",
			},
		},
	}
}

func (r *teamResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *teamResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan teamResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	teamName := plan.Name.ValueString()

	// Check if team already exists
	teamList, err := listTeams(ctx, r.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create team", err.Error())
		return
	}
	for _, t := range teamList {
		if t.Name == teamName {
			// Team already exists, perform update
			plan.ID = types.StringValue(strconv.FormatInt(t.ID, 10))
			r.update(ctx, &plan, &resp.Diagnostics)
			if !resp.Diagnostics.HasError() {
				r.readInto(ctx, &plan, &resp.State, &resp.Diagnostics)
			}
			return
		}
	}

	// Team not found, create new
	var created portainerTeam
	body := struct {
		Name                string  `json:"name"`
		TeamLeaders         []int64 `json:"teamLeaders"`
		DenyPortainerAccess *bool   `json:"DenyPortainerAccess,omitempty"`
	}{Name: teamName, DenyPortainerAccess: teamDenyAccessPayload(plan.DenyPortainerAccess)}
	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/teams", body, &created); err != nil {
		resp.Diagnostics.AddError("Failed to create team", fmt.Sprintf("failed to create team: %s", err))
		return
	}
	plan.ID = types.StringValue(strconv.FormatInt(created.ID, 10))
	r.readInto(ctx, &plan, &resp.State, &resp.Diagnostics)
}

// teamDenyAccessPayload is DenyPortainerAccess to send: nil when
// deny_portainer_access is left out, so the request stays as it was.
func teamDenyAccessPayload(v types.Bool) *bool {
	if v.IsNull() || v.IsUnknown() {
		return nil
	}
	return v.ValueBoolPointer()
}

// readInto reads the team after a write and stores it.
func (r *teamResource) readInto(ctx context.Context, m *teamResourceModel, state *tfsdk.State, diags *diag.Diagnostics) {
	planned := m.DenyPortainerAccess
	if found := r.read(ctx, m, diags); diags.HasError() {
		return
	} else if !found {
		diags.AddError("Failed to read team", fmt.Sprintf("team %s disappeared right after it was written", m.ID.ValueString()))
		return
	}
	diags.Append(state.Set(ctx, m)...)
	// Community Edition accepts the field and ignores it. The team is stored
	// first, so a created one is kept in state (tainted) rather than lost.
	if !planned.IsNull() && !m.DenyPortainerAccess.Equal(planned) {
		diags.AddError("Failed to update team", fmt.Sprintf("Portainer did not apply deny_portainer_access = %t to team %s; it requires %s",
			planned.ValueBool(), m.ID.ValueString(), requiresBE245))
	}
}

func (r *teamResource) read(ctx context.Context, m *teamResourceModel, diags *diag.Diagnostics) bool {
	id, _ := strconv.ParseInt(m.ID.ValueString(), 10, 64)
	var team struct {
		portainerTeam
		DenyPortainerAccess bool `json:"DenyPortainerAccess"`
	}
	if err := doJSON(ctx, r.client, http.MethodGet, fmt.Sprintf("%s/teams/%d", r.client.Endpoint, id), nil, &team); err != nil {
		if !isAPINotFound(err) {
			diags.AddError("Failed to read team", fmt.Sprintf("failed to read team: %s", err))
		}
		return false
	}
	m.Name = types.StringValue(team.Name)
	// Read back only while managed, so a team without it plans no change.
	if !m.DenyPortainerAccess.IsNull() {
		m.DenyPortainerAccess = types.BoolValue(team.DenyPortainerAccess)
	}
	return true
}

func (r *teamResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	fwSetIDIdentity(ctx, req.State, resp.Identity, &resp.Diagnostics)
	var state teamResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found := r.read(ctx, &state, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *teamResource) update(ctx context.Context, m *teamResourceModel, diags *diag.Diagnostics) {
	id, _ := strconv.ParseInt(m.ID.ValueString(), 10, 64)
	body := struct {
		Name                string `json:"name,omitempty"`
		DenyPortainerAccess *bool  `json:"DenyPortainerAccess,omitempty"`
	}{Name: m.Name.ValueString(), DenyPortainerAccess: teamDenyAccessPayload(m.DenyPortainerAccess)}
	if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/teams/%d", r.client.Endpoint, id), body, nil); err != nil {
		diags.AddError("Failed to update team", fmt.Sprintf("failed to update team: %s", err))
	}
}

func (r *teamResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan, state teamResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	r.update(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	r.readInto(ctx, &plan, &resp.State, &resp.Diagnostics)
}

func (r *teamResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state teamResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, _ := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err := doJSON(ctx, r.client, http.MethodDelete, fmt.Sprintf("%s/teams/%d", r.client.Endpoint, id), nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete team", fmt.Sprintf("failed to delete team: %s", err))
	}
}

func (r *teamResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	fwImportIDOrIdentity(ctx, req, resp)
}
