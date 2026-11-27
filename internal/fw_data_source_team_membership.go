package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*teamMembershipDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*teamMembershipDataSource)(nil)
)

func newTeamMembershipDataSource() datasource.DataSource { return &teamMembershipDataSource{} }

type teamMembershipDataSource struct{ client *APIClient }

type teamMembershipDataSourceModel struct {
	ID     types.String `tfsdk:"id"`
	TeamID types.Int64  `tfsdk:"team_id"`
	UserID types.Int64  `tfsdk:"user_id"`
	Role   types.Int64  `tfsdk:"role"`
}

func (d *teamMembershipDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_team_membership"
}

func (d *teamMembershipDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Identifier of the team membership."},
			"team_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the Portainer team the membership belongs to. Combined with user_id to find a specific membership.",
			},
			"user_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the Portainer user whose membership in the team is being looked up.",
			},
			"role": schema.Int64Attribute{
				Computed:    true,
				Description: "Role of the user within the team as returned by Portainer (e.g. team leader, member).",
			},
		},
	}
}

func (d *teamMembershipDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *teamMembershipDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data teamMembershipDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	teamID := data.TeamID.ValueInt64()
	userID := data.UserID.ValueInt64()

	memberships, err := listTeamMemberships(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read team membership", err.Error())
		return
	}
	for _, m := range memberships {
		if m.TeamID == teamID && m.UserID == userID {
			data.ID = types.StringValue(strconv.FormatInt(m.ID, 10))
			data.Role = types.Int64Value(m.Role)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}
	resp.Diagnostics.AddError("Failed to read team membership", fmt.Sprintf("team membership not found for team_id %d and user_id %d", teamID, userID))
}
