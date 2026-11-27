package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

var newDockerContainerGpusDataSource = dkNewDataSource(&dkDataSourceDef{
	typeSuffix: "_docker_container_gpus",
	schema:     dsDockerContainerGpusSchema,
	read:       dsDockerContainerGpusRead,
})

func dsDockerContainerGpusSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Description: "Identifier of the resource.", Computed: true},
			"container_id":   schema.StringAttribute{Required: true, Description: "Identifier of the container to inspect."},
			"environment_id": schema.Int64Attribute{Required: true, Description: "Environment (endpoint) identifier of the Docker environment the container runs in."},
			"gpus":           schema.StringAttribute{Computed: true, Description: "GPUs assigned to the container, as Portainer reports them. Empty when the container has none."},
		},
	}
}

func dsDockerContainerGpusRead(ctx context.Context, d *dkBag, client *APIClient) error {
	envID := dkInt(d, "environment_id")
	containerID := dkString(d, "container_id")

	var result struct {
		Gpus string `json:"gpus"`
	}
	url := fmt.Sprintf("%s/docker/%d/containers/%s/gpus", client.Endpoint, envID, containerID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &result); err != nil {
		return fmt.Errorf("failed to read the GPUs of container %s: %w", containerID, err)
	}

	if err := d.Set("gpus", result.Gpus); err != nil {
		return err
	}

	d.SetID(fmt.Sprintf("%d/%s/gpus", envID, containerID))
	return nil
}
