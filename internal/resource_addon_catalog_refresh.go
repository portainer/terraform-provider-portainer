package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// resourceAddonCatalogRefresh drops Portainer's cached addon catalog, so the
// next read loads it from the configured source. It is a one-shot action, so
// it has no read or update: the resource records that a refresh ran, and
// `triggers` is how another one is asked for.
func resourceAddonCatalogRefresh() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAddonCatalogRefreshCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: schema.NoopContext,

		Schema: map[string]*schema.Schema{
			"triggers": {
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Arbitrary values that force another refresh when they change, for example the catalog URL in the settings. A refresh is a one-shot action, so without a trigger the resource runs once and then stays put.",
			},
		},
	}
}

func resourceAddonCatalogRefreshCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/addons/catalog/refresh", nil, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to refresh the addon catalog: %w", err))
	}

	d.SetId(fmt.Sprintf("addon-catalog-refresh-%d", makeTimestamp()))
	return nil
}
