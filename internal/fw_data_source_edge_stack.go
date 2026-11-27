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

var _ datasource.DataSourceWithConfigure = (*edgeStackDataSource)(nil)

func newEdgeStackDataSource() datasource.DataSource { return &edgeStackDataSource{} }

// edgeStackDataSource looks up an edge stack by name.
type edgeStackDataSource struct{ edgeDataSourceBase }

type edgeStackDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	DeploymentType types.Int64  `tfsdk:"deployment_type"`
}

func (d *edgeStackDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_stack"
}

func (d *edgeStackDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Identifier of the edge stack."},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the edge stack to look up in Portainer.",
			},
			"deployment_type": schema.Int64Attribute{
				Computed:    true,
				Description: "Deployment type of the edge stack (0 = Compose, 1 = Kubernetes, 2 = Nomad).",
			},
		},
	}
}

func (d *edgeStackDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m edgeStackDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := m.Name.ValueString()

	var items []struct {
		ID             int    `json:"Id"`
		Name           string `json:"Name"`
		DeploymentType int    `json:"DeploymentType"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/edge_stacks", nil, &items); err != nil {
		resp.Diagnostics.AddError("Failed to list edge stacks", fmt.Sprintf("failed to list edge stacks: %s", err))
		return
	}
	for _, item := range items {
		if item.Name == name {
			m.ID = types.StringValue(strconv.Itoa(item.ID))
			m.DeploymentType = types.Int64Value(int64(item.DeploymentType))
			resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
			return
		}
	}
	resp.Diagnostics.AddError("Edge stack not found", fmt.Sprintf("edge stack %s not found", name))
}
