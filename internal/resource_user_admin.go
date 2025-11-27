package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceUserAdmin() *schema.Resource {
	return &schema.Resource{
		Create: resourceUserAdminCreate,
		Read:   resourceUserAdminRead,
		Update: resourceUserAdminUpdate,
		Delete: resourceUserAdminDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"username": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "admin",
				Description: "Username of the admin account to initialize (defaults to 'admin').",
			},
			"password": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				Description: "Password for the admin account. Only used during initial bootstrap.",
			},
			"initialized": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the admin user has been initialized via this resource.",
			},
		},
	}
}

func resourceUserAdminCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	username := d.Get("username").(string)
	password := d.Get("password").(string)

	payload := map[string]string{
		"username": username,
		"password": password,
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal admin init payload: %w", err)
	}

	// IMPORTANT: this endpoint is PUBLIC – do not send any auth headers.
	url := fmt.Sprintf("%s/users/admin/init", client.Endpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return fmt.Errorf("failed to build admin init request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform admin init request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)

	// Treat 409 (admin already initialized) as a successful, idempotent create.
	if resp.StatusCode == http.StatusConflict {
		// Admin already exists – we just mark the resource as initialized.
		if d.Id() == "" {
			d.SetId("portainer-admin")
		}
		_ = d.Set("initialized", true)
		return nil
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("failed to initialize admin user, status %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID       int    `json:"Id"`
		Username string `json:"Username"`
	}
	_ = json.Unmarshal(body, &result)

	if result.ID != 0 {
		d.SetId(fmt.Sprintf("%d", result.ID))
	} else {
		d.SetId("portainer-admin")
	}

	_ = d.Set("initialized", true)

	return nil
}

func resourceUserAdminRead(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}

func resourceUserAdminUpdate(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}

func resourceUserAdminDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
