package internal

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type sslSettings struct {
	Cert        string `json:"cert"`
	Key         string `json:"key"`
	ClientCert  string `json:"clientCert"`
	HTTPEnabled bool   `json:"httpenabled"`
}

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

	payload := sslSettings{
		Cert:        d.Get("cert").(string),
		Key:         d.Get("key").(string),
		ClientCert:  d.Get("client_cert").(string),
		HTTPEnabled: d.Get("http_enabled").(bool),
	}

	resp, err := client.DoRequest("PUT", "/ssl", nil, payload)
	if err != nil {
		return fmt.Errorf("failed to update SSL settings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update SSL settings: %s", string(body))
	}

	d.SetId("portainer-ssl")
	return nil
}

func resourceSSLSettingsRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	resp, err := client.DoRequest("GET", "/ssl", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to read SSL settings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read SSL settings: %s", string(body))
	}

	var result struct {
		HTTPEnabled bool `json:"httpEnabled"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	d.Set("http_enabled", result.HTTPEnabled)
	d.SetId("portainer-ssl")
	return nil
}

func resourceSSLSettingsDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
