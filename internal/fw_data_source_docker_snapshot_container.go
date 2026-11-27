package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var newDockerSnapshotContainerDataSource = dkNewDataSource(&dkDataSourceDef{
	typeSuffix: "_docker_snapshot_container",
	schema:     dsDockerSnapshotContainerSchema,
	read:       dsDockerSnapshotContainerRead,
})

func dsDockerSnapshotContainerSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Description: "Identifier of the resource.", Computed: true},
			"command":      schema.StringAttribute{Computed: true, Description: "Command the container runs."},
			"container_id": schema.StringAttribute{Required: true, Description: "Identifier of the container to read from the snapshot."},
			"created":      schema.Int64Attribute{Computed: true, Description: "Unix timestamp the container was created at."},
			"details":      schema.StringAttribute{Computed: true, Description: "The whole snapshot entry as Portainer returns it, encoded as JSON. Decode it with `jsondecode()` for the mounts, networks and host configuration the attributes above do not cover."},
			"endpoint_id":  schema.Int64Attribute{Required: true, Description: "Identifier of the Docker environment whose snapshot is read."},
			"image":        schema.StringAttribute{Computed: true, Description: "Image the container runs."},
			"image_id":     schema.StringAttribute{Computed: true, Description: "Identifier of that image."},
			"labels":       schema.MapAttribute{ElementType: types.StringType, Computed: true, Description: "Labels on the container."},
			"names":        schema.ListAttribute{ElementType: types.StringType, Computed: true, Description: "Names of the container, as Docker reports them with a leading slash."},
			"state":        schema.StringAttribute{Computed: true, Description: "State of the container, for example `running` or `exited`."},
			"status":       schema.StringAttribute{Computed: true, Description: "Human-readable status, for example `Up 3 hours`."},
		},
	}
}

func dsDockerSnapshotContainerRead(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	containerID := dkString(d, "container_id")

	containerURL := fmt.Sprintf("%s/docker/%d/snapshot/containers/%s",
		client.Endpoint, endpointID, url.PathEscape(containerID))
	body, err := apiGETRaw(ctx, client, containerURL)
	if err != nil {
		return fmt.Errorf("failed to read container %s from the snapshot of environment %d: %w", containerID, endpointID, err)
	}

	var container dockerSnapshotContainer
	if err := json.Unmarshal(body, &container); err != nil {
		return fmt.Errorf("failed to parse container %s from the snapshot: %w", containerID, err)
	}

	fields := container.fields()
	// id and ports are not top-level attributes here: the identifier is an
	// argument, and the port list belongs to the listing data source.
	delete(fields, "id")
	delete(fields, "ports")
	fields["details"] = string(body)

	if err := d.SetFields(fields); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("portainer-docker-snapshot-container-%d-%s", endpointID, containerID))
	return nil
}
