package internal

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*endpointGroupAccessDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*endpointGroupAccessDataSource)(nil)
)

func newEndpointGroupAccessDataSource() datasource.DataSource {
	return &endpointGroupAccessDataSource{}
}

type endpointGroupAccessDataSource struct{ client *APIClient }

type endpointGroupAccessDataSourceModel struct {
	ID              types.String `tfsdk:"id"`
	EndpointGroupID types.Int64  `tfsdk:"endpoint_group_id"`
	TeamID          types.Int64  `tfsdk:"team_id"`
	UserID          types.Int64  `tfsdk:"user_id"`
	RoleID          types.Int64  `tfsdk:"role_id"`
}

func (d *endpointGroupAccessDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint_group_access"
}

func (d *endpointGroupAccessDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the access policy: <endpoint_group_id>/team/<team_id> or <endpoint_group_id>/user/<user_id>.",
			},
			"endpoint_group_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the Portainer endpoint group whose access policy is being looked up.",
			},
			"team_id": schema.Int64Attribute{
				Optional:    true,
				Description: "Identifier of the Portainer team for which the access policy on the endpoint group is returned. Either team_id or user_id must be provided.",
			},
			"user_id": schema.Int64Attribute{
				Optional:    true,
				Description: "Identifier of the Portainer user for which the access policy on the endpoint group is returned. Either team_id or user_id must be provided.",
			},
			"role_id": schema.Int64Attribute{
				Computed:    true,
				Description: "Identifier of the Portainer role granted to the team or user on the endpoint group.",
			},
		},
	}
}

func (d *endpointGroupAccessDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *endpointGroupAccessDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m endpointGroupAccessDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	groupID := m.EndpointGroupID.ValueInt64()
	hasTeam, hasUser := egInt64Set(m.TeamID), egInt64Set(m.UserID)

	if !hasTeam && !hasUser {
		resp.Diagnostics.AddError("Invalid endpoint group access lookup", "either team_id or user_id must be provided")
		return
	}

	policies, err := getEndpointGroupPolicies(ctx, d.client, int(groupID))
	if err != nil {
		if errors.Is(err, ErrEndpointGroupNotFound) {
			resp.Diagnostics.AddError("Endpoint group not found", fmt.Sprintf("endpoint group %d not found", groupID))
			return
		}
		resp.Diagnostics.AddError("Failed to read endpoint group access", err.Error())
		return
	}

	var policy map[string]int
	var found bool
	id := fmt.Sprintf("%d/", groupID)
	if hasTeam {
		tid := strconv.FormatInt(m.TeamID.ValueInt64(), 10)
		policy, found = policies.TeamAccessPolicies[tid]
		id += "team/" + tid
	} else {
		uid := strconv.FormatInt(m.UserID.ValueInt64(), 10)
		policy, found = policies.UserAccessPolicies[uid]
		id += "user/" + uid
	}
	if !found {
		resp.Diagnostics.AddError("Access policy not found", fmt.Sprintf("access policy not found for the given team_id/user_id on endpoint group %d", groupID))
		return
	}

	m.ID = types.StringValue(id)
	m.RoleID = types.Int64Value(int64(policy["RoleId"]))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
