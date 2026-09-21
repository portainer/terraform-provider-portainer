package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// resourceSettingsDefaultRegistry controls whether Portainer offers the
// built-in anonymous Docker Hub registry when a user picks an image source.
// It has its own endpoint rather than being part of PUT /settings, so it is
// its own resource; folding it into portainer_settings would mean every
// settings apply also rewrote it.
func resourceSettingsDefaultRegistry() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSettingsDefaultRegistryWrite,
		ReadContext:   resourceSettingsDefaultRegistryRead,
		UpdateContext: resourceSettingsDefaultRegistryWrite,
		// There is nothing to delete: the setting always has a value. Removing
		// the resource stops managing it and leaves the registry as configured.
		DeleteContext: removeFromStateContext,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"hide": {
				Type:        schema.TypeBool,
				Required:    true,
				Description: "Whether to hide the built-in anonymous Docker Hub registry from the image source pickers, so users can only deploy from registries you have configured.",
			},
		},
	}
}

func resourceSettingsDefaultRegistryWrite(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := map[string]interface{}{"Hide": d.Get("hide").(bool)}
	url := client.Endpoint + "/settings/default_registry"
	if err := doJSON(ctx, client, http.MethodPut, url, payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update the default registry settings: %w", err))
	}

	d.SetId("portainer-settings-default-registry")
	return resourceSettingsDefaultRegistryRead(ctx, d, meta)
}

func resourceSettingsDefaultRegistryRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// The endpoint is write-only, so the current value is read from the main
	// settings object, which is where Portainer stores it.
	var settings struct {
		DefaultRegistry struct {
			Hide bool `json:"Hide"`
		} `json:"DefaultRegistry"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/settings", nil, &settings); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the default registry settings: %w", err))
	}

	d.SetId("portainer-settings-default-registry")
	if err := setFields(d, map[string]interface{}{"hide": settings.DefaultRegistry.Hide}); err != nil {
		return diag.FromErr(err)
	}
	return nil
}
