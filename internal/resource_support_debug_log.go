package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePortainerSupportDebugLog() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePortainerSupportDebugLogApply,
		ReadContext:   resourcePortainerSupportDebugLogRead,
		UpdateContext: resourcePortainerSupportDebugLogApply,
		DeleteContext: resourcePortainerSupportDebugLogDisable,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
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

func resourcePortainerSupportDebugLogApply(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := map[string]bool{
		"debugLogEnabled": d.Get("enabled").(bool),
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return diag.FromErr(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s/support/debug_log", client.Endpoint), bytes.NewBuffer(jsonBody))
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
		msg, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to set debug log: %s", string(msg)))
	}
	d.SetId(strconv.FormatBool(d.Get("enabled").(bool)))
	return nil
}

func resourcePortainerSupportDebugLogRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/support/debug_log", client.Endpoint), nil)
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
		msg, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to read debug log status: %s", string(msg)))
	}

	var result struct {
		DebugLogEnabled bool `json:"debugLogEnabled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("enabled", result.DebugLogEnabled); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(strconv.FormatBool(result.DebugLogEnabled))
	return nil
}

func resourcePortainerSupportDebugLogDisable(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if err := d.Set("enabled", false); err != nil {
		return diag.FromErr(err)
	}
	return resourcePortainerSupportDebugLogApply(ctx, d, meta)
}
