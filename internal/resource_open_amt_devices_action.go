package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type OpenAMTDeviceActionRequest struct {
	Action string `json:"action"`
}

func resourcePortainerOpenAMTDeviceAction() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePortainerOpenAMTDeviceActionCreate,
		ReadContext:   schema.NoopContext,
		UpdateContext: schema.NoopContext,
		DeleteContext: removeFromStateContext,
		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "The ID of the environment (endpoint).",
			},
			"device_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "The ID of the AMT managed device.",
			},
			"action": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The out-of-band action to execute on the device (e.g. poweron, poweroff, reset).",
			},
		},
	}
}

func resourcePortainerOpenAMTDeviceActionCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)
	deviceID := d.Get("device_id").(int)
	action := d.Get("action").(string)

	reqBody := OpenAMTDeviceActionRequest{Action: action}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return diag.FromErr(err)
	}

	url := fmt.Sprintf("%s/open_amt/%d/devices/%d/action", client.Endpoint, envID, deviceID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
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
		return diag.FromErr(fmt.Errorf("failed to execute AMT action: %s", resp.Status))
	}

	id := fmt.Sprintf("openamt-device-%d-action-%s", deviceID, action)
	d.SetId(id)
	return nil
}
