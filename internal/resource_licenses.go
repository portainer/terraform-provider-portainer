package internal

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
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
		CreateContext: resourceLicensesCreate,
		ReadContext:   resourceLicensesRead,
		DeleteContext: resourceLicensesDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
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

func resourceLicensesCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	licenseKey := d.Get("key").(string)
	force := d.Get("force").(bool)

	payload := LicensePayload{
		Key: licenseKey,
	}

	url := "/licenses/add"
	if force {
		url += "?force=true"
	}

	var result LicenseResponse
	resp, err := client.DoRequest("POST", url, nil, payload)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to attach license: %s", string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to parse license response: %w", err))
	}

	if err := d.Set("conflicting_keys", result.ConflictingKeys); err != nil {
		return diag.FromErr(fmt.Errorf("failed to set conflicting_keys: %w", err))
	}

	d.SetId(fmt.Sprintf("%x", sha256.Sum256([]byte(licenseKey))))
	return nil
}

func resourceLicensesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	resp, err := client.DoRequest("GET", "/licenses", nil, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to get licenses: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to read licenses, status %d: %s", resp.StatusCode, string(body)))
	}

	var licenses []struct {
		LicenseKey string `json:"licenseKey"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&licenses); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode licenses list: %w", err))
	}

	currentKey := d.Get("key").(string)
	found := false
	for _, lic := range licenses {
		if lic.LicenseKey == currentKey {
			found = true
			break
		}
	}

	if !found {
		d.SetId("")
	}
	return nil
}

func resourceLicensesDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := map[string]interface{}{
		"licenseKeys": []string{d.Get("key").(string)},
	}

	resp, err := client.DoRequest("POST", "/licenses/remove", nil, payload)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to send license removal request: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to delete license: %s", string(body)))
	}

	d.SetId("")
	return nil
}
