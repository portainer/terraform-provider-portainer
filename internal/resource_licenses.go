package internal

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type LicensePayload struct {
	Key string `json:"key"`
}

type LicenseResponse struct {
	ConflictingKeys []string `json:"conflictingKeys"`
}

func resourceLicenses() *schema.Resource {
	return &schema.Resource{
		Create: resourceLicensesCreate,
		Read:   resourceLicensesRead,
		Delete: resourceLicensesDelete,
		Schema: map[string]*schema.Schema{
			"key": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "License key to be attached",
				Sensitive:   true,
				ForceNew:    true,
			},
			"force": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				ForceNew:    true,
				Description: "Force attach even if there are conflicting licenses",
			},
			"conflicting_keys": {
				Type:        schema.TypeList,
				Computed:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "List of conflicting license keys, if any",
			},
		},
	}
}

func resourceLicensesCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	licenseKey := d.Get("key").(string)
	force := d.Get("force").(bool)

	payload := LicensePayload{
		Key: licenseKey,
	}

	url := fmt.Sprintf("%s/licenses/add", client.Endpoint)
	if force {
		url += "?force=true"
	}

	var result LicenseResponse
	resp, err := client.DoRequest("POST", url, nil, payload)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to attach license: %s", string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to parse license response: %w", err)
	}

	if err := d.Set("conflicting_keys", result.ConflictingKeys); err != nil {
		return fmt.Errorf("failed to set conflicting_keys: %w", err)
	}

	d.SetId(licenseKey)
	return nil
}

func resourceLicensesRead(d *schema.ResourceData, meta interface{}) error {
	return nil // Not supported by Portainer API
}

func resourceLicensesDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
