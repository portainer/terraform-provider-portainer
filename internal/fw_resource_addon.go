package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*addonResource)(nil)
	_ resource.ResourceWithConfigure   = (*addonResource)(nil)
	_ resource.ResourceWithImportState = (*addonResource)(nil)
)

func newAddonResource() resource.Resource { return &addonResource{} }

type addonResource struct{ client *APIClient }

type addonResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	AddonID                types.String `tfsdk:"addon_id"`
	Chart                  types.String `tfsdk:"chart"`
	Version                types.String `tfsdk:"version"`
	RegistryID             types.Int64  `tfsdk:"registry_id"`
	ImageRegistryID        types.Int64  `tfsdk:"image_registry_id"`
	Values                 types.String `tfsdk:"values"`
	ForceUninstall         types.Bool   `tfsdk:"force_uninstall"`
	PruneConfigOnUninstall types.Bool   `tfsdk:"prune_config_on_uninstall"`
	DisplayName            types.String `tfsdk:"display_name"`
	Description            types.String `tfsdk:"description"`
	ShortDescription       types.String `tfsdk:"short_description"`
	Icon                   types.String `tfsdk:"icon"`
	Path                   types.String `tfsdk:"path"`
	Enabled                types.Bool   `tfsdk:"enabled"`
	LifecycleStatus        types.String `tfsdk:"lifecycle_status"`
	LifecycleStatusMessage types.String `tfsdk:"lifecycle_status_message"`
	HealthStatus           types.String `tfsdk:"health_status"`
	HealthMessage          types.String `tfsdk:"health_message"`
	ChartVersion           types.String `tfsdk:"chart_version"`
	ChartPath              types.String `tfsdk:"chart_path"`
	HelmChartRepository    types.String `tfsdk:"helm_chart_repository"`
	AvailableVersions      types.List   `tfsdk:"available_versions"`
	UpgradeAvailable       types.Bool   `tfsdk:"upgrade_available"`
	VersionCheckError      types.String `tfsdk:"version_check_error"`
	PublicURL              types.String `tfsdk:"public_url"`
}

func (r *addonResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_addon"
}

func (r *addonResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	computedString := func(desc string) schema.StringAttribute {
		return schema.StringAttribute{Computed: true, Description: desc}
	}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"addon_id": schema.StringAttribute{
				Required:      true,
				Description:   "Catalog identifier of the addon, for example `portal-template`. Use the `portainer_addons` data source to list what the connected Portainer offers.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"chart": schema.StringAttribute{
				Optional:    true,
				Description: "Chart path within `registry_id`, for example `portainer/charts/portainer-run`. Required when `registry_id` is set; leave unset to install from the catalog's public chart reference.",
			},
			"version": schema.StringAttribute{
				Optional:    true,
				Description: "Chart version to install. Leave unset to resolve the latest published version at apply time - note that Portainer then picks the version, so the installed one is reported in `chart_version` rather than here.",
			},
			// Portainer only honours registryId on a first-time install: an
			// installed addon keeps the chart source it was installed from.
			// Changing it therefore has to reinstall to mean anything.
			"registry_id": schema.Int64Attribute{
				Optional:      true,
				Description:   "Identifier of the Portainer registry the chart is pulled from. Only honoured on a first install, so changing it reinstalls the addon. Leave unset to use the catalog's public chart reference.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"image_registry_id": schema.Int64Attribute{
				Optional:    true,
				Description: "Identifier of the Portainer registry whose credentials the addon's pods pull their container image with. Unlike `registry_id` this can be changed on an upgrade. Leave unset to keep whatever the addon is installed with.",
			},
			"values": schema.StringAttribute{
				Optional:    true,
				Description: "Helm value overrides as a JSON object, merged over Portainer's per-addon defaults. Use `jsonencode({...})` to build it.",
				Validators:  []validator.String{addonJSONObjectValidator{}},
			},
			"force_uninstall": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether to force the uninstall when the addon is destroyed, which lets Portainer remove a release that is in a broken state.",
			},
			"prune_config_on_uninstall": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether to also delete the addon's stored configuration entries when the addon is destroyed. By default the configuration survives an uninstall so a reinstall keeps it.",
			},
			"display_name":             computedString("Human-readable name of the addon as published in the catalog."),
			"description":              computedString("Full catalog description of the addon."),
			"short_description":        computedString("One-line catalog tagline for the addon."),
			"icon":                     computedString("Icon the Portainer UI shows for the addon."),
			"path":                     computedString("Path the addon is served under in Portainer."),
			"enabled":                  schema.BoolAttribute{Computed: true, Description: "Whether the addon is installed and switched on. Manage it with `portainer_addon_access`."},
			"lifecycle_status":         computedString("Helm release lifecycle state: `installing`, `upgrading`, `installed`, `failed`, `unknown` or `uninstalling`. This is not a health status."),
			"lifecycle_status_message": computedString("Explanation of a failed install or upgrade, empty after a clean one."),
			"health_status":            computedString("Live probe of the running addon: `healthy`, `unhealthy`, `credential-invalid` or `unknown`. Orthogonal to `lifecycle_status` - an installed addon can be unhealthy."),
			"health_message":           computedString("Explanation of an unhealthy probe result, empty otherwise."),
			"chart_version":            computedString("Chart version actually installed, which is what to read when `version` is left unset."),
			"chart_path":               computedString("Chart path recorded for the installed release, empty when the chart came from the public reference."),
			"helm_chart_repository":    computedString("Public OCI chart reference the catalog publishes for the addon."),
			"available_versions": schema.ListAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "Published stable chart versions, newest first. Empty when the lookup could not run, in which case `version_check_error` says why.",
			},
			"public_url": schema.StringAttribute{
				Optional:    true,
				Description: "URL users open the addon at, for example `https://run.example.com`. Portainer signs users in to the addon only by sending them back to this URL. Requires Portainer Business Edition 3.1 or later; leave unset to keep whatever is set in Portainer. Removing it from the configuration clears the URL.",
			},
			"upgrade_available":   schema.BoolAttribute{Computed: true, Description: "Whether a newer chart version than the installed one has been published."},
			"version_check_error": computedString("Why the available-version lookup failed, empty when it succeeded."),
		},
	}
}

