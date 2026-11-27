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

var _ datasource.DataSourceWithConfigure = (*registryAccessDataSource)(nil)

func newRegistryAccessDataSource() datasource.DataSource { return &registryAccessDataSource{} }

type registryAccessDataSource struct{ client *APIClient }

type registryAccessDataSourceModel struct {
	ID         types.String `tfsdk:"id"`
	RegistryID types.Int64  `tfsdk:"registry_id"`
	EndpointID types.Int64  `tfsdk:"endpoint_id"`
	TeamID     types.Int64  `tfsdk:"team_id"`
	UserID     types.Int64  `tfsdk:"user_id"`
	RoleID     types.Int64  `tfsdk:"role_id"`
}

func (d *registryAccessDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_registry_access"
}

func (d *registryAccessDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Computed: true, Description: "Identifier of the access policy: <registry_id>/<endpoint_id>/team/<team_id> or <registry_id>/<endpoint_id>/user/<user_id>."},
			"registry_id": schema.Int64Attribute{Required: true, Description: "Identifier of the Portainer registry whose access policy is queried."},
			"endpoint_id": schema.Int64Attribute{Required: true, Description: "Identifier of the Portainer endpoint to which the registry access policy applies."},
			"team_id":     schema.Int64Attribute{Optional: true, Description: "Identifier of the team whose access policy should be returned. Mutually exclusive with `user_id`."},
			"user_id":     schema.Int64Attribute{Optional: true, Description: "Identifier of the user whose access policy should be returned. Mutually exclusive with `team_id`."},
			"role_id":     schema.Int64Attribute{Computed: true, Description: "Role identifier assigned to the user or team for the registry on the given endpoint."},
		},
	}
}

func (d *registryAccessDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *registryAccessDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data registryAccessDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	registryID := int(data.RegistryID.ValueInt64())
	endpointID := int(data.EndpointID.ValueInt64())
	hasTeam, hasUser := egInt64Set(data.TeamID), egInt64Set(data.UserID)
	if !hasTeam && !hasUser {
		resp.Diagnostics.AddError("Failed to read registry access", "either team_id or user_id must be provided")
		return
	}

	policies, err := getRegistryPolicies(ctx, d.client, registryID, endpointID)
	if err != nil {
		if errors.Is(err, ErrRegistryNotFound) {
			resp.Diagnostics.AddError("Failed to read registry access", fmt.Sprintf("registry %d not found", registryID))
			return
		}
		resp.Diagnostics.AddError("Failed to read registry access", err.Error())
		return
	}

	id := fmt.Sprintf("%d/%d/", registryID, endpointID)
	var p accessPolicy
	var found bool
	if hasTeam {
		tid := strconv.FormatInt(data.TeamID.ValueInt64(), 10)
		p, found = policies.TeamAccessPolicies[tid]
		id += "team/" + tid
	} else {
		uid := strconv.FormatInt(data.UserID.ValueInt64(), 10)
		p, found = policies.UserAccessPolicies[uid]
		id += "user/" + uid
	}
	if !found {
		resp.Diagnostics.AddError("Failed to read registry access", fmt.Sprintf("access policy not found for the given team_id/user_id on registry %d and endpoint %d", registryID, endpointID))
		return
	}
	data.ID = types.StringValue(id)
	data.RoleID = types.Int64Value(p.RoleID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
