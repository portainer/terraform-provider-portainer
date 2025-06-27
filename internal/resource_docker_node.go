package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"

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
		Create: resourceDockerNodeUpdate,
		Read:   resourceDockerNodeRead,
		Update: resourceDockerNodeUpdate,
		Delete: resourceDockerNodeDelete,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"node_id": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"version": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Swarm node version required for update operation",
			},
			"name": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"availability": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"role": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"labels": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceDockerNodeUpdate(d *schema.ResourceData, meta interface{}) error {
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

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal node update payload: %w", err)
	}

	reqURL := fmt.Sprintf("%s/endpoints/%d/docker/nodes/%s/update?version=%d", client.Endpoint, endpointID, url.PathEscape(nodeID), version)
	req, err := http.NewRequest("POST", reqURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to update node, status: %d, body: %s", resp.StatusCode, string(respBody))
	}

	d.SetId(fmt.Sprintf("%d-%s", endpointID, nodeID))
	return nil
}

func resourceDockerNodeRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	nodeID := d.Get("node_id").(string)

	url := fmt.Sprintf("%s/endpoints/%d/docker/nodes/%s", client.Endpoint, endpointID, nodeID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to build read request: %w", err)
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send read request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	} else if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read node: %s", string(body))
	}

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

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	d.Set("version", result.Version.Index)
	d.Set("name", result.Spec.Name)
	d.Set("availability", result.Spec.Availability)
	d.Set("role", result.Spec.Role)
	d.Set("labels", result.Spec.Labels)
	d.SetId(fmt.Sprintf("%d-%s", endpointID, nodeID))
	return nil
}

func resourceDockerNodeDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	nodeID := d.Get("node_id").(string)

	reqURL := fmt.Sprintf("%s/endpoints/%d/docker/nodes/%s", client.Endpoint, endpointID, url.PathEscape(nodeID))
	req, err := http.NewRequest("DELETE", reqURL, nil)
	if err != nil {
		return fmt.Errorf("failed to build delete request: %w", err)
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send delete request: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to delete node, status: %d, body: %s", resp.StatusCode, string(respBody))
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
