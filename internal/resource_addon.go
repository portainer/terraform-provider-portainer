package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// addonListItem is the shape Portainer returns for a single addon, both from
// the list endpoint and from an inspect. Record is admin-only and simply
// absent for anyone else, so every field read out of it has to tolerate a
// zero value rather than assume it was populated.
type addonListItem struct {
	ID                     string `json:"id"`
	DisplayName            string `json:"displayName"`
	Description            string `json:"description"`
	ShortDescription       string `json:"shortDescription"`
	Icon                   string `json:"icon"`
	Path                   string `json:"path"`
	Enabled                bool   `json:"enabled"`
	HealthStatus           string `json:"healthStatus"`
	HealthMessage          string `json:"healthMessage"`
	LifecycleStatus        string `json:"lifecycleStatus"`
	LifecycleStatusMessage string `json:"lifecycleStatusMessage"`
	Record                 *struct {
		ChartVersion        string                    `json:"chartVersion"`
		ChartPath           string                    `json:"chartPath"`
		HelmChartRepository string                    `json:"helmChartRepository"`
		RegistryID          int                       `json:"registryId"`
		ImageRegistryID     int                       `json:"imageRegistryId"`
		AvailableVersions   []string                  `json:"availableVersions"`
		UpgradeAvailable    bool                      `json:"upgradeAvailable"`
		VersionCheckError   string                    `json:"versionCheckError"`
		UserAccessPolicies  map[string]map[string]int `json:"userAccessPolicies"`
		TeamAccessPolicies  map[string]map[string]int `json:"teamAccessPolicies"`
	} `json:"record"`
}

// addonURL builds the base URL for one addon. Addon identifiers are catalog
// slugs rather than numbers, so they are escaped rather than formatted in.
func addonURL(client *APIClient, addonID string) string {
	return fmt.Sprintf("%s/addons/%s", client.Endpoint, url.PathEscape(addonID))
}

// readAddon inspects a single addon. Callers use the nil result to mean the
// addon is not in the catalog at all.
func readAddon(ctx context.Context, client *APIClient, addonID string) (*addonListItem, error) {
	var addon addonListItem
	if err := doJSON(ctx, client, http.MethodGet, addonURL(client, addonID), nil, &addon); err != nil {
		if isAPINotFound(err) {
			return nil, nil
		}
		return nil, err
	}
	return &addon, nil
}

// validateJSONObject accepts only a JSON object, which is what Portainer's
// Helm value overrides have to be. A bare array or scalar parses as JSON but
// would be rejected by the server with a much less helpful message.
func validateJSONObject(v interface{}, key string) ([]string, []error) {
	raw, ok := v.(string)
	if !ok || raw == "" {
		return nil, nil
	}
	var decoded map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &decoded); err != nil {
		return nil, []error{fmt.Errorf("%s must be a JSON object (use jsonencode): %w", key, err)}
	}
	return nil, nil
}

func resourceAddon() *schema.Resource {
	return &schema.Resource{
		// POST /addons/{id} both installs and upgrades, so create and update
		// run the same call.
		CreateContext: resourceAddonInstall,
		ReadContext:   resourceAddonRead,
		UpdateContext: resourceAddonInstall,
		DeleteContext: resourceAddonUninstall,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"addon_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Catalog identifier of the addon, for example `portal-template`. Use the `portainer_addons` data source to list what the connected Portainer offers.",
			},
			"chart": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Chart path within `registry_id`, for example `portainer/charts/portainer-run`. Required when `registry_id` is set; leave unset to install from the catalog's public chart reference.",
			},
			"version": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Chart version to install. Leave unset to resolve the latest published version at apply time - note that Portainer then picks the version, so the installed one is reported in `chart_version` rather than here.",
			},
			"registry_id": {
				Type:     schema.TypeInt,
				Optional: true,
				// Portainer only honours registryId on a first-time install: an
				// installed addon keeps the chart source it was installed from.
				// Changing it therefore has to reinstall to mean anything.
				ForceNew:    true,
				Description: "Identifier of the Portainer registry the chart is pulled from. Only honoured on a first install, so changing it reinstalls the addon. Leave unset to use the catalog's public chart reference.",
			},
			"image_registry_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Identifier of the Portainer registry whose credentials the addon's pods pull their container image with. Unlike `registry_id` this can be changed on an upgrade. Leave unset to keep whatever the addon is installed with.",
			},
			"values": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validateJSONObject,
				Description:  "Helm value overrides as a JSON object, merged over Portainer's per-addon defaults. Use `jsonencode({...})` to build it.",
			},
			"force_uninstall": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to force the uninstall when the addon is destroyed, which lets Portainer remove a release that is in a broken state.",
			},
			"prune_config_on_uninstall": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to also delete the addon's stored configuration entries when the addon is destroyed. By default the configuration survives an uninstall so a reinstall keeps it.",
			},
			// Computed attributes
			"display_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Human-readable name of the addon as published in the catalog.",
			},
			"description": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Full catalog description of the addon.",
			},
			"short_description": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "One-line catalog tagline for the addon.",
			},
			"icon": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Icon the Portainer UI shows for the addon.",
			},
			"path": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Path the addon is served under in Portainer.",
			},
			"enabled": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the addon is installed and switched on. Manage it with `portainer_addon_access`.",
			},
			"lifecycle_status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Helm release lifecycle state: `installing`, `upgrading`, `installed`, `failed`, `unknown` or `uninstalling`. This is not a health status.",
			},
			"lifecycle_status_message": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Explanation of a failed install or upgrade, empty after a clean one.",
			},
			"health_status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Live probe of the running addon: `healthy`, `unhealthy`, `credential-invalid` or `unknown`. Orthogonal to `lifecycle_status` - an installed addon can be unhealthy.",
			},
			"health_message": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Explanation of an unhealthy probe result, empty otherwise.",
			},
			"chart_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Chart version actually installed, which is what to read when `version` is left unset.",
			},
			"chart_path": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Chart path recorded for the installed release, empty when the chart came from the public reference.",
			},
			"helm_chart_repository": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Public OCI chart reference the catalog publishes for the addon.",
			},
			"available_versions": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Published stable chart versions, newest first. Empty when the lookup could not run, in which case `version_check_error` says why.",
			},
			"upgrade_available": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether a newer chart version than the installed one has been published.",
			},
			"version_check_error": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Why the available-version lookup failed, empty when it succeeded.",
			},
		},
	}
}

