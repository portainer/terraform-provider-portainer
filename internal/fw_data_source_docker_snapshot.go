package internal

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

var newDockerSnapshotDataSource = dkNewDataSource(&dkDataSourceDef{
	typeSuffix: "_docker_snapshot",
	schema:     dsDockerSnapshotSchema,
	read:       dsDockerSnapshotRead,
})

func dsDockerSnapshotSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":          schema.StringAttribute{Description: "Identifier of the resource.", Computed: true},
			"endpoint_id": schema.Int64Attribute{Required: true, Description: "Identifier of the Docker environment whose snapshot is read."},
			"snapshot":    schema.StringAttribute{Computed: true, Description: "The latest snapshot as Portainer returns it, encoded as JSON. Decode it with `jsondecode()`. Use `portainer_docker_snapshot_containers` for a typed view of the containers in it."},
		},
	}
}

func dsDockerSnapshotRead(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")

	snapshotURL := fmt.Sprintf("%s/docker/%d/snapshot", client.Endpoint, endpointID)
	body, err := apiGETRaw(ctx, client, snapshotURL)
	if err != nil {
		return fmt.Errorf("failed to read the snapshot of environment %d: %w", endpointID, err)
	}

	if err := d.SetFields(map[string]interface{}{"snapshot": string(body)}); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("portainer-docker-snapshot-%d", endpointID))
	return nil
}
