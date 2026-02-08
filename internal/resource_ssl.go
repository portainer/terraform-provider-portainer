package internal

import (
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/ssl"
	"github.com/portainer/client-api-go/v2/pkg/models"
)

func resourceSSLSettings() *schema.Resource {
	return &schema.Resource{
		Create: resourceSSLSettingsUpdate,
		Read:   resourceSSLSettingsRead,
		Update: resourceSSLSettingsUpdate,
		Delete: resourceSSLSettingsDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
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

func resourceSSLSettingsUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	params := ssl.NewSSLUpdateParams()
	params.Body = &models.SslSslUpdatePayload{
		Cert:        d.Get("cert").(string),
		Key:         d.Get("key").(string),
		ClientCert:  d.Get("client_cert").(string),
		Httpenabled: d.Get("http_enabled").(bool),
	}

	_, err := client.Client.Ssl.SSLUpdate(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to update SSL settings: %w", err)
	}

	d.SetId("portainer-ssl")
	return nil
}

func resourceSSLSettingsRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	params := ssl.NewSSLInspectParams()
	resp, err := client.Client.Ssl.SSLInspect(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to read SSL settings: %w", err)
	}

	d.Set("http_enabled", resp.Payload.HTTPEnabled)
	d.SetId("portainer-ssl")
	return nil
}

func resourceSSLSettingsDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
