package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type ExperimentalSettingsPayload struct {
	OpenAIIntegration bool `json:"openAIIntegration"`
}

func resourceExperimentalSettings() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceExperimentalSettingsApply,
		ReadContext:   resourceExperimentalSettingsRead,
		UpdateContext: resourceExperimentalSettingsApply,
		DeleteContext: resourceExperimentalSettingsDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"openai_integration": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Enable or disable OpenAI integration.",
			},
		},
	}
}

func resourceExperimentalSettingsApply(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := ExperimentalSettingsPayload{
		OpenAIIntegration: d.Get("openai_integration").(bool),
	}

	if err := doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/settings/experimental", client.Endpoint), payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to apply experimental settings: %w", err))
	}

	d.SetId("portainer-experimental-settings")
	return nil
}

func resourceExperimentalSettingsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var result struct {
		ExperimentalFeatures struct {
			OpenAIIntegration bool `json:"OpenAIIntegration"`
		} `json:"experimentalFeatures"`
	}

	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/settings/experimental", client.Endpoint), nil, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to retrieve experimental settings: %w", err))
	}

	if err := d.Set("openai_integration", result.ExperimentalFeatures.OpenAIIntegration); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-experimental-settings")
	return nil
}

func resourceExperimentalSettingsDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// No DELETE endpoint; clear the ID to remove from state
	d.SetId("")
	return nil
}
