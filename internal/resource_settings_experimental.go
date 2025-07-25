package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type ExperimentalSettingsPayload struct {
	OpenAIIntegration bool `json:"openAIIntegration"`
}

func resourceExperimentalSettings() *schema.Resource {
	return &schema.Resource{
		Create: resourceExperimentalSettingsApply,
		Read:   resourceExperimentalSettingsRead,
		Update: resourceExperimentalSettingsApply,
		Delete: resourceExperimentalSettingsDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
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

func resourceExperimentalSettingsApply(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	payload := ExperimentalSettingsPayload{
		OpenAIIntegration: d.Get("openai_integration").(bool),
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/settings/experimental", client.Endpoint), bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to apply experimental settings: %s", string(body))
	}

	d.SetId("portainer-experimental-settings")
	return nil
}

func resourceExperimentalSettingsRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/settings/experimental", client.Endpoint), nil)
	if err != nil {
		return err
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to retrieve experimental settings: %s", string(body))
	}

	var result struct {
		ExperimentalFeatures struct {
			OpenAIIntegration bool `json:"OpenAIIntegration"`
		} `json:"experimentalFeatures"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	d.Set("openai_integration", result.ExperimentalFeatures.OpenAIIntegration)
	d.SetId("portainer-experimental-settings")
	return nil
}

func resourceExperimentalSettingsDelete(d *schema.ResourceData, meta interface{}) error {
	// No DELETE endpoint; clear the ID to remove from state
	d.SetId("")
	return nil
}
