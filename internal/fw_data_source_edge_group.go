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

var _ datasource.DataSourceWithConfigure = (*edgeGroupDataSource)(nil)

func newEdgeGroupDataSource() datasource.DataSource { return &edgeGroupDataSource{} }

// edgeGroupDataSource looks up an edge group by name.
type edgeGroupDataSource struct{ edgeDataSourceBase }

type edgeGroupDataSourceModel struct {
	ID      types.String `tfsdk:"id"`
	Name    types.String `tfsdk:"name"`
	Dynamic types.Bool   `tfsdk:"dynamic"`
}

func (d *edgeGroupDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_group"
}

func (d *edgeGroupDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Identifier of the edge group."},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the edge group to look up in Portainer.",
			},
			"dynamic": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the edge group is dynamic (membership computed from tags) or static (explicit endpoint list).",
			},
		},
	}
}

func (d *edgeGroupDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m edgeGroupDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := m.Name.ValueString()

	var items []struct {
		ID      int    `json:"Id"`
		Name    string `json:"Name"`
		Dynamic bool   `json:"Dynamic"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/edge_groups", nil, &items); err != nil {
		resp.Diagnostics.AddError("Failed to list edge groups", fmt.Sprintf("failed to list edge groups: %s", err))
		return
	}
	for _, item := range items {
		if item.Name == name {
			m.ID = types.StringValue(strconv.Itoa(item.ID))
			m.Dynamic = types.BoolValue(item.Dynamic)
			resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
			return
		}
	}
	resp.Diagnostics.AddError("Edge group not found", fmt.Sprintf("edge group %s not found", name))
}