func (r *addonResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// POST /addons/{id} both installs and upgrades, so create and update run the
// same call.
func (r *addonResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.install(ctx, req.Plan, nil, &resp.State, resp.Private, &resp.Diagnostics)
}

func (r *addonResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var prior addonResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &prior)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.install(ctx, req.Plan, &prior, &resp.State, resp.Private, &resp.Diagnostics)
}

// addonInstallInputsEqual reports whether the arguments POST /addons/{id}
// takes are the same in both models, in which case an update has nothing to
// install and only the public URL (or a destroy flag) changed.
func addonInstallInputsEqual(a, b addonResourceModel) bool {
	return a.Chart.Equal(b.Chart) && a.Version.Equal(b.Version) && a.RegistryID.Equal(b.RegistryID) &&
		a.ImageRegistryID.Equal(b.ImageRegistryID) && a.Values.Equal(b.Values)
}

// setAddonPublicURL sets (or, with "", clears) the URL users open the addon
// at. The endpoint is new in Business Edition 3.1; the addon itself exists at
// this point, so a 404 means the server lacks the endpoint.
func setAddonPublicURL(ctx context.Context, client *APIClient, addonID, publicURL string) error {
	err := doJSON(ctx, client, http.MethodPut, addonURL(client, addonID)+"/public-url", map[string]interface{}{"publicUrl": publicURL}, nil)
	if err != nil {
		err = explainUnsupported(ctx, client, err, "public_url of portainer_addon", requiresBE31Addons)
		return fmt.Errorf("failed to set the public URL of addon %s: %w", addonID, err)
	}
	return nil
}

