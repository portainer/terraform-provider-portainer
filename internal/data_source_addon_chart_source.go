package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// dataSourceAddonChartSource checks where an addon's chart would be pulled
// from, which is the way to find out whether a registry is usable and what
// versions it offers before an install is attempted.
func dataSourceAddonChartSource() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceAddonChartSourceRead,

		Schema: map[string]*schema.Schema{
			"addon_id": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Catalog identifier of the addon whose chart source is checked, for example `portal-template`.",
			},
			"registry_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Identifier of the Portainer registry to check against. Leave unset to check the catalog's public chart reference.",
			},
			"chart": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Chart path within `registry_id` to check, for example `portainer/charts/portainer-run`. Leave unset when checking the public reference.",
			},
			// Computed attributes
			"reachable": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the registry answered. A registry that answered but holds no such chart is still reachable, with an empty `versions`.",
			},
			"reason": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Why the source is unreachable, empty when `reachable` is true.",
			},
			"tls_verify": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the certificate the registry served verified against the trust store.",
			},
			"versions": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Published stable versions of the chart, newest first.",
			},
		},
	}
}

func dataSourceAddonChartSourceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	addonID := d.Get("addon_id").(string)

	query := url.Values{}
	if v, ok := d.GetOk("registry_id"); ok && v.(int) != 0 {
		query.Set("registryId", strconv.Itoa(v.(int)))
	}
	if v, ok := d.GetOk("chart"); ok && v.(string) != "" {
		query.Set("chart", v.(string))
	}

	endpoint := addonURL(client, addonID) + "/chart-source"
	if len(query) > 0 {
		endpoint += "?" + query.Encode()
	}

	var status struct {
		Reachable bool     `json:"reachable"`
		Reason    string   `json:"reason"`
		TLSVerify bool     `json:"tlsVerify"`
		Versions  []string `json:"versions"`
	}
	if err := doJSON(ctx, client, http.MethodGet, endpoint, nil, &status); err != nil {
		return diag.FromErr(fmt.Errorf("failed to check the chart source of addon %s: %w", addonID, err))
	}

	if err := setFields(d, map[string]interface{}{
		"reachable":  status.Reachable,
		"reason":     status.Reason,
		"tls_verify": status.TLSVerify,
		"versions":   status.Versions,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("portainer-addon-chart-source-%s", addonID))
	return nil
}
