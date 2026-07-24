package internal

import (
	"context"
	"fmt"
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

func findExistingDockerConfigByName(ctx context.Context, client *APIClient, endpointID int, name string) (string, error) {
	var configs []map[string]interface{}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/endpoints/%d/docker/configs", client.Endpoint, endpointID), nil, &configs); err != nil {
		return "", fmt.Errorf("failed to list docker configs: %w", err)
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

	if existingID, err := findExistingDockerConfigByName(ctx, client, endpointID, name); err != nil {
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

	if err := doJSON(ctx, client, http.MethodPost, fmt.Sprintf("%s/endpoints/%d/docker/configs/create", client.Endpoint, endpointID), payload, &response); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create docker config: %w", err))
	}

	d.SetId(response.ID)

	if response.Portainer.ResourceControl.Id != 0 {
		if err := d.Set("resource_control_id", response.Portainer.ResourceControl.Id); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourceDockerConfigRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	id := d.Id()

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

	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/endpoints/%d/docker/configs/%s", client.Endpoint, endpointID, id), nil, &result); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read docker config: %w", err))
	}

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

	fields := map[string]interface{}{
		"name":       result.Spec.Name,
		"labels":     result.Spec.Labels,
		"templating": templ,
	}
	if result.Portainer.ResourceControl.Id != 0 {
		fields["resource_control_id"] = result.Portainer.ResourceControl.Id
	}
	if err := setFields(d, fields); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceDockerConfigDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	id := d.Id()

	if err := doJSON(ctx, client, http.MethodDelete, fmt.Sprintf("%s/endpoints/%d/docker/configs/%s", client.Endpoint, endpointID, id), nil, nil); err != nil {
		if !isAPINotFound(err) {
			return diag.FromErr(fmt.Errorf("failed to delete docker config: %w", err))
		}
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

	if err := doJSON(ctx, client, http.MethodPost, fmt.Sprintf("%s/endpoints/%d/docker/configs/%s/update", client.Endpoint, endpointID, id), payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update docker config: %w", err))
	}

	return resourceDockerConfigRead(ctx, d, meta)
}