// install runs POST /addons/{id} (install or upgrade) and then sets the public
// URL. prior is the state of an update, nil on create; an update that changes
// none of the install arguments skips the reinstall.
func (r *addonResource) install(ctx context.Context, plan tfsdk.Plan, prior *addonResourceModel, state *tfsdk.State, private addonPrivateState, diags *diag.Diagnostics) {
	var m addonResourceModel
	diags.Append(plan.Get(ctx, &m)...)
	if diags.HasError() {
		return
	}
	addonID := m.AddonID.ValueString()

	if prior == nil || !addonInstallInputsEqual(m, *prior) {
		if !r.postInstall(ctx, m, diags) {
			return
		}
	}
	m.ID = types.StringValue(addonID)

	// public_url is only sent when configured, so a server without the
	// endpoint is never asked; removing it from the configuration clears it.
	switch {
	case !m.PublicURL.IsNull():
		if err := setAddonPublicURL(ctx, r.client, addonID, m.PublicURL.ValueString()); err != nil {
			addonDiagError(diags, err)
			return
		}
	case prior != nil && prior.PublicURL.ValueString() != "":
		if err := setAddonPublicURL(ctx, r.client, addonID, ""); err != nil {
			addonDiagError(diags, err)
			return
		}
	}

	publicURL := m.PublicURL
	addon, err := readAddon(ctx, r.client, addonID)
	if err != nil {
		addonDiagError(diags, fmt.Errorf("failed to read addon %s: %w", addonID, err))
		return
	}
	// The configured arguments are what was just applied; the read only
	// fills in what Portainer reports.
	explicit := addonExplicitIDs{RegistryID: !m.RegistryID.IsNull(), ImageRegistryID: !m.ImageRegistryID.IsNull()}
	diags.Append(explicit.save(ctx, private)...)
	registryID, imageRegistryID := m.RegistryID, m.ImageRegistryID
	if addon != nil {
		addonApplyRead(&m, addon, explicit)
	}
	m.RegistryID, m.ImageRegistryID, m.PublicURL = registryID, imageRegistryID, publicURL
	addonFillUnknown(&m)
	diags.Append(state.Set(ctx, &m)...)
}

// postInstall sends POST /addons/{id}, which installs or upgrades the addon.
func (r *addonResource) postInstall(ctx context.Context, m addonResourceModel, diags *diag.Diagnostics) bool {
	addonID := m.AddonID.ValueString()

	payload := map[string]interface{}{}
	if v := m.Chart.ValueString(); v != "" {
		payload["chart"] = v
	}
	if v := m.Version.ValueString(); v != "" {
		payload["version"] = v
	}
	if v := m.RegistryID.ValueInt64(); v != 0 {
		payload["registryId"] = int(v)
	}
	// imageRegistryId is always sent, including the 0 Portainer reads as "the
	// image needs no credentials". Omitting it would instead mean "keep whatever
	// the addon is installed with", which is not what an absent attribute means
	// in a Terraform configuration.
	payload["imageRegistryId"] = int(m.ImageRegistryID.ValueInt64())
	if raw := m.Values.ValueString(); raw != "" {
		var values map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &values); err != nil {
			addonDiagError(diags, fmt.Errorf("failed to parse the addon's values as a JSON object: %w", err))
			return false
		}
		payload["values"] = values
	}

	if err := doJSON(ctx, r.client, http.MethodPost, addonURL(r.client, addonID), payload, nil); err != nil {
		addonDiagError(diags, fmt.Errorf("failed to install or upgrade addon %s: %w", addonID, err))
		return false
	}
	return true
}

// addonPrivateState is the private state of a resource response.
type addonPrivateState interface {
	SetKey(ctx context.Context, key string, value []byte) diag.Diagnostics
}

// addonExplicitIDs records whether the configuration set registry_id and
// image_registry_id. SDKv2 stored 0 for either when it was left unset, and
// Portainer reports 0 for "none", so a 0 read back is only kept as 0 when the
// configuration said 0; otherwise it is null, which is what an unset
// attribute plans. State written by v3.0.0 carries no record, and its 0
// therefore reads back as null too.
type addonExplicitIDs struct {
	RegistryID      bool `json:"registry_id"`
	ImageRegistryID bool `json:"image_registry_id"`
}

const addonExplicitIDsKey = "addon_explicit_ids"

func (e addonExplicitIDs) save(ctx context.Context, private addonPrivateState) diag.Diagnostics {
	raw, _ := json.Marshal(e)
	return private.SetKey(ctx, addonExplicitIDsKey, raw)
}

// addonPrivateReader is the private state of a resource request.
type addonPrivateReader interface {
	GetKey(ctx context.Context, key string) ([]byte, diag.Diagnostics)
}

func loadAddonExplicitIDs(ctx context.Context, private addonPrivateReader) (addonExplicitIDs, diag.Diagnostics) {
	var e addonExplicitIDs
	raw, diags := private.GetKey(ctx, addonExplicitIDsKey)
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &e)
	}
	return e, diags
}

func addonZeroAsNull(apiValue int, explicit bool) types.Int64 {
	if apiValue == 0 && !explicit {
		return types.Int64Null()
	}
	return types.Int64Value(int64(apiValue))
}

