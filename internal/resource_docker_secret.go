package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/go-cty/cty"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceDockerSecret() *schema.Resource {
	return &schema.Resource{
		Create: resourceDockerSecretCreate,
		Read:   resourceDockerSecretRead,
		Delete: resourceDockerSecretDelete,
		Update: resourceDockerSecretUpdate,
		Importer: &schema.ResourceImporter{
			State: func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				importID := d.Id()
				var endpointID int
				var secretID string
				n, err := fmt.Sscanf(importID, "%d-%s", &endpointID, &secretID)
				if err != nil || n != 2 {
					return nil, fmt.Errorf("invalid import ID format. Expected '<endpoint_id>-<secret_id>'")
				}
				if err := d.Set("endpoint_id", endpointID); err != nil {
					return nil, err
				}
				d.SetId(secretID)
				return []*schema.ResourceData{d}, nil
			},
		},
		Schema: map[string]*schema.Schema{
			"endpoint_id": {Type: schema.TypeInt, Required: true},
			"name":        {Type: schema.TypeString, Required: true},
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
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"driver": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"templating": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeString},
			},
			"resource_control_id": {
				Type:     schema.TypeInt,
				Computed: true,
			},
		},
	}
}

func findExistingDockerSecretByName(client *APIClient, endpointID int, name string) (string, error) {
	path := fmt.Sprintf("/endpoints/%d/docker/secrets", endpointID)
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to list secrets: %s", string(body))
	}

	var secrets []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&secrets); err != nil {
		return "", err
	}

	for _, s := range secrets {
		if s["Spec"] != nil {
			spec := s["Spec"].(map[string]interface{})
			if spec["Name"] == name {
				if id, ok := s["ID"].(string); ok {
					return id, nil
				}
			}
		}
	}
	return "", nil
}

func buildSecretPayload(d *schema.ResourceData) map[string]interface{} {
	var dataValue string
	if v, ok := d.GetOk("data_wo_version"); ok && v.(int) != 0 && d.HasChange("data_wo_version") {
		raw, diags := d.GetRawConfigAt(cty.GetAttrPath("data_wo"))
		if diags.HasError() {
			fmt.Printf("[ERROR] Unable to read data_wo: %v\n", diags)
		} else if raw.IsKnown() && !raw.IsNull() {
			dataValue = raw.AsString()
			fmt.Printf("[DEBUG] Read write-only secret from raw config (len=%d)\n", len(dataValue))
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

	fmt.Printf("[DEBUG] Creating Docker secret %s with data length: %d\n", d.Get("name").(string), len(dataValue))
	return payload
}

type dockerSecretCreateResponse struct {
	ID        string `json:"ID"`
	Portainer struct {
		ResourceControl struct {
			Id int `json:"Id"`
		} `json:"ResourceControl"`
	} `json:"Portainer"`
}

func resourceDockerSecretCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	if existingID, err := findExistingDockerSecretByName(client, endpointID, name); err != nil {
		return fmt.Errorf("failed to check for existing secret: %w", err)
	} else if existingID != "" {
		d.SetId(existingID)
		return resourceDockerSecretUpdate(d, meta)
	}

	payload := buildSecretPayload(d)

	var response dockerSecretCreateResponse

	path := fmt.Sprintf("/endpoints/%d/docker/secrets/create", endpointID)
	resp, err := client.DoRequest(http.MethodPost, path, nil, payload)
	if err != nil {
		return fmt.Errorf("failed to create docker secret: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create docker secret: %s", string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}

	// ID secretu
	d.SetId(response.ID)

	// ID resource controlu
	if response.Portainer.ResourceControl.Id != 0 {
		_ = d.Set("resource_control_id", response.Portainer.ResourceControl.Id)
	}

	return nil
}

func resourceDockerSecretRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	id := d.Id()

	path := fmt.Sprintf("/endpoints/%d/docker/secrets/%s", endpointID, id)
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to read docker secret: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	} else if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read docker secret: %s", string(body))
	}

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
				Id int `json:"Id"`
			} `json:"ResourceControl"`
		} `json:"Portainer"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	d.Set("name", result.Spec.Name)
	d.Set("labels", result.Spec.Labels)
	d.Set("driver", result.Spec.Driver)
	d.Set("templating", result.Spec.Templating)

	if result.Portainer.ResourceControl.Id != 0 {
		_ = d.Set("resource_control_id", result.Portainer.ResourceControl.Id)
	}

	return nil
}

func resourceDockerSecretUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	id := d.Id()

	payload := buildSecretPayload(d)

	path := fmt.Sprintf("/endpoints/%d/docker/secrets/%s/update", endpointID, id)
	resp, err := client.DoRequest(http.MethodPost, path, nil, payload)
	if err != nil {
		return fmt.Errorf("failed to update docker secret: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update docker secret: %s", string(body))
	}

	return resourceDockerSecretRead(d, meta)
}

func resourceDockerSecretDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	id := d.Id()

	path := fmt.Sprintf("/endpoints/%d/docker/secrets/%s", endpointID, id)
	resp, err := client.DoRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete docker secret: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 && resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete docker secret: %s", string(body))
	}

	d.SetId("")
	return nil
}
