package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// portainer_docker_config manages a Docker Swarm config.

var newDockerConfigResource = dkNewResource(&dkResourceDef{
	typeSuffix: "_docker_config",
	schema:     dockerConfigSchema,
	create:     dockerConfigCreate,
	read:       dockerConfigRead,
	update:     dockerConfigUpdate,
	delete:     dockerConfigDelete,
	importer:   dockerConfigImport,
	identity:   dockerConfigIdentity,
})

func dockerConfigSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":                  schema.StringAttribute{Computed: true, Description: "Identifier of the resource.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"data":                schema.StringAttribute{Required: true, Sensitive: true, Description: "Base64-encoded config payload stored in the Docker Swarm config.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"endpoint_id":         schema.Int64Attribute{Required: true, Description: "ID of the Portainer environment (Docker Swarm) where the config is created.", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}},
			"labels":              schema.MapAttribute{ElementType: types.StringType, Optional: true, Description: "Key/value labels attached to the Docker Swarm config.", PlanModifiers: []planmodifier.Map{dkReplaceMap()}},
			"name":                schema.StringAttribute{Required: true, Description: "Name of the Docker Swarm config."},
			"resource_control_id": schema.Int64Attribute{Computed: true, Description: "ID of the Portainer resource control associated with this Docker Swarm config."},
			"templating":          schema.MapAttribute{ElementType: types.StringType, Optional: true, Description: "Templating driver configuration applied to the config payload at runtime.", PlanModifiers: []planmodifier.Map{dkReplaceMap()}},
		},
	}
}

func findExistingDockerConfigByName(ctx context.Context, client *APIClient, endpointID int, name string) (string, error) {
	var configs []map[string]interface{}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/endpoints/%d/docker/configs", client.Endpoint, endpointID), nil, &configs); err != nil {
		return "", fmt.Errorf("failed to list docker configs: %w", err)
	}

	for _, cfg := range configs {
		if spec, ok := cfg["Spec"].(map[string]interface{}); ok {
			if spec["Name"] == name {
				if id, ok := cfg["ID"].(string); ok {
					return id, nil
				}
			}
		}
	}

	return "", nil
}

type dockerConfigCreateResponse struct {
	ID        string `json:"ID"`
	Portainer struct {
		ResourceControl struct {
			ID int `json:"Id"`
		} `json:"ResourceControl"`
	} `json:"Portainer"`
}

func dockerConfigCreate(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	name := dkString(d, "name")

	if existingID, err := findExistingDockerConfigByName(ctx, client, endpointID, name); err != nil {
		return fmt.Errorf("failed to check for existing docker config: %w", err)
	} else if existingID != "" {
		d.SetID(existingID)
		return dockerConfigUpdate(ctx, d, client)
	}

	payload := map[string]interface{}{
		"Name":   name,
		"Data":   dkString(d, "data"),
		"Labels": dkMap(d, "labels"),
	}

	if v, ok := d.GetOk("templating"); ok {
		templating := v.(map[string]interface{})
		payload["Templating"] = map[string]interface{}{
			"Name":    templating["name"],
			"Options": templating,
		}
	}

	var response dockerConfigCreateResponse

	if err := doJSON(ctx, client, http.MethodPost, fmt.Sprintf("%s/endpoints/%d/docker/configs/create", client.Endpoint, endpointID), payload, &response); err != nil {
		return fmt.Errorf("failed to create docker config: %w", err)
	}

	d.SetID(response.ID)

	if response.Portainer.ResourceControl.ID != 0 {
		if err := d.Set("resource_control_id", response.Portainer.ResourceControl.ID); err != nil {
			return err
		}
	}

	return nil
}

func dockerConfigRead(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	id := d.ID()

	var result struct {
		ID   string `json:"ID"`
		Spec struct {
			Name       string                 `json:"Name"`
			Labels     map[string]string      `json:"Labels"`
			Templating map[string]interface{} `json:"Templating"`
		} `json:"Spec"`
		Portainer struct {
			ResourceControl struct {
				ID int `json:"Id"`
			} `json:"ResourceControl"`
		} `json:"Portainer"`
	}

	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/endpoints/%d/docker/configs/%s", client.Endpoint, endpointID, id), nil, &result); err != nil {
		if isAPINotFound(err) {
			d.SetID("")
			return nil
		}
		return fmt.Errorf("failed to read docker config: %w", err)
	}

	templ := make(map[string]interface{})
	if t := result.Spec.Templating; t != nil {
		if name, ok := t["Name"]; ok {
			templ["name"] = name
		}
		if opts, ok := t["Options"].(map[string]interface{}); ok {
			for k, v := range opts {
				templ[k] = v
			}
		}
	}

	fields := map[string]interface{}{
		"name":       result.Spec.Name,
		"labels":     result.Spec.Labels,
		"templating": templ,
	}
	if result.Portainer.ResourceControl.ID != 0 {
		fields["resource_control_id"] = result.Portainer.ResourceControl.ID
	}
	if err := d.SetFields(fields); err != nil {
		return err
	}

	return nil
}

func dockerConfigDelete(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	id := d.ID()

	if err := doJSON(ctx, client, http.MethodDelete, fmt.Sprintf("%s/endpoints/%d/docker/configs/%s", client.Endpoint, endpointID, id), nil, nil); err != nil {
		if !isAPINotFound(err) {
			return fmt.Errorf("failed to delete docker config: %w", err)
		}
	}

	d.SetID("")
	return nil
}

func dockerConfigUpdate(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	id := d.ID()

	payload := map[string]interface{}{
		"Name":   dkString(d, "name"),
		"Data":   dkString(d, "data"),
		"Labels": dkMap(d, "labels"),
	}

	if v, ok := d.GetOk("templating"); ok {
		templating := v.(map[string]interface{})
		payload["Templating"] = map[string]interface{}{
			"Name":    templating["name"],
			"Options": templating,
		}
	}

	if err := doJSON(ctx, client, http.MethodPost, fmt.Sprintf("%s/endpoints/%d/docker/configs/%s/update", client.Endpoint, endpointID, id), payload, nil); err != nil {
		return fmt.Errorf("failed to update docker config: %w", err)
	}

	return dockerConfigRead(ctx, d, client)
}

// dockerConfigImport takes "<endpoint_id>:<config_id>", or the "-" form from
// before v3.0.0.
func dockerConfigImport(_ context.Context, d *dkBag, _ *APIClient) error {
	endpointID, configID, err := splitEndpointImportID(d.ID(), "<endpoint_id>:<config_id>")
	if err != nil {
		return err
	}
	if err := d.Set("endpoint_id", endpointID); err != nil {
		return err
	}
	d.SetID(configID)
	return nil
}
