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

var _ datasource.DataSourceWithConfigure = (*edgeJobDataSource)(nil)

func newEdgeJobDataSource() datasource.DataSource { return &edgeJobDataSource{} }

// edgeJobDataSource looks up an edge job by name.
type edgeJobDataSource struct{ edgeDataSourceBase }

type edgeJobDataSourceModel struct {
	ID             types.String `tfsdk:"id"`
	Name           types.String `tfsdk:"name"`
	CronExpression types.String `tfsdk:"cron_expression"`
}

func (d *edgeJobDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_job"
}

func (d *edgeJobDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Computed: true, Description: "Identifier of the edge job."},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the edge job to look up in Portainer.",
			},
			"cron_expression": schema.StringAttribute{
				Computed:    true,
				Description: "Cron expression that schedules execution of the edge job.",
			},
		},
	}
}

func (d *edgeJobDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m edgeJobDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	name := m.Name.ValueString()

	var items []struct {
		ID             int    `json:"Id"`
		Name           string `json:"Name"`
		CronExpression string `json:"CronExpression"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/edge_jobs", nil, &items); err != nil {
		resp.Diagnostics.AddError("Failed to list edge jobs", fmt.Sprintf("failed to list edge jobs: %s", err))
		return
	}
	for _, item := range items {
		if item.Name == name {
			m.ID = types.StringValue(strconv.Itoa(item.ID))
			m.CronExpression = types.StringValue(item.CronExpression)
			resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
			return
		}
	}
	resp.Diagnostics.AddError("Edge job not found", fmt.Sprintf("edge job %s not found", name))
}
