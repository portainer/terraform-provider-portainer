package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func dataSourceAddons() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceAddonsRead,

		Schema: map[string]*schema.Schema{
			"view": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validation.StringInSlice([]string{"switcher"}, false),
				Description:  "Response view. Set to `switcher` for the lightweight app-switcher listing, which omits the catalog details and is readable by non-admin users. Leave unset for the full listing.",
			},
			// Computed attributes
			"environment_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Identifier of the environment addon resources are read through, as reported by Portainer.",
			},
			"catalog_error": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Why the addon catalog could not be refreshed. When addons are still listed the listing is a cached copy; when it is empty nothing could be loaded. Admin-only, and never set on the `switcher` view.",
			},
			"addons": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Addons the connected Portainer offers.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Catalog identifier of the addon.",
						},
						"display_name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Human-readable name of the addon.",
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
							Description: "Whether the addon is installed and switched on.",
						},
						"lifecycle_status": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Helm release lifecycle state of the addon, empty when it was never installed.",
						},
						"lifecycle_status_message": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Explanation of a failed install or upgrade, empty after a clean one.",
						},
						"health_status": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Live probe of the running addon: `healthy`, `unhealthy`, `credential-invalid` or `unknown`.",
						},
						"health_message": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Explanation of an unhealthy probe result, empty otherwise.",
						},
						"chart_version": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Chart version currently installed, empty when the addon is not installed or the caller cannot see the admin-only record.",
						},
						"upgrade_available": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether a newer chart version than the installed one has been published.",
						},
						"available_versions": {
							Type:        schema.TypeList,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Published stable chart versions, newest first.",
						},
					},
				},
			},
		},
	}
}

func dataSourceAddonsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	endpoint := client.Endpoint + "/addons"
	if view, ok := d.GetOk("view"); ok && view.(string) != "" {
		endpoint += "?view=" + view.(string)
	}

	var listing struct {
		Addons       []addonListItem `json:"addons"`
		CatalogError string          `json:"catalogError"`
	}
	if err := doJSON(ctx, client, http.MethodGet, endpoint, nil, &listing); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the addons: %w", err))
	}

	addons := make([]interface{}, 0, len(listing.Addons))
	for _, addon := range listing.Addons {
		entry := map[string]interface{}{
			"id":                       addon.ID,
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
		// The record is admin-only, so the fields drawn from it stay at their
		// zero values for anyone else rather than failing the read.
		if r := addon.Record; r != nil {
			entry["chart_version"] = r.ChartVersion
			entry["upgrade_available"] = r.UpgradeAvailable
			entry["available_versions"] = r.AvailableVersions
		}
		addons = append(addons, entry)
	}

	var environment struct {
		EnvironmentID int `json:"environmentId"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/addons/environment", nil, &environment); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the environment addon resources are served from: %w", err))
	}

	if err := setFields(d, map[string]interface{}{
		"addons":         addons,
		"catalog_error":  listing.CatalogError,
		"environment_id": environment.EnvironmentID,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("portainer-addons")
	return nil
}
