package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceInit() *schema.Resource {
	return &schema.Resource{
		Create: resourceInitCreate,
		Read:   schema.Noop,
		Delete: schema.Noop,

		Schema: map[string]*schema.Schema{
			"username": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
				ForceNew:  true,
			},
			"password": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
				ForceNew:  true,
			},
		},
	}
}

func resourceInitCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	plainPassword := d.Get("password").(string)

	creds := map[string]string{
		"username": d.Get("username").(string),
		"password": plainPassword, // Send plain password
	}

	// Create a direct HTTP request WITHOUT authentication headers
	jsonData, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("failed to marshal credentials: %w", err)
	}

	// Use a basic HTTP client instead of client.DoRequest
	httpClient := &http.Client{}
	req, err := http.NewRequest("POST", client.Endpoint+"/api/users/admin/init", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	// DON'T set Authorization header - that's the key difference

	resp, err := httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusConflict {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("admin user already initialized: %s", string(data))
	} else if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to initialize Portainer: %s", string(data))
	}

	d.SetId("init-result")
	return nil
}
