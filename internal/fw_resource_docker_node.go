package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// portainer_docker_node manages the spec (availability, role, name, labels)
// of an existing Docker Swarm node. Create and Update send the same update.

var newDockerNodeResource = dkNewResource(&dkResourceDef{
	typeSuffix: "_docker_node",
	schema:     dockerNodeSchema,
	create:     dockerNodeUpdate,
	read:       dockerNodeRead,
	update:     dockerNodeUpdate,
	delete:     dockerNodeDelete,
	importer:   dockerNodeImport,
	identity:   dockerNodeIdentity,
})

func dockerNodeSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":           schema.StringAttribute{Computed: true, Description: "Identifier of the resource.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"availability": schema.StringAttribute{Optional: true, Description: "Desired availability of the Swarm node (active, pause, or drain)."},
			"endpoint_id":  schema.Int64Attribute{Required: true, Description: "ID of the Portainer environment (Docker Swarm cluster) where the node is managed.", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}},
			"labels":       schema.MapAttribute{ElementType: types.StringType, Optional: true, Description: "Key/value labels assigned to the Swarm node for placement and selection."},
			"name":         schema.StringAttribute{Optional: true, Description: "Desired display name for the Swarm node."},
			"node_id":      schema.StringAttribute{Required: true, Description: "Docker Swarm node ID to update.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"role":         schema.StringAttribute{Optional: true, Description: "Desired role of the Swarm node (worker or manager)."},
			"version":      schema.Int64Attribute{Required: true, Description: "Swarm node version required for update operation"},
		},
	}
}

type DockerNodeUpdatePayload struct {
	Availability string            `json:"Availability,omitempty"`
	Name         string            `json:"Name,omitempty"`
	Role         string            `json:"Role,omitempty"`
	Labels       map[string]string `json:"Labels,omitempty"`
}

func dockerNodeUpdate(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	nodeID := dkString(d, "node_id")
	version := dkInt(d, "version")

	payload := DockerNodeUpdatePayload{
		Availability: dkString(d, "availability"),
		Name:         dkString(d, "name"),
		Role:         dkString(d, "role"),
		Labels:       convertMapsString(dkMap(d, "labels")),
	}

	reqURL := fmt.Sprintf("%s/endpoints/%d/docker/nodes/%s/update?version=%d", client.Endpoint, endpointID, url.PathEscape(nodeID), version)
	if err := doJSON(ctx, client, http.MethodPost, reqURL, payload, nil); err != nil {
		return fmt.Errorf("failed to update node: %w", err)
	}

	d.SetID(fmt.Sprintf("%d:%s", endpointID, nodeID))
	return nil
}

func dockerNodeRead(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	nodeID := dkString(d, "node_id")

	reqURL := fmt.Sprintf("%s/endpoints/%d/docker/nodes/%s", client.Endpoint, endpointID, nodeID)

	var result struct {
		ID      string `json:"ID"`
		Version struct {
			Index int `json:"Index"`
		} `json:"Version"`
		Spec struct {
			Availability string            `json:"Availability"`
			Name         string            `json:"Name"`
			Role         string            `json:"Role"`
			Labels       map[string]string `json:"Labels"`
		} `json:"Spec"`
	}

	if err := doJSON(ctx, client, http.MethodGet, reqURL, nil, &result); err != nil {
		if isAPINotFound(err) {
			d.SetID("")
			return nil
		}
		return fmt.Errorf("failed to read node: %w", err)
	}

	if err := d.Set("version", result.Version.Index); err != nil {
		return err
	}
	if err := d.Set("name", result.Spec.Name); err != nil {
		return err
	}
	if err := d.Set("availability", result.Spec.Availability); err != nil {
		return err
	}
	if err := d.Set("role", result.Spec.Role); err != nil {
		return err
	}
	if err := d.Set("labels", result.Spec.Labels); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("%d:%s", endpointID, nodeID))
	return nil
}

func dockerNodeDelete(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	nodeID := dkString(d, "node_id")

	reqURL := fmt.Sprintf("%s/endpoints/%d/docker/nodes/%s", client.Endpoint, endpointID, url.PathEscape(nodeID))
	if err := doJSON(ctx, client, http.MethodDelete, reqURL, nil, nil); err != nil {
		return fmt.Errorf("failed to delete node: %w", err)
	}

	d.SetID("")
	return nil
}

func convertMapsString(input map[string]interface{}) map[string]string {
	out := make(map[string]string)
	for k, v := range input {
		out[k] = fmt.Sprintf("%v", v)
	}
	return out
}

// dockerNodeImport takes "<endpoint_id>:<node_id>", or "<endpoint_id>-<node_id>"
// from before v3.0.0, and stores the ID in the current format.
func dockerNodeImport(_ context.Context, d *dkBag, _ *APIClient) error {
	const format = "<endpoint_id>:<node_id>"
	parts, err := importIDPartsCompat(d.ID(), "-", 2, format)
	if err != nil {
		return err
	}
	endpointID, err := strconv.Atoi(parts[0])
	if err != nil {
		return fmt.Errorf("invalid import ID %q: expected %s", d.ID(), format)
	}
	if err := d.Set("endpoint_id", endpointID); err != nil {
		return err
	}
	if err := d.Set("node_id", parts[1]); err != nil {
		return err
	}
	d.SetID(strings.Join(parts, ":"))
	return nil
}
