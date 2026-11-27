package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// addonCatalogDataSource reads the addon catalog document itself, as the
// configured catalog URL publishes it: catalog content only, without the
// install state and records portainer_addons folds in.
var (
	_ datasource.DataSource              = (*addonCatalogDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*addonCatalogDataSource)(nil)
)

func newAddonCatalogDataSource() datasource.DataSource { return &addonCatalogDataSource{} }

type addonCatalogDataSource struct{ client *APIClient }

type addonCatalogModel struct {
	ID     types.String       `tfsdk:"id"`
	Addons []addonCatalogItem `tfsdk:"addons"`
}

type addonCatalogItem struct {
	ID                  string   `tfsdk:"id" json:"id"`
	DisplayName         string   `tfsdk:"display_name" json:"displayName"`
	Description         string   `tfsdk:"description" json:"description"`
	ShortDescription    string   `tfsdk:"short_description" json:"shortDescription"`
	Icon                string   `tfsdk:"icon" json:"icon"`
	Path                string   `tfsdk:"path" json:"path"`
	ServiceURL          string   `tfsdk:"service_url" json:"serviceUrl"`
	HealthCheckPath     string   `tfsdk:"health_check_path" json:"healthCheckPath"`
	HelmChartRepository string   `tfsdk:"helm_chart_repository" json:"helmChartRepository"`
	TLSVerify           bool     `tfsdk:"tls_verify" json:"tlsVerify"`
	MachineAccess       []string `tfsdk:"machine_access" json:"machineAccess"`
}

func (d *addonCatalogDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_addon_catalog"
}

func (d *addonCatalogDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"addons": schema.ListAttribute{
				Computed:    true,
				Description: "Addons the catalog publishes.",
				ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{
					"id": types.StringType, "display_name": types.StringType, "description": types.StringType,
					"short_description": types.StringType, "icon": types.StringType, "path": types.StringType,
					"service_url": types.StringType, "health_check_path": types.StringType, "helm_chart_repository": types.StringType,
					"tls_verify": types.BoolType, "machine_access": types.ListType{ElemType: types.StringType},
				}},
			},
		},
	}
}

func (d *addonCatalogDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *addonCatalogDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	// When the catalog source is unreachable Portainer serves its last good
	// copy; with no copy at all it answers 503, which fails the read rather
	// than reporting an empty catalog as current.
	var catalog struct {
		Addons []addonCatalogItem `json:"addons"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/addons/catalog", nil, &catalog); err != nil {
		err = explainUnsupported(ctx, d.client, err, "portainer_addon_catalog", requiresBE2452)
		addonDiagError(&resp.Diagnostics, fmt.Errorf("failed to read the addon catalog: %w", err))
		return
	}

	addons := make([]addonCatalogItem, 0, len(catalog.Addons))
	for _, a := range catalog.Addons {
		if a.MachineAccess == nil {
			a.MachineAccess = []string{}
		}
		addons = append(addons, a)
	}

	m := addonCatalogModel{ID: types.StringValue("portainer-addon-catalog"), Addons: addons}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
