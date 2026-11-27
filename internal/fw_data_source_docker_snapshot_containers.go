package internal

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var newDockerSnapshotContainersDataSource = dkNewDataSource(&dkDataSourceDef{
	typeSuffix: "_docker_snapshot_containers",
	schema:     dsDockerSnapshotContainersSchema,
	read:       dsDockerSnapshotContainersRead,
})

func dsDockerSnapshotContainersSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":            schema.StringAttribute{Description: "Identifier of the resource.", Computed: true},
			"containers":    schema.ListAttribute{ElementType: types.ObjectType{AttrTypes: map[string]attr.Type{"command": types.StringType, "created": types.Int64Type, "id": types.StringType, "image": types.StringType, "image_id": types.StringType, "labels": types.MapType{ElemType: types.StringType}, "names": types.ListType{ElemType: types.StringType}, "ports": types.ListType{ElemType: types.ObjectType{AttrTypes: map[string]attr.Type{"ip": types.StringType, "private_port": types.Int64Type, "public_port": types.Int64Type, "type": types.StringType}}}, "state": types.StringType, "status": types.StringType}}, Computed: true, Description: "The containers in the latest snapshot. This is what Portainer last saw, not a live query, so it can lag behind the environment."},
			"edge_stack_id": schema.Int64Attribute{Optional: true, Description: "Only list the containers belonging to this edge stack."},
			"endpoint_id":   schema.Int64Attribute{Required: true, Description: "Identifier of the Docker environment whose snapshot is read."},
		},
	}
}

func dsDockerSnapshotContainersRead(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")

	listURL := fmt.Sprintf("%s/docker/%d/snapshot/containers", client.Endpoint, endpointID)
	if v, ok := d.GetOk("edge_stack_id"); ok && v.(int) != 0 {
		listURL += fmt.Sprintf("?edgeStackId=%d", v.(int))
	}

	body, err := apiGETRaw(ctx, client, listURL)
	if err != nil {
		return fmt.Errorf("failed to read the snapshot containers of environment %d: %w", endpointID, err)
	}

	// Portainer's specification types this response as a single container even
	// though the endpoint lists them, so both shapes are accepted rather than
	// trusting the document over the endpoint's name.
	var containers []dockerSnapshotContainer
	if err := json.Unmarshal(body, &containers); err != nil {
		var single dockerSnapshotContainer
		if err := json.Unmarshal(body, &single); err != nil {
			return fmt.Errorf("failed to parse the snapshot containers of environment %d: %w", endpointID, err)
		}
		containers = []dockerSnapshotContainer{single}
	}

	entries := make([]interface{}, 0, len(containers))
	for _, container := range containers {
		entries = append(entries, container.fields())
	}

	if err := d.SetFields(map[string]interface{}{"containers": entries}); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("portainer-docker-snapshot-containers-%d", endpointID))
	return nil
}

// dockerSnapshotContainer is the container shape both snapshot endpoints
// return. Only the fields worth acting on are decoded; the rest stays in the
// raw response each data source also exposes.
type dockerSnapshotContainer struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	ImageID string            `json:"ImageID"`
	Command string            `json:"Command"`
	Created int64             `json:"Created"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Labels  map[string]string `json:"Labels"`
	Ports   []struct {
		IP          string `json:"IP"`
		PrivatePort int    `json:"PrivatePort"`
		PublicPort  int    `json:"PublicPort"`
		Type        string `json:"Type"`
	} `json:"Ports"`
}

func (c dockerSnapshotContainer) fields() map[string]interface{} {
	ports := make([]interface{}, 0, len(c.Ports))
	for _, port := range c.Ports {
		ports = append(ports, map[string]interface{}{
			"ip": port.IP, "private_port": port.PrivatePort,
			"public_port": port.PublicPort, "type": port.Type,
		})
	}
	return map[string]interface{}{
		"id": c.ID, "names": c.Names, "image": c.Image, "image_id": c.ImageID,
		"command": c.Command, "created": int(c.Created), "state": c.State,
		"status": c.Status, "labels": c.Labels, "ports": ports,
	}
}
