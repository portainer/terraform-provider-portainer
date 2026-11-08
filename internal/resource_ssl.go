package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceSSLSettings() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceSSLSettingsUpdate,
		ReadContext:   resourceSSLSettingsRead,
		UpdateContext: resourceSSLSettingsUpdate,
		DeleteContext: resourceSSLSettingsDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"cert": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "SSL certificate content",
				Sensitive:   true,
			},
			"key": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "SSL private key content",
				Sensitive:   true,
			},
			"client_cert": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "SSL client certificate content",
				Sensitive:   true,
			},
			"http_enabled": {
				Type:        schema.TypeBool,
				Required:    true,
				Description: "Whether HTTP should be enabled (true) or disabled (false)",
			},
		},
	}
}

func resourceSSLSettingsUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// httpenabled is always sent: Portainer keeps the current setting when it
	// is missing, so leaving false out could never switch HTTP off.
	body := struct {
		Cert        string `json:"cert,omitempty"`
		Key         string `json:"key,omitempty"`
		ClientCert  string `json:"clientCert,omitempty"`
		HTTPEnabled bool   `json:"httpenabled"`
	}{
		Cert:        d.Get("cert").(string),
		Key:         d.Get("key").(string),
		ClientCert:  d.Get("client_cert").(string),
		HTTPEnabled: d.Get("http_enabled").(bool),
	}

	if err := doJSON(ctx, client, http.MethodPut, client.Endpoint+"/ssl", body, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update SSL settings: %w", err))
	}

	d.SetId("portainer-ssl")
	return nil
}

func resourceSSLSettingsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var settings struct {
		HTTPEnabled bool `json:"httpEnabled"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/ssl", nil, &settings); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read SSL settings: %w", err))
	}

	if err := d.Set("http_enabled", settings.HTTPEnabled); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-ssl")
	return nil
}

func resourceSSLSettingsDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId("")
	return nil
}
