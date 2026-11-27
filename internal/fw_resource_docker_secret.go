package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// portainer_docker_secret manages a Docker Swarm secret. Only its labels
// (and environment) change in place.

var newDockerSecretResource = dkNewResource(&dkResourceDef{
	typeSuffix: "_docker_secret",
	schema:     dockerSecretSchema,
	create:     dockerSecretCreate,
	read:       dockerSecretRead,
	update:     dockerSecretUpdate,
	delete:     dockerSecretDelete,
	importer:   dockerSecretImport,
	identity:   dockerSecretIdentity,
})

func dockerSecretSchema(_ context.Context) schema.Schema {
	return schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":                  schema.StringAttribute{Computed: true, Description: "Identifier of the resource.", PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()}},
			"data":                schema.StringAttribute{Optional: true, Sensitive: true, Description: "Base64-encoded secret data (stored in Terraform state).", PlanModifiers: []planmodifier.String{dkReplaceString()}, Validators: []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("data_wo")), stringvalidator.ConflictsWith(path.MatchRoot("data_wo_version"))}},
			"data_wo":             schema.StringAttribute{Optional: true, Sensitive: true, WriteOnly: true, Description: "Write-only secret data (supports ephemeral values; not stored in Terraform state).", Validators: []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("data")), stringvalidator.AlsoRequires(path.MatchRoot("data_wo_version"))}},
			"data_wo_version":     schema.Int64Attribute{Optional: true, Description: "Version flag for write-only data; must be set when using `data_wo` to trigger updates.", PlanModifiers: []planmodifier.Int64{dkReplaceInt64()}, Validators: []validator.Int64{int64validator.ConflictsWith(path.MatchRoot("data"))}},
			"driver":              schema.MapAttribute{ElementType: types.StringType, Optional: true, Description: "External secret driver configuration used to fetch the secret value at runtime. Docker cannot change it on an existing secret, so changing it replaces the secret.", PlanModifiers: []planmodifier.Map{dkReplaceMap()}},
			"endpoint_id":         schema.Int64Attribute{Required: true, Description: "ID of the Portainer environment (Docker Swarm) where the secret is created."},
			"labels":              schema.MapAttribute{ElementType: types.StringType, Optional: true, Description: "Key/value labels attached to the Docker Swarm secret."},
			"name":                schema.StringAttribute{Required: true, Description: "Name of the Docker Swarm secret. Docker cannot rename a secret, so changing it replaces the secret.", PlanModifiers: []planmodifier.String{dkReplaceString()}},
			"resource_control_id": schema.Int64Attribute{Computed: true, Description: "ID of the Portainer resource control associated with this Docker Swarm secret."},
			"templating":          schema.MapAttribute{ElementType: types.StringType, Optional: true, Description: "Templating driver configuration applied to the secret payload at runtime. Docker cannot change it on an existing secret, so changing it replaces the secret.", PlanModifiers: []planmodifier.Map{dkReplaceMap()}},
		},
	}
}

func findExistingDockerSecretByName(ctx context.Context, client *APIClient, endpointID int, name string) (string, error) {
	url := fmt.Sprintf("%s/endpoints/%d/docker/secrets", client.Endpoint, endpointID)
	var secrets []map[string]interface{}
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &secrets); err != nil {
		return "", fmt.Errorf("failed to list secrets: %w", err)
	}

	for _, s := range secrets {
		if spec, ok := s["Spec"].(map[string]interface{}); ok {
			if spec["Name"] == name {
				if id, ok := s["ID"].(string); ok {
					return id, nil
				}
			}
		}
	}
	return "", nil
}

func buildSecretPayload(d *dkBag) (map[string]interface{}, error) {
	var dataValue string
	if v, ok := d.GetOk("data_wo_version"); ok && v.(int) != 0 && d.HasChange("data_wo_version") {
		raw := d.ConfigValue("data_wo")
		if raw.IsKnown() && !raw.IsNull() {
			_ = raw.As(&dataValue)
		}
	}
	if dataValue == "" {
		if s := dkString(d, "data"); s != "" {
			dataValue = s
		}
	}

	payload := map[string]interface{}{
		"Name":   dkString(d, "name"),
		"Data":   dataValue,
		"Labels": dkMap(d, "labels"),
	}

	if v, ok := d.GetOk("driver"); ok {
		driver := v.(map[string]interface{})
		payload["Driver"] = map[string]interface{}{
			"Name":    driver["name"],
			"Options": driver,
		}
	}

	if v, ok := d.GetOk("templating"); ok {
		templating := v.(map[string]interface{})
		payload["Templating"] = map[string]interface{}{
			"Name":    templating["name"],
			"Options": templating,
		}
	}

	return payload, nil
}

type dockerSecretCreateResponse struct {
	ID        string `json:"ID"`
	Portainer struct {
		ResourceControl struct {
			ID int `json:"Id"`
		} `json:"ResourceControl"`
	} `json:"Portainer"`
}

