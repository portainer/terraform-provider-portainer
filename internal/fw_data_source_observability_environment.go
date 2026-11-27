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
	_ datasource.DataSource              = (*observabilityEnvironmentDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*observabilityEnvironmentDataSource)(nil)
)

// observabilityEnvironmentDataSource reports whether observability is on for
// an environment and which provider serves it - the address of the OneUptime
// instance an Observability policy deployed or connected to, for instance,
// which a configuration can hand to dashboards or links.
func newObservabilityEnvironmentDataSource() datasource.DataSource {
	return &observabilityEnvironmentDataSource{}
}

type observabilityEnvironmentDataSource struct{ client *APIClient }

type observabilityEnvironmentModel struct {
	ID            types.String `tfsdk:"id"`
	EnvironmentID types.Int64  `tfsdk:"environment_id"`
	Enabled       types.Bool   `tfsdk:"enabled"`
	ProviderName  types.String `tfsdk:"provider_name"`
	URL           types.String `tfsdk:"url"`
	ProjectID     types.String `tfsdk:"project_id"`
	ClusterID     types.String `tfsdk:"cluster_id"`
	ClusterName   types.String `tfsdk:"cluster_name"`
}

func (d *observabilityEnvironmentDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_observability_environment"
}

func (d *observabilityEnvironmentDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"environment_id": schema.Int64Attribute{
				Required:    true,
				Description: "ID of the environment to report on.",
			},
			"enabled":       schema.BoolAttribute{Computed: true, Description: "Whether observability is enabled for the environment."},
			"provider_name": schema.StringAttribute{Computed: true, Description: "Observability provider serving the environment, empty when none does."},
			"url":           schema.StringAttribute{Computed: true, Description: "URL of the observability provider."},
			"project_id":    schema.StringAttribute{Computed: true, Description: "Project of the environment at the provider."},
			"cluster_id":    schema.StringAttribute{Computed: true, Description: "Provider's internal identifier of the cluster (OneUptime's cluster ID, used for deep links). Empty when Portainer could not resolve it."},
			"cluster_name":  schema.StringAttribute{Computed: true, Description: "Name of the cluster at the provider."},
		},
	}
}

func (d *observabilityEnvironmentDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *observabilityEnvironmentDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data observabilityEnvironmentModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)
	if resp.Diagnostics.HasError() {
		return
	}
	envID := data.EnvironmentID.ValueInt64()

	var out struct {
		Enabled     bool   `json:"enabled"`
		Provider    string `json:"provider"`
		URL         string `json:"url"`
		ProjectID   string `json:"projectId"`
		ClusterID   string `json:"clusterId"`
		ClusterName string `json:"clusterName"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, fmt.Sprintf("%s/observability/environments/%d", d.client.Endpoint, envID), nil, &out); err != nil {
		// A 404 is also what a missing environment gets; explainUnsupported
		// only rewrites it when the server is too old or not BE.
		err = explainUnsupported(ctx, d.client, err, "portainer_observability_environment", requiresBE245)
		resp.Diagnostics.AddError("Failed to read environment observability", fmt.Sprintf("failed to read the observability of environment %d: %s", envID, err))
		return
	}

	data.Enabled = types.BoolValue(out.Enabled)
	data.ProviderName = types.StringValue(out.Provider)
	data.URL = types.StringValue(out.URL)
	data.ProjectID = types.StringValue(out.ProjectID)
	data.ClusterID = types.StringValue(out.ClusterID)
	data.ClusterName = types.StringValue(out.ClusterName)
	data.ID = types.StringValue(fmt.Sprintf("portainer-observability-environment-%d", envID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
