package internal

import (
	"context"
	"crypto/sha256"
	"fmt"
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
				Type:         schema.TypeString,
				Optional:     true,
				ExactlyOneOf: []string{"key", "key_wo"},
				Description:  "License key to be attached. Stored in state as a sensitive value; use `key_wo` to keep it out of state.",
				Sensitive:    true,
				ForceNew:     true,
			},
			"key_wo":         writeOnlySecretSchema("key", "key_wo_version", "license key to be attached"),
			"key_wo_version": writeOnlyVersionSchema("key_wo", true),
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

	licenseKey, err := twinSecret(d, "key")
	if err != nil {
		return diag.FromErr(err)
	}
	force := d.Get("force").(bool)

	payload := LicensePayload{
		Key: licenseKey,
	}

	url := "/licenses/add"
	if force {
		url += "?force=true"
	}

	var result LicenseResponse
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+url, payload, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to attach license: %w", err))
	}

	if err := d.Set("conflicting_keys", result.ConflictingKeys); err != nil {
		return diag.FromErr(fmt.Errorf("failed to set conflicting_keys: %w", err))
	}

	d.SetId(fmt.Sprintf("%x", sha256.Sum256([]byte(licenseKey))))
	return nil
}

// attachedLicenseKey returns the attached license whose SHA-256 is the
// resource ID, or "" when it is not attached any more. The ID is all there is
// to go by after an import or with key_wo, which leave key empty.
func attachedLicenseKey(ctx context.Context, client *APIClient, id string) (string, error) {
	var licenses []struct {
		LicenseKey string `json:"licenseKey"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/licenses", nil, &licenses); err != nil {
		return "", fmt.Errorf("failed to get licenses: %w", err)
	}
	for _, lic := range licenses {
		if fmt.Sprintf("%x", sha256.Sum256([]byte(lic.LicenseKey))) == id {
			return lic.LicenseKey, nil
		}
	}
	return "", nil
}

func resourceLicensesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	key, err := attachedLicenseKey(ctx, meta.(*APIClient), d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	if key == "" {
		d.SetId("")
	}
	return nil
}

func resourceLicensesDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	key, err := attachedLicenseKey(ctx, client, d.Id())
	if err != nil {
		return diag.FromErr(err)
	}
	if key == "" {
		d.SetId("")
		return nil
	}
	payload := map[string]interface{}{
		"licenseKeys": []string{key},
	}

	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/licenses/remove", payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to send license removal request: %w", err))
	}

	d.SetId("")
	return nil
}
