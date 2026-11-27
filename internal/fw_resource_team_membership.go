package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*teamMembershipResource)(nil)
	_ resource.ResourceWithConfigure   = (*teamMembershipResource)(nil)
	_ resource.ResourceWithImportState = (*teamMembershipResource)(nil)
)

func newTeamMembershipResource() resource.Resource { return &teamMembershipResource{} }

type teamMembershipResource struct{ client *APIClient }

type teamMembershipResourceModel struct {
	ID     types.String `tfsdk:"id"`
	Role   types.Int64  `tfsdk:"role"`
	TeamID types.Int64  `tfsdk:"team_id"`
	UserID types.Int64  `tfsdk:"user_id"`
}

func (r *teamMembershipResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_membership"
}

func (r *teamMembershipResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"role": schema.Int64Attribute{
				Required:    true,
				Description: "Membership role: 1 = team leader, 2 = regular member",
				Validators:  []validator.Int64{int64validator.Between(1, 2)},
			},
			"team_id": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the team",
			},
			"user_id": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the user",
			},
		},
	}
}

func (r *teamMembershipResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *teamMembershipResource) payload(m teamMembershipResourceModel) teamMembershipPayload {
	return teamMembershipPayload{Role: m.Role.ValueInt64(), TeamID: m.TeamID.ValueInt64(), UserID: m.UserID.ValueInt64()}
}

func (r *teamMembershipResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan teamMembershipResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var created portainerTeamMembership
	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/team_memberships", r.payload(plan), &created); err != nil {
		resp.Diagnostics.AddError("Failed to create team membership", fmt.Sprintf("failed to create team membership: %s", err))
		return
	}
	plan.ID = types.StringValue(strconv.FormatInt(created.ID, 10))
	r.readInto(ctx, &plan, &resp.State, &resp.Diagnostics)
}

func (r *teamMembershipResource) readInto(ctx context.Context, m *teamMembershipResourceModel, state *tfsdk.State, diags *diag.Diagnostics) {
	if found := r.read(ctx, m, diags); diags.HasError() {
		return
	} else if !found {
		diags.AddError("Failed to read team membership", fmt.Sprintf("team membership %s disappeared right after it was written", m.ID.ValueString()))
		return
	}
	diags.Append(state.Set(ctx, m)...)
}

func (r *teamMembershipResource) read(ctx context.Context, m *teamMembershipResourceModel, diags *diag.Diagnostics) bool {
	id, _ := strconv.ParseInt(m.ID.ValueString(), 10, 64)
	memberships, err := listTeamMemberships(ctx, r.client)
	if err != nil {
		diags.AddError("Failed to read team membership", err.Error())
		return false
	}
	for _, tm := range memberships {
		if tm.ID == id {
			m.Role = types.Int64Value(tm.Role)
			m.TeamID = types.Int64Value(tm.TeamID)
			m.UserID = types.Int64Value(tm.UserID)
			return true
		}
	}
	return false
}

func (r *teamMembershipResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state teamMembershipResourceModel
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

func (r *teamMembershipResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state teamMembershipResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	id, _ := strconv.ParseInt(plan.ID.ValueString(), 10, 64)
	if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/team_memberships/%d", r.client.Endpoint, id), r.payload(plan), nil); err != nil {
		resp.Diagnostics.AddError("Failed to update team membership", fmt.Sprintf("failed to update team membership: %s", err))
		return
	}
	r.readInto(ctx, &plan, &resp.State, &resp.Diagnostics)
}

func (r *teamMembershipResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state teamMembershipResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, _ := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err := doJSON(ctx, r.client, http.MethodDelete, fmt.Sprintf("%s/team_memberships/%d", r.client.Endpoint, id), nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete team membership", fmt.Sprintf("failed to delete team membership: %s", err))
	}
}

// ImportState takes the membership ID; Terraform then reads it, which fails
// the import when the membership does not exist.
func (r *teamMembershipResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
