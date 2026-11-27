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
	_ datasource.DataSource              = (*endpointGroupDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*endpointGroupDataSource)(nil)
)

func newEndpointGroupDataSource() datasource.DataSource { return &endpointGroupDataSource{} }

type endpointGroupDataSource struct{ client *APIClient }

type endpointGroupDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
}

func (d *endpointGroupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_endpoint_group"
}

func (d *endpointGroupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the endpoint group.",
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the Portainer endpoint group to look up. The data source will fail if no matching endpoint group is found.",
			},
			"description": schema.StringAttribute{
				Computed:    true,
				Description: "Description of the Portainer endpoint group as configured in Portainer.",
			},
		},
	}
}

func (d *endpointGroupDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *endpointGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m endpointGroupDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := m.Name.ValueString()

	groups, err := listEndpointGroups(ctx, d.client)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read endpoint group", err.Error())
		return
	}
	for _, g := range groups {
		if g.Name == name {
			m.ID = types.StringValue(strconv.FormatInt(g.ID, 10))
			m.Description = types.StringValue(g.Description)
			resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
			return
		}
	}
	resp.Diagnostics.AddError("Endpoint group not found", fmt.Sprintf("endpoint group %s not found", name))
}
