package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSource              = (*addonsDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*addonsDataSource)(nil)
)

func newAddonsDataSource() datasource.DataSource { return &addonsDataSource{} }

type addonsDataSource struct{ client *APIClient }

type addonsDataSourceModel struct {
	ID            types.String `tfsdk:"id"`
	View          types.String `tfsdk:"view"`
	EnvironmentID types.Int64  `tfsdk:"environment_id"`
	CatalogError  types.String `tfsdk:"catalog_error"`
	Addons        []addonsItem `tfsdk:"addons"`
}

type addonsItem struct {
	ID                     string   `tfsdk:"id"`
	DisplayName            string   `tfsdk:"display_name"`
	Description            string   `tfsdk:"description"`
	ShortDescription       string   `tfsdk:"short_description"`
	Icon                   string   `tfsdk:"icon"`
	Path                   string   `tfsdk:"path"`
	Enabled                bool     `tfsdk:"enabled"`
	LifecycleStatus        string   `tfsdk:"lifecycle_status"`
	LifecycleStatusMessage string   `tfsdk:"lifecycle_status_message"`
	HealthStatus           string   `tfsdk:"health_status"`
	HealthMessage          string   `tfsdk:"health_message"`
	ChartVersion           string   `tfsdk:"chart_version"`
	UpgradeAvailable       bool     `tfsdk:"upgrade_available"`
	AvailableVersions      []string `tfsdk:"available_versions"`
}

func (d *addonsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_addons"
}

func (d *addonsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"view": schema.StringAttribute{
				Optional:    true,
				Validators:  []validator.String{stringvalidator.OneOf("switcher")},
				Description: "Response view. Set to `switcher` for the lightweight app-switcher listing, which omits the catalog details and is readable by non-admin users. Leave unset for the full listing.",
			},
			"environment_id": schema.Int64Attribute{
				Computed:    true,
				Description: "Identifier of the environment addon resources are read through, as reported by Portainer.",
			},
			"catalog_error": schema.StringAttribute{Computed: true, Description: "Why the addon catalog could not be refreshed. When addons are still listed the listing is a cached copy; when it is empty nothing could be loaded. Admin-only, and never set on the `switcher` view."},
			"addons": schema.ListAttribute{
				Computed:    true,
				Description: "Addons the connected Portainer offers.",
				ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{
					"id": types.StringType, "display_name": types.StringType, "description": types.StringType,
					"short_description": types.StringType, "icon": types.StringType, "path": types.StringType,
					"enabled": types.BoolType, "lifecycle_status": types.StringType, "lifecycle_status_message": types.StringType,
					"health_status": types.StringType, "health_message": types.StringType, "chart_version": types.StringType,
					"upgrade_available": types.BoolType, "available_versions": types.ListType{ElemType: types.StringType},
				}},
			},
		},
	}
}

func (d *addonsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *addonsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m addonsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	endpoint := d.client.Endpoint + "/addons"
	if view := m.View.ValueString(); view != "" {
		endpoint += "?view=" + view
	}

	var listing struct {
		Addons       []addonListItem `json:"addons"`
		CatalogError string          `json:"catalogError"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, endpoint, nil, &listing); err != nil {
		addonDiagError(&resp.Diagnostics, fmt.Errorf("failed to list the addons: %w", explainAddonsUnavailable(ctx, d.client, err)))
		return
	}

	addons := make([]addonsItem, 0, len(listing.Addons))
	for _, addon := range listing.Addons {
		item := addonsItem{
			ID:                     addon.ID,
			DisplayName:            addon.DisplayName,
			Description:            addon.Description,
			ShortDescription:       addon.ShortDescription,
			Icon:                   addon.Icon,
			Path:                   addon.Path,
			Enabled:                addon.Enabled,
			LifecycleStatus:        addon.LifecycleStatus,
			LifecycleStatusMessage: addon.LifecycleStatusMessage,
			HealthStatus:           addon.HealthStatus,
			HealthMessage:          addon.HealthMessage,
			AvailableVersions:      []string{},
		}
		// The record is admin-only, so the fields drawn from it stay at their
		// zero values for anyone else rather than failing the read.
		if rec := addon.Record; rec != nil {
			item.ChartVersion = rec.ChartVersion
			item.UpgradeAvailable = rec.UpgradeAvailable
			if rec.AvailableVersions != nil {
				item.AvailableVersions = rec.AvailableVersions
			}
		}
		addons = append(addons, item)
	}

	var environment struct {
		EnvironmentID int `json:"environmentId"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/addons/environment", nil, &environment); err != nil {
		addonDiagError(&resp.Diagnostics, fmt.Errorf("failed to read the environment addon resources are served from: %w", explainAddonsUnavailable(ctx, d.client, err)))
		return
	}

	m.Addons = addons
	m.CatalogError = types.StringValue(listing.CatalogError)
	m.EnvironmentID = types.Int64Value(int64(environment.EnvironmentID))
	m.ID = types.StringValue("portainer-addons")
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
