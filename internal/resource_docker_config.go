package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceDockerConfig() *schema.Resource {
	return &schema.Resource{
		Create: resourceDockerConfigCreate,
		Read:   resourceDockerConfigRead,
		Update: nil, // resourceDockerConfigUpdate,
		Delete: resourceDockerConfigDelete,
		Importer: &schema.ResourceImporter{
			State: func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				importID := d.Id()
				var endpointID int
				var configID string
				n, err := fmt.Sscanf(importID, "%d-%s", &endpointID, &configID)
				if err != nil || n != 2 {
					return nil, fmt.Errorf("invalid import ID format. Expected '<endpoint_id>-<config_id>'")
				}
				if err := d.Set("endpoint_id", endpointID); err != nil {
					return nil, err
				}
				d.SetId(configID)
				return []*schema.ResourceData{d}, nil
			},
		},
		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"data": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
				ForceNew:  true,
			},
			"labels": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				ForceNew: true,
			},
			"templating": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
				ForceNew: true,
			},
		},
	}
}

func findExistingDockerConfigByName(client *APIClient, endpointID int, name string) (string, error) {
	path := fmt.Sprintf("/endpoints/%d/docker/configs", endpointID)
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return "", fmt.Errorf("failed to list docker configs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to list docker configs: %s", string(body))
	}

	var configs []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&configs); err != nil {
		return "", err
	}

	for _, cfg := range configs {
		if cfg["Spec"] != nil {
			spec := cfg["Spec"].(map[string]interface{})
			if spec["Name"] == name {
				if id, ok := cfg["ID"].(string); ok {
					return id, nil
				}
			}
		}
	}

	return "", nil
}

func resourceDockerConfigCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	if existingID, err := findExistingDockerConfigByName(client, endpointID, name); err != nil {
		return fmt.Errorf("failed to check for existing docker config: %w", err)
	} else if existingID != "" {
		d.SetId(existingID)
		return resourceDockerConfigUpdate(d, meta)
	}

	payload := map[string]interface{}{
		"Name":   name,
		"Data":   d.Get("data").(string),
		"Labels": d.Get("labels").(map[string]interface{}),
	}

	if v, ok := d.GetOk("templating"); ok {
		templating := v.(map[string]interface{})
		payload["Templating"] = map[string]interface{}{
			"Name":    templating["name"],
			"Options": templating,
		}
	}

	var response struct {
		ID string `json:"Id"`
	}

	path := fmt.Sprintf("/endpoints/%d/docker/configs/create", endpointID)
	resp, err := client.DoRequest(http.MethodPost, path, nil, payload)
	if err != nil {
		return fmt.Errorf("failed to create docker config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create docker config: %s", string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}

	d.SetId(response.ID)
	return nil
}

func resourceDockerConfigRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	id := d.Id()

	path := fmt.Sprintf("/endpoints/%d/docker/configs/%s", endpointID, id)
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to read docker config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	}
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read docker config: %s", string(body))
	}

	var result struct {
		ID   string `json:"ID"`
		Spec struct {
			Name       string                 `json:"Name"`
			Labels     map[string]string      `json:"Labels"`
			Templating map[string]interface{} `json:"Templating"`
		} `json:"Spec"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode docker config: %w", err)
	}

	_ = d.Set("name", result.Spec.Name)
	_ = d.Set("labels", result.Spec.Labels)

	templ := make(map[string]interface{})
	if t := result.Spec.Templating; t != nil {
		if name, ok := t["Name"]; ok {
			templ["name"] = name
		}
		if opts, ok := t["Options"].(map[string]interface{}); ok {
			for k, v := range opts {
				templ[k] = v
			}
		}
	}
	_ = d.Set("templating", templ)
	return nil
}

func resourceDockerConfigDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	id := d.Id()

	path := fmt.Sprintf("/endpoints/%d/docker/configs/%s", endpointID, id)
	resp, err := client.DoRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete docker config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 && resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete docker config: %s", string(body))
	}

	d.SetId("")
	return nil
}

func resourceDockerConfigUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	id := d.Id()

	payload := map[string]interface{}{
		"Name":   d.Get("name").(string),
		"Data":   d.Get("data").(string),
		"Labels": d.Get("labels").(map[string]interface{}),
	}

	if v, ok := d.GetOk("templating"); ok {
		templating := v.(map[string]interface{})
		payload["Templating"] = map[string]interface{}{
			"Name":    templating["name"],
			"Options": templating,
		}
	}

	path := fmt.Sprintf("/endpoints/%d/docker/configs/%s/update", endpointID, id)
	resp, err := client.DoRequest(http.MethodPost, path, nil, payload)
	if err != nil {
		return fmt.Errorf("failed to update docker config: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update docker config: %s", string(body))
	}

	return resourceDockerConfigRead(d, meta)
}