// addonApplyRead folds an inspect response into the model. The fields drawn
// from the admin-only record are left as they were when it is absent.
func addonApplyRead(m *addonResourceModel, addon *addonListItem, explicit addonExplicitIDs) {
	m.AddonID = types.StringValue(addon.ID)
	m.DisplayName = types.StringValue(addon.DisplayName)
	m.Description = types.StringValue(addon.Description)
	m.ShortDescription = types.StringValue(addon.ShortDescription)
	m.Icon = types.StringValue(addon.Icon)
	m.Path = types.StringValue(addon.Path)
	m.Enabled = types.BoolValue(addon.Enabled)
	m.LifecycleStatus = types.StringValue(addon.LifecycleStatus)
	m.LifecycleStatusMessage = types.StringValue(addon.LifecycleStatusMessage)
	m.HealthStatus = types.StringValue(addon.HealthStatus)
	m.HealthMessage = types.StringValue(addon.HealthMessage)
	if rec := addon.Record; rec != nil {
		m.ChartVersion = types.StringValue(rec.ChartVersion)
		m.ChartPath = types.StringValue(rec.ChartPath)
		m.HelmChartRepository = types.StringValue(rec.HelmChartRepository)
		m.AvailableVersions = addonStringList(rec.AvailableVersions)
		m.UpgradeAvailable = types.BoolValue(rec.UpgradeAvailable)
		m.VersionCheckError = types.StringValue(rec.VersionCheckError)
		m.ImageRegistryID = addonZeroAsNull(rec.ImageRegistryID, explicit.ImageRegistryID)
		// registry_id is fixed at install time, so reading it back keeps an
		// imported resource from planning a reinstall against its own state.
		m.RegistryID = addonZeroAsNull(rec.RegistryID, explicit.RegistryID)
		// public_url is read back only while it is configured: unset, it
		// leaves the URL to whoever manages it in Portainer.
		if !m.PublicURL.IsNull() {
			m.PublicURL = types.StringValue(rec.PublicURL)
		}
	}
}

// addonFillUnknown settles the computed attributes a response without the
// admin-only record left unknown after an apply.
func addonFillUnknown(m *addonResourceModel) {
	for _, s := range []*types.String{
		&m.ChartVersion, &m.ChartPath, &m.HelmChartRepository, &m.VersionCheckError,
		&m.DisplayName, &m.Description, &m.ShortDescription, &m.Icon, &m.Path,
		&m.LifecycleStatus, &m.LifecycleStatusMessage, &m.HealthStatus, &m.HealthMessage,
	} {
		if s.IsUnknown() {
			*s = types.StringValue("")
		}
	}
	for _, b := range []*types.Bool{&m.UpgradeAvailable, &m.Enabled} {
		if b.IsUnknown() {
			*b = types.BoolValue(false)
		}
	}
	if m.AvailableVersions.IsUnknown() {
		m.AvailableVersions = addonStringList(nil)
	}
}

func (r *addonResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m addonResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := m.ID.ValueString()

	addon, err := readAddon(ctx, r.client, id)
	if err != nil {
		addonDiagError(&resp.Diagnostics, fmt.Errorf("failed to read addon %s: %w", id, err))
		return
	}
	// An addon stays in the catalog after it is uninstalled; what disappears is
	// its release. An empty lifecycle status is Portainer's way of saying the
	// addon was never installed, so that is what removes the resource from
	// state rather than a 404.
	if addon == nil || addon.LifecycleStatus == "" {
		resp.State.RemoveResource(ctx)
		return
	}

	explicit, d := loadAddonExplicitIDs(ctx, req.Private)
	resp.Diagnostics.Append(d...)
	addonApplyRead(&m, addon, explicit)
	addonFillUnknown(&m)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *addonResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var m addonResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id := m.ID.ValueString()

	endpoint := fmt.Sprintf("%s?force=%t&pruneConfig=%t",
		addonURL(r.client, id),
		m.ForceUninstall.ValueBool(),
		m.PruneConfigOnUninstall.ValueBool(),
	)
	if err := doJSON(ctx, r.client, http.MethodDelete, endpoint, nil, nil); err != nil && !isAPINotFound(err) {
		addonDiagError(&resp.Diagnostics, fmt.Errorf("failed to uninstall addon %s: %w", id, err))
	}
}

func (r *addonResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	// The destroy flags are never read back; start them at their defaults so
	// an imported addon plans no change for them.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("force_uninstall"), false)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("prune_config_on_uninstall"), false)...)
}
