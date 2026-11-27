package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

var newDockerSecretDataSource = dkNewDataSource(&dkDataSourceDef{
	typeSuffix: "_docker_secret",
	schema:     dsDockerSecretSchema,
	read:       dsDockerSecretRead,
})

func dsDockerSecretSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Description: "Identifier of the resource.", Computed: true},
			"endpoint_id": schema.Int64Attribute{Required: true, Description: "Identifier of the Portainer endpoint hosting the Docker secret."},
			"name":        schema.StringAttribute{Required: true, Description: "Name of the Docker secret to look up."},
		},
	}
}

func dsDockerSecretRead(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	name := dkString(d, "name")

	path := fmt.Sprintf("/endpoints/%d/docker/secrets", endpointID)
	var secrets []struct {
		ID   string `json:"ID"`
		Spec struct {
			Name string `json:"Name"`
		} `json:"Spec"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+path, nil, &secrets); err != nil {
		return fmt.Errorf("failed to list docker secrets: %w", err)
	}

	for _, s := range secrets {
		if s.Spec.Name == name {
			d.SetID(s.ID)
			return nil
		}
	}

	return fmt.Errorf("docker secret %s not found in endpoint %d", name, endpointID)
}
