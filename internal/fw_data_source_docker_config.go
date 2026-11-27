package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

var newDockerConfigDataSource = dkNewDataSource(&dkDataSourceDef{
	typeSuffix: "_docker_config",
	schema:     dsDockerConfigSchema,
	read:       dsDockerConfigRead,
})

func dsDockerConfigSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Description: "Identifier of the resource.", Computed: true},
			"endpoint_id": schema.Int64Attribute{Required: true, Description: "Identifier of the Portainer endpoint hosting the Docker config."},
			"name":        schema.StringAttribute{Required: true, Description: "Name of the Docker config to look up."},
		},
	}
}

func dsDockerConfigRead(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	name := dkString(d, "name")

	path := fmt.Sprintf("/endpoints/%d/docker/configs", endpointID)
	var configs []struct {
		ID   string `json:"ID"`
		Spec struct {
			Name string `json:"Name"`
		} `json:"Spec"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+path, nil, &configs); err != nil {
		return fmt.Errorf("failed to list docker configs: %w", err)
	}

	for _, c := range configs {
		if c.Spec.Name == name {
			d.SetID(c.ID)
			return nil
		}
	}

	return fmt.Errorf("docker config %s not found in endpoint %d", name, endpointID)
}
