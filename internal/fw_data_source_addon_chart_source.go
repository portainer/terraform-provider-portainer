package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// addonChartSourceDataSource checks where an addon's chart would be pulled
// from, which is the way to find out whether a registry is usable and what
// versions it offers before an install is attempted.
var (
	_ datasource.DataSource              = (*addonChartSourceDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*addonChartSourceDataSource)(nil)
)

func newAddonChartSourceDataSource() datasource.DataSource { return &addonChartSourceDataSource{} }

type addonChartSourceDataSource struct{ client *APIClient }

type addonChartSourceModel struct {
	ID         types.String `tfsdk:"id"`
	AddonID    types.String `tfsdk:"addon_id"`
	RegistryID types.Int64  `tfsdk:"registry_id"`
	Chart      types.String `tfsdk:"chart"`
	Reachable  types.Bool   `tfsdk:"reachable"`
	Reason     types.String `tfsdk:"reason"`
	TLSVerify  types.Bool   `tfsdk:"tls_verify"`
	Versions   types.List   `tfsdk:"versions"`
}

func (d *addonChartSourceDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_addon_chart_source"
}

func (d *addonChartSourceDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"addon_id": schema.StringAttribute{
				Required:    true,
				Description: "Catalog identifier of the addon whose chart source is checked, for example `portal-template`.",
			},
			"registry_id": schema.Int64Attribute{
				Optional:    true,
				Description: "Identifier of the Portainer registry to check against. Leave unset to check the catalog's public chart reference.",
			},
			"chart": schema.StringAttribute{
				Optional:    true,
				Description: "Chart path within `registry_id` to check, for example `portainer/charts/portainer-run`. Leave unset when checking the public reference.",
			},
			"reachable": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the registry answered. A registry that answered but holds no such chart is still reachable, with an empty `versions`.",
			},
			"reason": schema.StringAttribute{
				Computed:    true,
				Description: "Why the source is unreachable, empty when `reachable` is true.",
			},
			"tls_verify": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the certificate the registry served verified against the trust store.",
			},
			"versions": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Published stable versions of the chart, newest first.",
			},
		},
	}
}

func (d *addonChartSourceDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *addonChartSourceDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m addonChartSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	addonID := m.AddonID.ValueString()

	query := url.Values{}
	if v := m.RegistryID.ValueInt64(); v != 0 {
		query.Set("registryId", strconv.FormatInt(v, 10))
	}
	if v := m.Chart.ValueString(); v != "" {
		query.Set("chart", v)
	}

	endpoint := addonURL(d.client, addonID) + "/chart-source"
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var status struct {
		Reachable bool     `json:"reachable"`
		Reason    string   `json:"reason"`
		TLSVerify bool     `json:"tlsVerify"`
		Versions  []string `json:"versions"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, endpoint, nil, &status); err != nil {
		addonDiagError(&resp.Diagnostics, fmt.Errorf("failed to check the chart source of addon %s: %w", addonID, err))
		return
	}

	m.Reachable = types.BoolValue(status.Reachable)
	m.Reason = types.StringValue(status.Reason)
	m.TLSVerify = types.BoolValue(status.TLSVerify)
	m.Versions = addonStringList(status.Versions)
	m.ID = types.StringValue(fmt.Sprintf("portainer-addon-chart-source-%s", addonID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
