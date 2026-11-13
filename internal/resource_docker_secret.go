package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/go-cty/cty"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceDockerSecret() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceDockerSecretCreate,
		ReadContext:   resourceDockerSecretRead,
		DeleteContext: resourceDockerSecretDelete,
		UpdateContext: resourceDockerSecretUpdate,
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				endpointID, secretID, err := splitEndpointImportID(d.Id(), "<endpoint_id>:<secret_id>")
				if err != nil {
					return nil, err
				}
				if err := d.Set("endpoint_id", endpointID); err != nil {
					return nil, err
				}
				d.SetId(secretID)
				return []*schema.ResourceData{d}, nil
			},
		},
		Schema: map[string]*schema.Schema{
			"endpoint_id": {Type: schema.TypeInt, Required: true, Description: "ID of the Portainer environment (Docker Swarm) where the secret is created."},
			"name":        {Type: schema.TypeString, Required: true, ForceNew: true, Description: "Name of the Docker Swarm secret. Docker cannot rename a secret, so changing it replaces the secret."},
			"data": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				Sensitive:     true,
				ConflictsWith: []string{"data_wo", "data_wo_version"},
				Description:   "Base64-encoded secret data (stored in Terraform state).",
			},
			"data_wo": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				WriteOnly:     true,
				ConflictsWith: []string{"data"},
				RequiredWith:  []string{"data_wo_version"},
				Description:   "Write-only secret data (supports ephemeral values; not stored in Terraform state).",
			},
			"data_wo_version": {
				Type:          schema.TypeInt,
				Optional:      true,
				ForceNew:      true,
				Description:   "Version flag for write-only data; must be set when using `data_wo` to trigger updates.",
				ConflictsWith: []string{"data"},
			},
			"labels": {
				Type:        schema.TypeMap,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Key/value labels attached to the Docker Swarm secret.",
			},
			"driver": {
				Type:        schema.TypeMap,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				ForceNew:    true,
				Description: "External secret driver configuration used to fetch the secret value at runtime. Docker cannot change it on an existing secret, so changing it replaces the secret.",
			},
			"templating": {
				Type:        schema.TypeMap,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				ForceNew:    true,
				Description: "Templating driver configuration applied to the secret payload at runtime. Docker cannot change it on an existing secret, so changing it replaces the secret.",
			},
			"resource_control_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "ID of the Portainer resource control associated with this Docker Swarm secret.",
			},
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

func buildSecretPayload(d *schema.ResourceData) (map[string]interface{}, error) {
	var dataValue string
	if v, ok := d.GetOk("data_wo_version"); ok && v.(int) != 0 && d.HasChange("data_wo_version") {
		raw, diags := d.GetRawConfigAt(cty.GetAttrPath("data_wo"))
		if diags.HasError() {
			// Going on would store the secret without its data.
			return nil, fmt.Errorf("unable to read data_wo: %v", diags)
		}
		if raw.IsKnown() && !raw.IsNull() {
			dataValue = raw.AsString()
		}
	}
	if dataValue == "" {
		if v := d.Get("data"); v != nil {
			if s, ok := v.(string); ok && s != "" {
				dataValue = s
			}
		}
	}

	payload := map[string]interface{}{
		"Name":   d.Get("name").(string),
		"Data":   dataValue,
		"Labels": d.Get("labels").(map[string]interface{}),
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

func resourceDockerSecretCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	if existingID, err := findExistingDockerSecretByName(ctx, client, endpointID, name); err != nil {
		return diag.FromErr(fmt.Errorf("failed to check for existing secret: %w", err))
	} else if existingID != "" {
		d.SetId(existingID)
		return resourceDockerSecretUpdate(ctx, d, meta)
	}

	payload, err := buildSecretPayload(d)
	if err != nil {
		return diag.FromErr(err)
	}

	var response dockerSecretCreateResponse

	url := fmt.Sprintf("%s/endpoints/%d/docker/secrets/create", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodPost, url, payload, &response); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create docker secret: %w", err))
	}

	// ID secretu
	d.SetId(response.ID)

	// ID resource controlu
	if response.Portainer.ResourceControl.ID != 0 {
		if err := d.Set("resource_control_id", response.Portainer.ResourceControl.ID); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourceDockerSecretRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	id := d.Id()

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
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read docker secret: %w", err))
	}

	if err := d.Set("name", result.Spec.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("labels", result.Spec.Labels); err != nil {
		return diag.FromErr(err)
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
		return diag.FromErr(err)
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
		return diag.FromErr(err)
	}

	if result.Portainer.ResourceControl.ID != 0 {
		if err := d.Set("resource_control_id", result.Portainer.ResourceControl.ID); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourceDockerSecretUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	id := d.Id()

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
		return diag.FromErr(fmt.Errorf("failed to read docker secret before updating it: %w", err))
	}
	spec := current.Spec
	if spec == nil {
		spec = map[string]interface{}{"Name": d.Get("name").(string)}
	}
	spec["Labels"] = d.Get("labels").(map[string]interface{})

	url := fmt.Sprintf("%s/update?version=%d", secretURL, current.Version.Index)
	if err := doJSON(ctx, client, http.MethodPost, url, spec, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update docker secret: %w", err))
	}

	return resourceDockerSecretRead(ctx, d, meta)
}

func resourceDockerSecretDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	id := d.Id()

	url := fmt.Sprintf("%s/endpoints/%d/docker/secrets/%s", client.Endpoint, endpointID, id)
	if err := doJSON(ctx, client, http.MethodDelete, url, nil, nil); err != nil && !isAPINotFound(err) {
		return diag.FromErr(fmt.Errorf("failed to delete docker secret: %w", err))
	}

	d.SetId("")
	return nil
}
