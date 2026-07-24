package internal

import (
	"context"
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

	url := fmt.Sprintf("%s/open_amt/%d/devices/%d/action", client.Endpoint, envID, deviceID)
	if err := doJSON(ctx, client, http.MethodPost, url, reqBody, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to execute AMT action: %w", err))
	}

	id := fmt.Sprintf("openamt-device-%d-action-%s", deviceID, action)
	d.SetId(id)
	return nil
}
