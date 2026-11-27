package internal

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

var newDockerImageDataSource = dkNewDataSource(&dkDataSourceDef{
	typeSuffix: "_docker_image",
	schema:     dsDockerImageSchema,
	read:       dsDockerImageRead,
})

func dsDockerImageSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Description: "Identifier of the resource.", Computed: true},
			"endpoint_id": schema.Int64Attribute{Required: true, Description: "Identifier of the Portainer endpoint where the Docker image resides."},
			"name":        schema.StringAttribute{Required: true, Description: "The name of the image (e.g. nginx:latest or nginx)"},
		},
	}
}

func dsDockerImageRead(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	name := dkString(d, "name")

	// Ensure name has a tag if it doesn't
	searchName := name
	if !strings.Contains(name, ":") {
		searchName = name + ":latest"
	}

	path := fmt.Sprintf("/endpoints/%d/docker/images/json", endpointID)
	var images []struct {
		ID       string   `json:"Id"`
		RepoTags []string `json:"RepoTags"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+path, nil, &images); err != nil {
		return fmt.Errorf("failed to list docker images: %w", err)
	}

	for _, img := range images {
		for _, tag := range img.RepoTags {
			if tag == searchName || tag == name {
				d.SetID(img.ID)
				return nil
			}
		}
	}

	return fmt.Errorf("docker image %s not found in endpoint %d", name, endpointID)
}
