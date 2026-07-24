package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type DockerNodeUpdatePayload struct {
	Availability string            `json:"Availability,omitempty"`
	Name         string            `json:"Name,omitempty"`
	Role         string            `json:"Role,omitempty"`
	Labels       map[string]string `json:"Labels,omitempty"`
}

func resourceDockerNode() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceDockerNodeUpdate,
		ReadContext:   resourceDockerNodeRead,
		UpdateContext: resourceDockerNodeUpdate,
		DeleteContext: resourceDockerNodeDelete,
		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "ID of the Portainer environment (Docker Swarm cluster) where the node is managed.",
			},
			"node_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Docker Swarm node ID to update.",
			},
			"version": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Swarm node version required for update operation",
			},
			"name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Desired display name for the Swarm node.",
			},
			"availability": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Desired availability of the Swarm node (active, pause, or drain).",
			},
			"role": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Desired role of the Swarm node (worker or manager).",
			},
			"labels": {
				Type:        schema.TypeMap,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Key/value labels assigned to the Swarm node for placement and selection.",
			},
		},
	}
}

func resourceDockerNodeUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	nodeID := d.Get("node_id").(string)
	version := d.Get("version").(int)

	payload := DockerNodeUpdatePayload{
		Availability: d.Get("availability").(string),
		Name:         d.Get("name").(string),
		Role:         d.Get("role").(string),
		Labels:       convertMapsString(d.Get("labels").(map[string]interface{})),
	}

	reqURL := fmt.Sprintf("%s/endpoints/%d/docker/nodes/%s/update?version=%d", client.Endpoint, endpointID, url.PathEscape(nodeID), version)
	if err := doJSON(ctx, client, http.MethodPost, reqURL, payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update node: %w", err))
	}

	d.SetId(fmt.Sprintf("%d-%s", endpointID, nodeID))
	return nil
}

func resourceDockerNodeRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	nodeID := d.Get("node_id").(string)

	reqURL := fmt.Sprintf("%s/endpoints/%d/docker/nodes/%s", client.Endpoint, endpointID, nodeID)

	var result struct {
		ID      string `json:"ID"`
		Version struct {
			Index int `json:"Index"`
		} `json:"Version"`
		Spec struct {
			Availability string            `json:"Availability"`
			Name         string            `json:"Name"`
			Role         string            `json:"Role"`
			Labels       map[string]string `json:"Labels"`
		} `json:"Spec"`
	}

	if err := doJSON(ctx, client, http.MethodGet, reqURL, nil, &result); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read node: %w", err))
	}

	if err := d.Set("version", result.Version.Index); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("name", result.Spec.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("availability", result.Spec.Availability); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("role", result.Spec.Role); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("labels", result.Spec.Labels); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("%d-%s", endpointID, nodeID))
	return nil
}

func resourceDockerNodeDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	nodeID := d.Get("node_id").(string)

	reqURL := fmt.Sprintf("%s/endpoints/%d/docker/nodes/%s", client.Endpoint, endpointID, url.PathEscape(nodeID))
	if err := doJSON(ctx, client, http.MethodDelete, reqURL, nil, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to delete node: %w", err))
	}

	d.SetId("")
	return nil
}

func convertMapsString(input map[string]interface{}) map[string]string {
	out := make(map[string]string)
	for k, v := range input {
		out[k] = fmt.Sprintf("%v", v)
	}
	return out
}
