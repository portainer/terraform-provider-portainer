package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
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
		CreateContext: resourcePortainerOpenAMTDevicesFeaturesCreate,
		ReadContext:   schema.NoopContext,
		UpdateContext: schema.NoopContext,
		DeleteContext: removeFromStateContext,
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

func resourcePortainerOpenAMTDevicesFeaturesCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
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

	url := fmt.Sprintf("%s/open_amt/%d/devices_features/%d", client.Endpoint, envID, deviceID)
	if err := doJSON(ctx, client, http.MethodPost, url, reqBody, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to enable AMT device features: %w", err))
	}

	d.SetId("amt-device-features-" + strconv.Itoa(deviceID))
	return nil
}
