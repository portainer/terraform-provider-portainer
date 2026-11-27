package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

var newDockerNodeDataSource = dkNewDataSource(&dkDataSourceDef{
	typeSuffix: "_docker_node",
	schema:     dsDockerNodeSchema,
	read:       dsDockerNodeRead,
})

func dsDockerNodeSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Description: "Identifier of the resource.", Computed: true},
			"endpoint_id": schema.Int64Attribute{Required: true, Description: "ID of the Portainer environment (Docker Swarm cluster) where the node is located."},
			"hostname":    schema.StringAttribute{Required: true, Description: "Hostname of the Docker Swarm node to look up."},
			"role":        schema.StringAttribute{Computed: true, Description: "Role of the Swarm node (worker or manager)."},
			"status":      schema.StringAttribute{Computed: true, Description: "Current status state of the Swarm node (e.g., ready, down, disconnected)."},
		},
	}
}

func dsDockerNodeRead(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	hostname := dkString(d, "hostname")

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
		return fmt.Errorf("failed to list docker nodes (is this a Swarm cluster?): %w", err)
	}

	for _, n := range nodes {
		if n.Description.Hostname == hostname {
			d.SetID(n.ID)
			if err := d.Set("role", n.Spec.Role); err != nil {
				return err
			}
			if err := d.Set("status", n.Status.State); err != nil {
				return err
			}
			return nil
		}
	}

	return fmt.Errorf("docker node with hostname %s not found in endpoint %d", hostname, endpointID)
}
