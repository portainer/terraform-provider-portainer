package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

var newDockerNetworkDataSource = dkNewDataSource(&dkDataSourceDef{
	typeSuffix: "_docker_network",
	schema:     dsDockerNetworkSchema,
	read:       dsDockerNetworkRead,
})

func dsDockerNetworkSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Description: "Identifier of the resource.", Computed: true},
			"driver":      schema.StringAttribute{Computed: true, Description: "Driver used by the Docker network (e.g., bridge, overlay, macvlan, host)."},
			"endpoint_id": schema.Int64Attribute{Required: true, Description: "ID of the Portainer environment (Docker host or Swarm) where the network is located."},
			"name":        schema.StringAttribute{Required: true, Description: "Name of the Docker network to look up."},
			"scope":       schema.StringAttribute{Computed: true, Description: "Scope of the Docker network (e.g., local, global, swarm)."},
		},
	}
}

func dsDockerNetworkRead(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	name := dkString(d, "name")

	path := fmt.Sprintf("/endpoints/%d/docker/networks", endpointID)
	var networks []struct {
		ID     string `json:"Id"`
		Name   string `json:"Name"`
		Driver string `json:"Driver"`
		Scope  string `json:"Scope"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+path, nil, &networks); err != nil {
		return fmt.Errorf("failed to list docker networks: %w", err)
	}

	for _, n := range networks {
		if n.Name == name {
			d.SetID(n.ID)
			if err := d.Set("driver", n.Driver); err != nil {
				return err
			}
			if err := d.Set("scope", n.Scope); err != nil {
				return err
			}
			return nil
		}
	}

	return fmt.Errorf("docker network %s not found in endpoint %d", name, endpointID)
}
