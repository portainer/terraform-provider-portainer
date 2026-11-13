package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// dataSourceAddonCatalog reads the addon catalog document itself, as the
// configured catalog URL publishes it: catalog content only, without the
// install state and records portainer_addons folds in.
func dataSourceAddonCatalog() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceAddonCatalogRead,

		Schema: map[string]*schema.Schema{
			// Computed attributes
			"addons": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Addons the catalog publishes.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Catalog identifier of the addon, the `addon_id` of the addon resources.",
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
						"service_url": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "In-cluster URL Portainer proxies the addon's requests to.",
						},
						"health_check_path": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Readiness path, appended to `service_url`, that Portainer probes for the addon's health.",
						},
						"helm_chart_repository": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Public OCI reference of the addon's Helm chart, for example `oci://ghcr.io/portainer/charts/portal-template`.",
						},
						"tls_verify": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether the TLS certificate of the registry `helm_chart_repository` points at is verified.",
						},
						"machine_access": {
							Type:        schema.TypeList,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "What the addon's machine token may reach beyond the addon's own settings and data. Empty for most addons.",
						},
					},
				},
			},
		},
	}
}

func dataSourceAddonCatalogRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// When the catalog source is unreachable Portainer serves its last good
	// copy; with no copy at all it answers 503, which fails the read rather
	// than reporting an empty catalog as current.
	var catalog struct {
		Addons []struct {
			ID                  string   `json:"id"`
			DisplayName         string   `json:"displayName"`
			Description         string   `json:"description"`
			ShortDescription    string   `json:"shortDescription"`
			Icon                string   `json:"icon"`
			Path                string   `json:"path"`
			ServiceURL          string   `json:"serviceUrl"`
			HealthCheckPath     string   `json:"healthCheckPath"`
			HelmChartRepository string   `json:"helmChartRepository"`
			TLSVerify           bool     `json:"tlsVerify"`
			MachineAccess       []string `json:"machineAccess"`
		} `json:"addons"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/addons/catalog", nil, &catalog); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the addon catalog: %w", err))
	}

	addons := make([]interface{}, 0, len(catalog.Addons))
	for _, a := range catalog.Addons {
		addons = append(addons, map[string]interface{}{
			"id":                    a.ID,
			"display_name":          a.DisplayName,
			"description":           a.Description,
			"short_description":     a.ShortDescription,
			"icon":                  a.Icon,
			"path":                  a.Path,
			"service_url":           a.ServiceURL,
			"health_check_path":     a.HealthCheckPath,
			"helm_chart_repository": a.HelmChartRepository,
			"tls_verify":            a.TLSVerify,
			"machine_access":        a.MachineAccess,
		})
	}

	if err := d.Set("addons", addons); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("portainer-addon-catalog")
	return nil
}
