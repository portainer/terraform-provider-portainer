package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type AMTFeatures struct {
	IDER        bool   `json:"IDER"`
	KVM         bool   `json:"KVM"`
	SOL         bool   `json:"SOL"`
	Redirection bool   `json:"redirection"`
	UserConsent string `json:"userConsent"`
}

type EnableAMTFeaturesRequest struct {
	Features AMTFeatures `json:"features"`
}

func resourcePortainerOpenAMTDevicesFeatures() *schema.Resource {
	return &schema.Resource{
		Create: resourcePortainerOpenAMTDevicesFeaturesCreate,
		Read:   schema.Noop,
		Update: schema.Noop,
		Delete: schema.RemoveFromState,
		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Portainer environment (endpoint) ID.",
			},
			"device_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "ID of the AMT-managed device.",
			},
			"ider": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Enable IDER (IDE Redirection).",
			},
			"kvm": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Enable KVM (Keyboard/Video/Mouse).",
			},
			"sol": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Enable SOL (Serial Over LAN).",
			},
			"redirection": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Enable redirection.",
			},
			"user_consent": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "User consent policy (e.g., 'none', 'all', 'kvmOnly').",
			},
		},
	}
}

func resourcePortainerOpenAMTDevicesFeaturesCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)
	deviceID := d.Get("device_id").(int)

	features := AMTFeatures{
		IDER:        d.Get("ider").(bool),
		KVM:         d.Get("kvm").(bool),
		SOL:         d.Get("sol").(bool),
		Redirection: d.Get("redirection").(bool),
		UserConsent: d.Get("user_consent").(string),
	}

	reqBody := EnableAMTFeaturesRequest{Features: features}
	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/open_amt/%d/devices_features/%d", client.Endpoint, envID, deviceID)
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
		return fmt.Errorf("failed to enable AMT device features: %s", resp.Status)
	}

	d.SetId("amt-device-features-" + strconv.Itoa(deviceID))
	return nil
}
