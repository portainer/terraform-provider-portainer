package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceDockerConfig() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceDockerConfigCreate,
		ReadContext:   resourceDockerConfigRead,
		UpdateContext: resourceDockerConfigUpdate,
		DeleteContext: resourceDockerConfigDelete,
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
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
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "ID of the Portainer environment (Docker Swarm) where the config is created.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Docker Swarm config.",
				// ForceNew: true,
			},
			"data": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				ForceNew:    true,
				Description: "Base64-encoded config payload stored in the Docker Swarm config.",
			},
			"labels": {
				Type:        schema.TypeMap,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				ForceNew:    true,
				Description: "Key/value labels attached to the Docker Swarm config.",
			},
			"templating": {
				Type:        schema.TypeMap,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				ForceNew:    true,
				Description: "Templating driver configuration applied to the config payload at runtime.",
			},
			"resource_control_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "ID of the Portainer resource control associated with this Docker Swarm config.",
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

	if resp.StatusCode != http.StatusOK {
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

type dockerConfigCreateResponse struct {
	ID        string `json:"ID"`
	Portainer struct {
		ResourceControl struct {
			Id int `json:"Id"`
		} `json:"ResourceControl"`
	} `json:"Portainer"`
}

func resourceDockerConfigCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	if existingID, err := findExistingDockerConfigByName(client, endpointID, name); err != nil {
		return diag.FromErr(fmt.Errorf("failed to check for existing docker config: %w", err))
	} else if existingID != "" {
		d.SetId(existingID)
		return resourceDockerConfigUpdate(ctx, d, meta)
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

	var response dockerConfigCreateResponse

	path := fmt.Sprintf("/endpoints/%d/docker/configs/create", endpointID)
	resp, err := client.DoRequest(http.MethodPost, path, nil, payload)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create docker config: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to create docker config: %s", string(body)))
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(response.ID)

	if response.Portainer.ResourceControl.Id != 0 {
		_ = d.Set("resource_control_id", response.Portainer.ResourceControl.Id)
	}

	return nil
}

func resourceDockerConfigRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	id := d.Id()

	path := fmt.Sprintf("/endpoints/%d/docker/configs/%s", endpointID, id)
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read docker config: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		d.SetId("")
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to read docker config: %s", string(body)))
	}

	var result struct {
		ID   string `json:"ID"`
		Spec struct {
			Name       string                 `json:"Name"`
			Labels     map[string]string      `json:"Labels"`
			Templating map[string]interface{} `json:"Templating"`
		} `json:"Spec"`
		Portainer struct {
			ResourceControl struct {
				Id int `json:"Id"`
			} `json:"ResourceControl"`
		} `json:"Portainer"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode docker config: %w", err))
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

	if result.Portainer.ResourceControl.Id != 0 {
		_ = d.Set("resource_control_id", result.Portainer.ResourceControl.Id)
	}

	return nil
}

func resourceDockerConfigDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	id := d.Id()

	path := fmt.Sprintf("/endpoints/%d/docker/configs/%s", endpointID, id)
	resp, err := client.DoRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to delete docker config: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to delete docker config: %s", string(body)))
	}

	d.SetId("")
	return nil
}

func resourceDockerConfigUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
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
		return diag.FromErr(fmt.Errorf("failed to update docker config: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to update docker config: %s", string(body)))
	}

	return resourceDockerConfigRead(ctx, d, meta)
}
