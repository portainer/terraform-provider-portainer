package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var _ datasource.DataSourceWithConfigure = (*edgeConfigurationDataSource)(nil)

func newEdgeConfigurationDataSource() datasource.DataSource { return &edgeConfigurationDataSource{} }

// edgeConfigurationDataSource looks up an edge configuration by name.
type edgeConfigurationDataSource struct{ edgeDataSourceBase }

type edgeConfigurationDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Name     types.String `tfsdk:"name"`
	Type     types.Int64  `tfsdk:"type"`
	Category types.String `tfsdk:"category"`
}

func (d *edgeConfigurationDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_configuration"
}

func (d *edgeConfigurationDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Identifier of the edge configuration."},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the edge configuration to look up.",
			},
			"type": schema.Int64Attribute{
				Computed:    true,
				Description: "Edge configuration type identifier as returned by the Portainer API.",
			},
			"category": schema.StringAttribute{
				Computed:    true,
				Description: "Category of the edge configuration (e.g. `configuration` or `secret`).",
			},
		},
	}
}

func (d *edgeConfigurationDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m edgeConfigurationDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := m.Name.ValueString()

	var items []struct {
		ID       int    `json:"id"`
		Name     string `json:"name"`
		Type     int    `json:"type"`
		Category string `json:"category"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/edge_configurations", nil, &items); err != nil {
		resp.Diagnostics.AddError("Failed to list edge configurations", fmt.Sprintf("failed to list edge configurations: %s", err))
		return
	}
	for _, item := range items {
		if item.Name == name {
			m.ID = types.StringValue(strconv.Itoa(item.ID))
			m.Type = types.Int64Value(int64(item.Type))
			m.Category = types.StringValue(item.Category)
			resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
			return
		}
	}
	resp.Diagnostics.AddError("Edge configuration not found", fmt.Sprintf("edge configuration %s not found", name))
}