func dockerSecretCreate(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	name := dkString(d, "name")

	if existingID, err := findExistingDockerSecretByName(ctx, client, endpointID, name); err != nil {
		return fmt.Errorf("failed to check for existing secret: %w", err)
	} else if existingID != "" {
		d.SetID(existingID)
		return dockerSecretUpdate(ctx, d, client)
	}

	payload, err := buildSecretPayload(d)
	if err != nil {
		return err
	}

	var response dockerSecretCreateResponse

	url := fmt.Sprintf("%s/endpoints/%d/docker/secrets/create", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodPost, url, payload, &response); err != nil {
		return fmt.Errorf("failed to create docker secret: %w", err)
	}

	// ID secretu
	d.SetID(response.ID)

	// ID resource controlu
	if response.Portainer.ResourceControl.ID != 0 {
		if err := d.Set("resource_control_id", response.Portainer.ResourceControl.ID); err != nil {
			return err
		}
	}

	return nil
}

func dockerSecretRead(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	id := d.ID()

	url := fmt.Sprintf("%s/endpoints/%d/docker/secrets/%s", client.Endpoint, endpointID, id)

	var result struct {
		ID   string `json:"ID"`
		Spec struct {
			Name       string                 `json:"Name"`
			Labels     map[string]string      `json:"Labels"`
			Driver     map[string]interface{} `json:"Driver"`
			Templating map[string]interface{} `json:"Templating"`
		} `json:"Spec"`
		Version struct {
			Index int `json:"Index"`
		} `json:"Version"`
		Portainer struct {
			ResourceControl struct {
				ID int `json:"Id"`
			} `json:"ResourceControl"`
		} `json:"Portainer"`
	}

	if err := doJSON(ctx, client, http.MethodGet, url, nil, &result); err != nil {
		if isAPINotFound(err) {
			d.SetID("")
			return nil
		}
		return fmt.Errorf("failed to read docker secret: %w", err)
	}

	if err := d.Set("name", result.Spec.Name); err != nil {
		return err
	}
	if err := d.Set("labels", result.Spec.Labels); err != nil {
		return err
	}
	// Create stores both driver and templating as {Name, Options: <full map>}.
	// Flatten Options (which carries the original config map) back into the
	// flat TypeMap[string]string schema so the value round-trips cleanly.
	driverMap := make(map[string]interface{})
	if dr := result.Spec.Driver; dr != nil {
		if name, ok := dr["Name"]; ok {
			driverMap["name"] = name
		}
		if opts, ok := dr["Options"].(map[string]interface{}); ok {
			for k, v := range opts {
				driverMap[k] = v
			}
		}
	}
	if err := d.Set("driver", driverMap); err != nil {
		return err
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
	if err := d.Set("templating", templ); err != nil {
		return err
	}

	if result.Portainer.ResourceControl.ID != 0 {
		if err := d.Set("resource_control_id", result.Portainer.ResourceControl.ID); err != nil {
			return err
		}
	}

	return nil
}

func dockerSecretUpdate(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	id := d.ID()

	// Docker only lets the labels of a secret change (everything else forces
	// a new one) and rejects an update without the secret's current version
	// ("invalid secret version"), so the update always failed. The current
	// spec is sent back with the configured labels.
	var current struct {
		Version struct {
			Index int `json:"Index"`
		} `json:"Version"`
		Spec map[string]interface{} `json:"Spec"`
	}
	secretURL := fmt.Sprintf("%s/endpoints/%d/docker/secrets/%s", client.Endpoint, endpointID, id)
	if err := doJSON(ctx, client, http.MethodGet, secretURL, nil, &current); err != nil {
		return fmt.Errorf("failed to read docker secret before updating it: %w", err)
	}
	spec := current.Spec
	if spec == nil {
		spec = map[string]interface{}{"Name": dkString(d, "name")}
	}
	spec["Labels"] = dkMap(d, "labels")

	url := fmt.Sprintf("%s/update?version=%d", secretURL, current.Version.Index)
	if err := doJSON(ctx, client, http.MethodPost, url, spec, nil); err != nil {
		return fmt.Errorf("failed to update docker secret: %w", err)
	}

	return dockerSecretRead(ctx, d, client)
}

func dockerSecretDelete(ctx context.Context, d *dkBag, client *APIClient) error {
	endpointID := dkInt(d, "endpoint_id")
	id := d.ID()

	url := fmt.Sprintf("%s/endpoints/%d/docker/secrets/%s", client.Endpoint, endpointID, id)
	if err := doJSON(ctx, client, http.MethodDelete, url, nil, nil); err != nil && !isAPINotFound(err) {
		return fmt.Errorf("failed to delete docker secret: %w", err)
	}

	d.SetID("")
	return nil
}

// dockerSecretImport takes "<endpoint_id>:<secret_id>", or the "-" form from
// before v3.0.0.
func dockerSecretImport(_ context.Context, d *dkBag, _ *APIClient) error {
	endpointID, secretID, err := splitEndpointImportID(d.ID(), "<endpoint_id>:<secret_id>")
	if err != nil {
		return err
	}
	if err := d.Set("endpoint_id", endpointID); err != nil {
		return err
	}
	d.SetID(secretID)
	return nil
}
