package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type OpenAMTDeviceActionRequest struct {
	Action string `json:"action"`
}

func resourcePortainerOpenAMTDeviceAction() *schema.Resource {
	return &schema.Resource{
		Create: resourcePortainerOpenAMTDeviceActionCreate,
		Read:   schema.Noop,
		Update: schema.Noop,
		Delete: schema.RemoveFromState,
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

func resourcePortainerOpenAMTDeviceActionCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)
	deviceID := d.Get("device_id").(int)
	action := d.Get("action").(string)

	reqBody := OpenAMTDeviceActionRequest{Action: action}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/open_amt/%d/devices/%d/action", client.Endpoint, envID, deviceID)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}

	req.Header.Set("X-API-Key", client.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to execute AMT action: %s", resp.Status)
	}

	id := fmt.Sprintf("openamt-device-%d-action-%s", deviceID, action)
	d.SetId(id)
	return nil
}
