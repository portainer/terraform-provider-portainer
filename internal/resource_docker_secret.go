package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceDockerSecret() *schema.Resource {
	return &schema.Resource{
		Create: resourceDockerSecretCreate,
		Read:   resourceDockerSecretRead,
		Delete: resourceDockerSecretDelete,
		Update: resourceDockerSecretUpdate,
		Schema: map[string]*schema.Schema{
			"endpoint_id": {Type: schema.TypeInt, Required: true},
			"name":        {Type: schema.TypeString, Required: true},
			"data":        {Type: schema.TypeString, Required: true, Sensitive: true},
			"labels":      {Type: schema.TypeMap, Optional: true, Elem: &schema.Schema{Type: schema.TypeString}},
			"driver": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"templating": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceDockerSecretCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	payload := map[string]interface{}{
		"Name":   d.Get("name").(string),
		"Data":   d.Get("data").(string),
		"Labels": d.Get("labels").(map[string]interface{}),
	}

	if v, ok := d.GetOk("driver"); ok {
		payload["Driver"] = map[string]interface{}{
			"Name":    v.(map[string]interface{})["name"],
			"Options": v.(map[string]interface{}),
		}
	}

	if v, ok := d.GetOk("templating"); ok {
		payload["Templating"] = map[string]interface{}{
			"Name":    v.(map[string]interface{})["name"],
			"Options": v.(map[string]interface{}),
		}
	}

	var response struct {
		ID string `json:"Id"`
	}

	path := fmt.Sprintf("/endpoints/%d/docker/secrets/create", endpointID)
	resp, err := client.DoRequest(http.MethodPost, path, nil, payload)
	if err != nil {
		return fmt.Errorf("failed to create docker secret: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create docker secret: %s", string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}

	d.SetId(response.ID)
	return nil
}

func resourceDockerSecretRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceDockerSecretUpdate(d *schema.ResourceData, meta interface{}) error {
	if err := resourceDockerSecretDelete(d, meta); err != nil {
		return fmt.Errorf("failed to delete docker secret during update: %w", err)
	}

	return resourceDockerSecretCreate(d, meta)
}

func resourceDockerSecretDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	id := d.Id()

	path := fmt.Sprintf("/endpoints/%d/docker/secrets/%s", endpointID, id)
	resp, err := client.DoRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete docker secret: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 && resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete docker secret: %s", string(body))
	}

	d.SetId("")
	return nil
}