func resourceAddonInstall(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	addonID := d.Get("addon_id").(string)

	payload := map[string]interface{}{}
	if v, ok := d.GetOk("chart"); ok && v.(string) != "" {
		payload["chart"] = v.(string)
	}
	if v, ok := d.GetOk("version"); ok && v.(string) != "" {
		payload["version"] = v.(string)
	}
	if v, ok := d.GetOk("registry_id"); ok && v.(int) != 0 {
		payload["registryId"] = v.(int)
	}
	// imageRegistryId is always sent, including the 0 Portainer reads as "the
	// image needs no credentials". Omitting it would instead mean "keep whatever
	// the addon is installed with", which is not what an absent attribute means
	// in a Terraform configuration.
	payload["imageRegistryId"] = d.Get("image_registry_id").(int)
	if raw, ok := d.GetOk("values"); ok && raw.(string) != "" {
		var values map[string]interface{}
		if err := json.Unmarshal([]byte(raw.(string)), &values); err != nil {
			return diag.FromErr(fmt.Errorf("failed to parse the addon's values as a JSON object: %w", err))
		}
		payload["values"] = values
	}

	if err := doJSON(ctx, client, http.MethodPost, addonURL(client, addonID), payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to install or upgrade addon %s: %w", addonID, err))
	}

	d.SetId(addonID)
	return resourceAddonRead(ctx, d, meta)
}

func resourceAddonRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	addon, err := readAddon(ctx, client, d.Id())
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read addon %s: %w", d.Id(), err))
	}
	if addon == nil {
		d.SetId("")
		return nil
	}

	// An addon stays in the catalog after it is uninstalled; what disappears is
	// its release. An empty lifecycle status is Portainer's way of saying the
	// addon was never installed, so that is what removes the resource from
	// state rather than a 404.
	if addon.LifecycleStatus == "" {
		d.SetId("")
		return nil
	}

	fields := map[string]interface{}{
		"addon_id":                 addon.ID,
		"display_name":             addon.DisplayName,
		"description":              addon.Description,
		"short_description":        addon.ShortDescription,
		"icon":                     addon.Icon,
		"path":                     addon.Path,
		"enabled":                  addon.Enabled,
		"lifecycle_status":         addon.LifecycleStatus,
		"lifecycle_status_message": addon.LifecycleStatusMessage,
		"health_status":            addon.HealthStatus,
		"health_message":           addon.HealthMessage,
	}
	if r := addon.Record; r != nil {
		fields["chart_version"] = r.ChartVersion
		fields["chart_path"] = r.ChartPath
		fields["helm_chart_repository"] = r.HelmChartRepository
		fields["available_versions"] = r.AvailableVersions
		fields["upgrade_available"] = r.UpgradeAvailable
		fields["version_check_error"] = r.VersionCheckError
		fields["image_registry_id"] = r.ImageRegistryID
		// registry_id is fixed at install time, so reading it back keeps an
		// imported resource from planning a reinstall against its own state.
		fields["registry_id"] = r.RegistryID
	}

	if err := setFields(d, fields); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceAddonUninstall(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	endpoint := fmt.Sprintf("%s?force=%t&pruneConfig=%t",
		addonURL(client, d.Id()),
		d.Get("force_uninstall").(bool),
		d.Get("prune_config_on_uninstall").(bool),
	)
	if err := doJSON(ctx, client, http.MethodDelete, endpoint, nil, nil); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to uninstall addon %s: %w", d.Id(), err))
	}

	d.SetId("")
	return nil
}
