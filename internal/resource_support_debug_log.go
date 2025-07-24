package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePortainerSupportDebugLog() *schema.Resource {
	return &schema.Resource{
		Create: resourcePortainerSupportDebugLogApply,
		Read:   resourcePortainerSupportDebugLogRead,
		Update: resourcePortainerSupportDebugLogApply,
		Delete: resourcePortainerSupportDebugLogDisable,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"enabled": {
				Type:        schema.TypeBool,
				Required:    true,
				Description: "Enable or disable the global debug log",
			},
		},
	}
}

func resourcePortainerSupportDebugLogApply(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	payload := map[string]bool{
		"debugLogEnabled": d.Get("enabled").(bool),
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/support/debug_log", client.Endpoint), bytes.NewBuffer(jsonBody))
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
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to set debug log: %s", string(msg))
	}
	d.SetId(strconv.FormatBool(d.Get("enabled").(bool)))
	return nil
}

func resourcePortainerSupportDebugLogRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/support/debug_log", client.Endpoint), nil)
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
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read debug log status: %s", string(msg))
	}

	var result struct {
		DebugLogEnabled bool `json:"debugLogEnabled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	d.Set("enabled", result.DebugLogEnabled)
	d.SetId(strconv.FormatBool(result.DebugLogEnabled))
	return nil
}

func resourcePortainerSupportDebugLogDisable(d *schema.ResourceData, meta interface{}) error {
	d.Set("enabled", false)
	return resourcePortainerSupportDebugLogApply(d, meta)
}
