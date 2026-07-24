package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceDockerNode() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceDockerNodeRead,

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

func dataSourceDockerNodeRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	hostname := d.Get("hostname").(string)

	path := fmt.Sprintf("/endpoints/%d/docker/nodes", endpointID)
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
	// Nodes endpoint might fail if not in a Swarm cluster
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+path, nil, &nodes); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list docker nodes (is this a Swarm cluster?): %w", err))
	}

	for _, n := range nodes {
		if n.Description.Hostname == hostname {
			d.SetId(n.ID)
			if err := d.Set("role", n.Spec.Role); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set("status", n.Status.State); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("docker node with hostname %s not found in endpoint %d", hostname, endpointID))
}
