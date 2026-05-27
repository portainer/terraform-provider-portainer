package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceDockerNode() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceDockerNodeRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "ID of the Portainer environment (Docker Swarm cluster) where the node is located.",
			},
			"hostname": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Hostname of the Docker Swarm node to look up.",
			},
			"role": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Role of the Swarm node (worker or manager).",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Current status state of the Swarm node (e.g., ready, down, disconnected).",
			},
		},
	}
}

func dataSourceDockerNodeRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	hostname := d.Get("hostname").(string)

	path := fmt.Sprintf("/endpoints/%d/docker/nodes", endpointID)
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to list docker nodes: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		// Nodes endpoint might fail if not in a Swarm cluster
		return fmt.Errorf("failed to list docker nodes (is this a Swarm cluster?), status %d: %s", resp.StatusCode, string(data))
	}

	var nodes []struct {
		ID          string `json:"ID"`
		Description struct {
			Hostname string `json:"Hostname"`
		} `json:"Description"`
		Spec struct {
			Role string `json:"Role"`
		} `json:"Spec"`
		Status struct {
			State string `json:"State"`
		} `json:"Status"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&nodes); err != nil {
		return fmt.Errorf("failed to decode docker node list: %w", err)
	}

	for _, n := range nodes {
		if n.Description.Hostname == hostname {
			d.SetId(n.ID)
			d.Set("role", n.Spec.Role)
			d.Set("status", n.Status.State)
			return nil
		}
	}

	return fmt.Errorf("docker node with hostname %s not found in endpoint %d", hostname, endpointID)
}
