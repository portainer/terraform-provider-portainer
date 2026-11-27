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
	_ datasource.DataSource              = (*teamMembershipsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*teamMembershipsDataSource)(nil)
)

func newTeamMembershipsDataSource() datasource.DataSource { return &teamMembershipsDataSource{} }

type teamMembershipsDataSource struct{ client *APIClient }

type teamMembershipsDataSourceModel struct {
	ID            types.String                `tfsdk:"id"`
	TeamID        types.Int64                 `tfsdk:"team_id"`
	Memberships   []teamMembershipsEntryModel `tfsdk:"memberships"`
	LeaderUserIDs []int64                     `tfsdk:"leader_user_ids"`
}

type teamMembershipsEntryModel struct {
	ID     int64 `tfsdk:"id"`
	UserID int64 `tfsdk:"user_id"`
	Role   int64 `tfsdk:"role"`
}

func (d *teamMembershipsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_memberships"
}

func (d *teamMembershipsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Identifier of the data source, `<team_id>/memberships`."},
			"team_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the team whose members are listed.",
			},
			"memberships": schema.ListAttribute{
				Computed:    true,
				Description: "Members of the team.",
				ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{
					"id":      types.Int64Type,
					"user_id": types.Int64Type,
					"role":    types.Int64Type,
				}},
			},
			"leader_user_ids": schema.ListAttribute{
				Computed:    true,
				ElementType: types.Int64Type,
				Description: "Identifiers of the team's leaders, who may manage its membership.",
			},
		},
	}
}

func (d *teamMembershipsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *teamMembershipsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data teamMembershipsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	teamID := data.TeamID.ValueInt64()

	var list []struct {
		ID     int64 `json:"Id"`
		UserID int64 `json:"UserID"`
		Role   int64 `json:"Role"`
	}
	url := fmt.Sprintf("%s/teams/%d/memberships", d.client.Endpoint, teamID)
	if err := doJSON(ctx, d.client, http.MethodGet, url, nil, &list); err != nil {
		resp.Diagnostics.AddError("Failed to read team memberships", fmt.Errorf("failed to list the memberships of team %d: %w", teamID, err).Error())
		return
	}

	// Role 1 is the team leader in Portainer's membership enum.
	const roleTeamLeader = 1

	data.Memberships = make([]teamMembershipsEntryModel, len(list))
	data.LeaderUserIDs = []int64{}
	for i, m := range list {
		data.Memberships[i] = teamMembershipsEntryModel{ID: m.ID, UserID: m.UserID, Role: m.Role}
		if m.Role == roleTeamLeader {
			data.LeaderUserIDs = append(data.LeaderUserIDs, m.UserID)
		}
	}
	data.ID = types.StringValue(strconv.FormatInt(teamID, 10) + "/memberships")
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
