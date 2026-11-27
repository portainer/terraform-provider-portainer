package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// addonCatalogRefreshResource drops Portainer's cached addon catalog, so the
// next read loads it from the configured source. It is a one-shot action, so
// it has no read or update: the resource records that a refresh ran, and
// `triggers` is how another one is asked for.
var (
	_ resource.Resource              = (*addonCatalogRefreshResource)(nil)
	_ resource.ResourceWithConfigure = (*addonCatalogRefreshResource)(nil)
)

func newAddonCatalogRefreshResource() resource.Resource { return &addonCatalogRefreshResource{} }

type addonCatalogRefreshResource struct{ client *APIClient }

type addonCatalogRefreshModel struct {
	ID       types.String `tfsdk:"id"`
	Triggers types.Map    `tfsdk:"triggers"`
}

func (r *addonCatalogRefreshResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_addon_catalog_refresh"
}

func (r *addonCatalogRefreshResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"triggers": schema.MapAttribute{
				Optional:      true,
				ElementType:   types.StringType,
				Description:   "Arbitrary values that force another refresh when they change, for example the catalog URL in the settings. A refresh is a one-shot action, so without a trigger the resource runs once and then stays put.",
				PlanModifiers: []planmodifier.Map{addonTriggersRequiresReplace{}},
			},
		},
	}
}

func (r *addonCatalogRefreshResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *addonCatalogRefreshResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m addonCatalogRefreshModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	addonCatalogRefreshRun(ctx, r.client, &m, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// addonCatalogRefreshRun refreshes the addon catalog; it is the operation of
// both the resource and the portainer_addon_catalog_refresh action.
func addonCatalogRefreshRun(ctx context.Context, client *APIClient, m *addonCatalogRefreshModel, diags *diag.Diagnostics) {
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/addons/catalog/refresh", nil, nil); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_addon_catalog_refresh", requiresBE2452)
		addonDiagError(diags, fmt.Errorf("failed to refresh the addon catalog: %w", err))
		return
	}

	m.ID = types.StringValue(fmt.Sprintf("addon-catalog-refresh-%d", makeTimestamp()))
}

// Read keeps the state: the refresh is recorded, not read back.
func (r *addonCatalogRefreshResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}

// Update only records a switch between an unset and an empty `triggers`,
// which SDKv2 did not tell apart; every real change replaces the resource.
func (r *addonCatalogRefreshResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m addonCatalogRefreshModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *addonCatalogRefreshResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
