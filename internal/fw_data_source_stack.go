package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

var newStackDataSource = dkNewDataSource(&dkDataSourceDef{
	typeSuffix: "_stack",
	schema:     dsStackSchema,
	read:       dsStackRead,
})

func dsStackSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Description: "Identifier of the resource.", Computed: true},
			"endpoint_id": schema.Int64Attribute{Required: true, Description: "Identifier of the Portainer environment (endpoint) where the stack is deployed."},
			"name":        schema.StringAttribute{Required: true, Description: "Name of the Portainer stack to look up. Combined with endpoint_id to uniquely identify the stack."},
			"swarm_id":    schema.StringAttribute{Computed: true, Description: "Identifier of the Docker Swarm cluster the stack is deployed to. Empty for non-Swarm stacks."},
			"type":        schema.Int64Attribute{Computed: true, Description: "Type of the Portainer stack: 1 = Swarm, 2 = Compose, 3 = Kubernetes."},
		},
	}
}

func dsStackRead(ctx context.Context, d *dkBag, client *APIClient) error {
	name := dkString(d, "name")
	endpointID := dkInt(d, "endpoint_id")

	var stacks []struct {
		ID         int    `json:"Id"`
		Name       string `json:"Name"`
		EndpointID int    `json:"EndpointId"`
		Type       int    `json:"Type"`
		SwarmID    string `json:"SwarmId"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/stacks", nil, &stacks); err != nil {
		return fmt.Errorf("failed to list stacks: %w", err)
	}

	for _, s := range stacks {
		if s.Name == name && s.EndpointID == endpointID {
			d.SetID(strconv.Itoa(s.ID))
			if err := d.Set("type", s.Type); err != nil {
				return err
			}
			if err := d.Set("swarm_id", s.SwarmID); err != nil {
				return err
			}
			return nil
		}
	}

	return fmt.Errorf("stack %s not found in endpoint %d", name, endpointID)
}
