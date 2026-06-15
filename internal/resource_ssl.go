package internal

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/ssl"
	"github.com/portainer/client-api-go/v2/pkg/models"
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

	ctx, errBody := withErrorCapture(ctx)
	params := ssl.NewSSLUpdateParams()
	params.SetContext(ctx)
	params.Body = &models.SslSslUpdatePayload{
		Cert:        d.Get("cert").(string),
		Key:         d.Get("key").(string),
		ClientCert:  d.Get("client_cert").(string),
		Httpenabled: d.Get("http_enabled").(bool),
	}

	_, err := client.Client.Ssl.SSLUpdate(params, client.AuthInfo)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to update SSL settings: %w", decorateSDKError(err, errBody)))
	}

	d.SetId("portainer-ssl")
	return nil
}

func resourceSSLSettingsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	ctx, errBody := withErrorCapture(ctx)
	params := ssl.NewSSLInspectParams()
	params.SetContext(ctx)
	resp, err := client.Client.Ssl.SSLInspect(params, client.AuthInfo)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read SSL settings: %w", decorateSDKError(err, errBody)))
	}

	if err := d.Set("http_enabled", resp.Payload.HTTPEnabled); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-ssl")
	return nil
}

func resourceSSLSettingsDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	d.SetId("")
	return nil
}
