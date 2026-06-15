package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return diag.FromErr(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s/settings/experimental", client.Endpoint), bytes.NewBuffer(jsonBody))
	if err != nil {
		return diag.FromErr(err)
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return diag.FromErr(fmt.Errorf("no valid authentication method provided (api_key or jwt token)"))
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to apply experimental settings: %s", string(body)))
	}

	d.SetId("portainer-experimental-settings")
	return nil
}

func resourceExperimentalSettingsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/settings/experimental", client.Endpoint), nil)
	if err != nil {
		return diag.FromErr(err)
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return diag.FromErr(fmt.Errorf("no valid authentication method provided (api_key or jwt token)"))
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to retrieve experimental settings: %s", string(body)))
	}

	var result struct {
		ExperimentalFeatures struct {
			OpenAIIntegration bool `json:"OpenAIIntegration"`
		} `json:"experimentalFeatures"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode response: %w", err))
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
