package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*userAccessDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*userAccessDataSource)(nil)
)

func newUserAccessDataSource() datasource.DataSource { return &userAccessDataSource{} }

type userAccessDataSource struct{ client *APIClient }

type userAccessDataSourceModel struct {
	ID              types.String                `tfsdk:"id"`
	UserID          types.Int64                 `tfsdk:"user_id"`
	Username        types.String                `tfsdk:"username"`
	Role            types.Int64                 `tfsdk:"role"`
	Memberships     []userAccessMembershipModel `tfsdk:"memberships"`
	EffectiveAccess []userAccessEffectiveModel  `tfsdk:"effective_access"`
}

type userAccessMembershipModel struct {
	ID     int64 `tfsdk:"id"`
	TeamID int64 `tfsdk:"team_id"`
	Role   int64 `tfsdk:"role"`
}

type userAccessEffectiveModel struct {
	EndpointID     int64  `tfsdk:"endpoint_id"`
	EndpointName   string `tfsdk:"endpoint_name"`
	GroupID        int64  `tfsdk:"group_id"`
	GroupName      string `tfsdk:"group_name"`
	TeamID         int64  `tfsdk:"team_id"`
	TeamName       string `tfsdk:"team_name"`
	RoleID         int64  `tfsdk:"role_id"`
	RoleName       string `tfsdk:"role_name"`
	RolePriority   int64  `tfsdk:"role_priority"`
	AccessLocation string `tfsdk:"access_location"`
}

func (d *userAccessDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_access"
}

// Schema: user_id is optional+computed (SDKv2 declared it optional only, but
// wrote the resolved caller's ID into it, which the framework allows only for
// a computed attribute).
func (d *userAccessDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Identifier of the data source, `<user_id>/access`."},
			"user_id": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Description: "Identifier of the user to inspect. Leave unset to describe the user the provider authenticates as.",
			},
			"username": schema.StringAttribute{
				Computed:    true,
				Description: "Username of the inspected user.",
			},
			"role": schema.Int64Attribute{
				Computed:    true,
				Description: "Portainer role of the user: 1 = administrator, 2 = standard user.",
			},
			"memberships": schema.ListAttribute{
				Computed:    true,
				Description: "Team memberships of the user.",
				ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{
					"id":      types.Int64Type,
					"team_id": types.Int64Type,
					"role":    types.Int64Type,
				}},
			},
			"effective_access": schema.ListAttribute{
				Computed:    true,
				Description: "Access the user effectively has on every environment, with the policy each entry comes from. This is the resolved result of user policies, team policies and group inheritance — the answer to \"why can this person see that environment\".",
				ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{
					"endpoint_id":     types.Int64Type,
					"endpoint_name":   types.StringType,
					"group_id":        types.Int64Type,
					"group_name":      types.StringType,
					"team_id":         types.Int64Type,
					"team_name":       types.StringType,
					"role_id":         types.Int64Type,
					"role_name":       types.StringType,
					"role_priority":   types.Int64Type,
					"access_location": types.StringType,
				}},
			},
		},
	}
}

func (d *userAccessDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *userAccessDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data userAccessDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var user struct {
		ID       int64  `json:"Id"`
		Username string `json:"Username"`
		Role     int64  `json:"Role"`
	}

	// A zero user_id counts as unset, as SDKv2's GetOk treated it.
	if userID := data.UserID.ValueInt64(); userID != 0 {
		url := fmt.Sprintf("%s/users/%d", d.client.Endpoint, userID)
		if err := doJSON(ctx, d.client, http.MethodGet, url, nil, &user); err != nil {
			resp.Diagnostics.AddError("Failed to read user access", fmt.Errorf("failed to read user %d: %w", userID, err).Error())
			return
		}
	} else {
		// Without an explicit ID the endpoint resolves the caller, which is how
		// a configuration learns which account its API key belongs to.
		if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/users/me", nil, &user); err != nil {
			resp.Diagnostics.AddError("Failed to read user access", fmt.Errorf("failed to read the current user: %w", err).Error())
			return
		}
	}
	if user.ID == 0 {
		resp.Diagnostics.AddError("Failed to read user access", "failed to resolve the user: Portainer returned no identifier")
		return
	}

	var rawMemberships []struct {
		ID     int64 `json:"Id"`
		TeamID int64 `json:"TeamID"`
		Role   int64 `json:"Role"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, fmt.Sprintf("%s/users/%d/memberships", d.client.Endpoint, user.ID), nil, &rawMemberships); err != nil {
		resp.Diagnostics.AddError("Failed to read user access", fmt.Errorf("failed to read the memberships of user %d: %w", user.ID, err).Error())
		return
	}
	data.Memberships = make([]userAccessMembershipModel, len(rawMemberships))
	for i, m := range rawMemberships {
		data.Memberships[i] = userAccessMembershipModel{ID: m.ID, TeamID: m.TeamID, Role: m.Role}
	}

	var rawAccess []struct {
		EndpointID     int64  `json:"endpointId"`
		EndpointName   string `json:"endpointName"`
		GroupID        int64  `json:"groupId"`
		GroupName      string `json:"groupName"`
		TeamID         int64  `json:"teamId"`
		TeamName       string `json:"teamName"`
		RoleID         int64  `json:"roleId"`
		RoleName       string `json:"roleName"`
		RolePriority   int64  `json:"rolePriority"`
		AccessLocation string `json:"accessLocation"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, fmt.Sprintf("%s/users/%d/effective-access", d.client.Endpoint, user.ID), nil, &rawAccess); err != nil {
		err = explainUnsupported(ctx, d.client, err, "portainer_user_access", requires242)
		resp.Diagnostics.AddError("Failed to read user access", fmt.Errorf("failed to read the effective access of user %d: %w", user.ID, err).Error())
		return
	}
	data.EffectiveAccess = make([]userAccessEffectiveModel, len(rawAccess))
	for i, a := range rawAccess {
		data.EffectiveAccess[i] = userAccessEffectiveModel{
			EndpointID: a.EndpointID, EndpointName: a.EndpointName,
			GroupID: a.GroupID, GroupName: a.GroupName,
			TeamID: a.TeamID, TeamName: a.TeamName,
			RoleID: a.RoleID, RoleName: a.RoleName, RolePriority: a.RolePriority,
			AccessLocation: a.AccessLocation,
		}
	}

	data.UserID = types.Int64Value(user.ID)
	data.Username = types.StringValue(user.Username)
	data.Role = types.Int64Value(user.Role)
	data.ID = types.StringValue(strconv.FormatInt(user.ID, 10) + "/access")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
