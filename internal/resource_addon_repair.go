package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// resourceAddonRepair reissues the credential an installed addon authenticates
// to Portainer with. It is a one-shot action, so it has no read or update: the
// resource records that a repair ran, and `triggers` is how another one is
// asked for.
func resourceAddonRepair() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAddonRepairCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: schema.NoopContext,

		Schema: map[string]*schema.Schema{
			"addon_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Catalog identifier of the addon whose credential is repaired, for example `portal-template`. Repair the addon when its `health_status` reports `credential-invalid`.",
			},
			"triggers": {
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Arbitrary values that force another repair when they change. A repair is a one-shot action, so without a trigger the resource runs once and then stays put.",
			},
			// Computed attributes
			"enabled": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the addon is enabled, as reported after the repair.",
			},
			"chart_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Chart version of the addon, as reported after the repair.",
			},
			"lifecycle_status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Helm release lifecycle state of the addon, as reported after the repair.",
			},
			"lifecycle_status_message": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Explanation of a failed lifecycle operation, as reported after the repair.",
			},
		},
	}
}

func resourceAddonRepairCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	addonID := d.Get("addon_id").(string)

	var result struct {
		ID                     string `json:"id"`
		Enabled                bool   `json:"enabled"`
		ChartVersion           string `json:"chartVersion"`
		LifecycleStatus        string `json:"lifecycleStatus"`
		LifecycleStatusMessage string `json:"lifecycleStatusMessage"`
	}
	endpoint := addonURL(client, addonID) + "/repair"
	if err := doJSON(ctx, client, http.MethodPost, endpoint, nil, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to repair the credential of addon %s: %w", addonID, err))
	}

	if err := setFields(d, map[string]interface{}{
		"enabled":                  result.Enabled,
		"chart_version":            result.ChartVersion,
		"lifecycle_status":         result.LifecycleStatus,
		"lifecycle_status_message": result.LifecycleStatusMessage,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%s-repair-%d", addonID, makeTimestamp()))
	return nil
}
