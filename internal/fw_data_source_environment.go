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
	_ datasource.DataSource              = (*environmentDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*environmentDataSource)(nil)
)

func newEnvironmentDataSource() datasource.DataSource { return &environmentDataSource{} }

type environmentDataSource struct{ client *APIClient }

type environmentDataSourceModel struct {
	ID                 types.String `tfsdk:"id"`
	Name               types.String `tfsdk:"name"`
	Type               types.Int64  `tfsdk:"type"`
	EnvironmentAddress types.String `tfsdk:"environment_address"`
	GroupID            types.Int64  `tfsdk:"group_id"`
}

func (d *environmentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (d *environmentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the environment.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the Portainer environment to look up. The data source will fail if no matching environment is found.",
			},
			"type": schema.Int64Attribute{
				Computed:    true,
				Description: "Type of the Portainer environment: 1 = Docker, 2 = Agent, 3 = Azure, 4 = Edge Agent, 5 = Kubernetes, 6 = Kubernetes via agent, 7 = Kubernetes Edge Agent.",
			},
			"environment_address": schema.StringAttribute{
				Computed:    true,
				Description: "URL or address used by Portainer to reach the environment.",
			},
			"group_id": schema.Int64Attribute{
				Computed:    true,
				Description: "Identifier of the Portainer endpoint group that the environment belongs to.",
			},
		},
	}
}

func (d *environmentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *environmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data environmentDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := data.Name.ValueString()

	envs, err := listEnvironments(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read environment", err.Error())
		return
	}
	for _, e := range envs {
		if e.Name == name {
			data.ID = types.StringValue(strconv.FormatInt(e.ID, 10))
			data.Type = types.Int64Value(e.Type)
			data.EnvironmentAddress = types.StringValue(e.URL)
			data.GroupID = types.Int64Value(e.GroupID)
			resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
			return
		}
	}
	resp.Diagnostics.AddError("Failed to read environment", fmt.Sprintf("environment %s not found", name))
}
