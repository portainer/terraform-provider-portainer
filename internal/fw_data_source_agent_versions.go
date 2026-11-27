package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*agentVersionsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*agentVersionsDataSource)(nil)
)

// agentVersionsDataSource lists the agent versions in use across the
// environments the token can see. It is a different question from
// portainer_edge_update_schedule_info, which reports the versions an update
// schedule can move environments to.
func newAgentVersionsDataSource() datasource.DataSource { return &agentVersionsDataSource{} }

type agentVersionsDataSource struct{ client *APIClient }

type agentVersionsDataSourceModel struct {
	ID       types.String `tfsdk:"id"`
	Versions types.List   `tfsdk:"versions"`
}

func (d *agentVersionsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_agent_versions"
}

func (d *agentVersionsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:    true,
				Description: "Identifier of the data source, always `portainer-agent-versions`.",
			},
			"versions": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Agent versions in use across the environments the token can see, sorted. More than one entry means the fleet is not on a single version.",
			},
		},
	}
}

func (d *agentVersionsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *agentVersionsDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	var versions []string
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/endpoints/agent_versions", nil, &versions); err != nil {
		resp.Diagnostics.AddError("Failed to list the agent versions", fmt.Errorf("failed to list the agent versions: %w", err).Error())
		return
	}

	sorted := sortedStrings(versions)
	if sorted == nil {
		sorted = []string{}
	}
	list, diags := types.ListValueFrom(ctx, types.StringType, sorted)
	resp.Diagnostics.Append(diags...)
	if resp.Diagnostics.HasError() {
		return
	}
	state := agentVersionsDataSourceModel{ID: types.StringValue("portainer-agent-versions"), Versions: list}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}
