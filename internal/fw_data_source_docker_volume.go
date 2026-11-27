package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

var newDockerVolumeDataSource = dkNewDataSource(&dkDataSourceDef{
	typeSuffix: "_docker_volume",
	schema:     dsDockerVolumeSchema,
	read:       dsDockerVolumeRead,
})

func dsDockerVolumeSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Description: "Identifier of the resource.", Computed: true},
			"driver":      schema.StringAttribute{Computed: true, Description: "Driver used by the Docker volume (e.g., local, nfs, btrfs)."},
			"endpoint_id": schema.Int64Attribute{Required: true, Description: "ID of the Portainer environment (Docker host or Swarm) where the volume is located."},
			"mount_point": schema.StringAttribute{Computed: true, Description: "Filesystem path on the Docker host where the volume is mounted."},
			"name":        schema.StringAttribute{Required: true, Description: "Name of the Docker volume to look up."},
		},
	}
}

func dsDockerVolumeRead(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	name := dkString(d, "name")

	path := fmt.Sprintf("/endpoints/%d/docker/volumes", endpointID)
	var result struct {
		Volumes []struct {
			Name       string `json:"Name"`
			Driver     string `json:"Driver"`
			Mountpoint string `json:"Mountpoint"`
		} `json:"Volumes"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+path, nil, &result); err != nil {
		return fmt.Errorf("failed to list docker volumes: %w", err)
	}

	for _, v := range result.Volumes {
		if v.Name == name {
			d.SetID(v.Name) // For volumes, the name is the ID in the API
			if err := d.Set("driver", v.Driver); err != nil {
				return err
			}
			if err := d.Set("mount_point", v.Mountpoint); err != nil {
				return err
			}
			return nil
		}
	}

	return fmt.Errorf("docker volume %s not found in endpoint %d", name, endpointID)
}
