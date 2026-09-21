package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// resourceSettingsAdditionalFunctionality switches the optional Portainer
// features on and off. Today that is the policy engine, which is what gates
// the portainer_policy resources.
func resourceSettingsAdditionalFunctionality() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSettingsAdditionalFunctionalityWrite,
		ReadContext:   resourceSettingsAdditionalFunctionalityRead,
		UpdateContext: resourceSettingsAdditionalFunctionalityWrite,
		// The setting always has a value; removing the resource stops managing
		// it rather than turning the feature off behind the operator's back.
		DeleteContext: removeFromStateContext,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"policies": {
				Type:        schema.TypeBool,
				Required:    true,
				Description: "Whether the policy engine is enabled. The `portainer_policy` resources need it switched on.",
			},
		},
	}
}

func resourceSettingsAdditionalFunctionalityWrite(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := map[string]interface{}{"Policies": d.Get("policies").(bool)}
	url := client.Endpoint + "/settings/additional_functionality"
	if err := doJSON(ctx, client, http.MethodPut, url, payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update the additional functionality settings: %w", err))
	}

	d.SetId("portainer-settings-additional-functionality")
	return resourceSettingsAdditionalFunctionalityRead(ctx, d, meta)
}

func resourceSettingsAdditionalFunctionalityRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var response struct {
		AdditionalFunctionality struct {
			Policies bool `json:"Policies"`
		} `json:"additionalFunctionality"`
	}
	url := client.Endpoint + "/settings/additional_functionality"
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &response); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the additional functionality settings: %w", err))
	}

	d.SetId("portainer-settings-additional-functionality")
	if err := setFields(d, map[string]interface{}{
		"policies": response.AdditionalFunctionality.Policies,
	}); err != nil {
		return diag.FromErr(err)
	}
	return nil
}
