package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceKubernetesVolumes() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesVolumesCreate,
		ReadContext:   resourceKubernetesVolumesRead,
		UpdateContext: resourceKubernetesVolumesUpdate,
		DeleteContext: resourceKubernetesVolumesDelete,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer Kubernetes endpoint where the volume resource is managed.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Kubernetes namespace for namespaced volume resources (required for `persistent-volume-claim`).",
			},
			"type": {
				Type:     schema.TypeString,
				Required: true,
				ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
					v := val.(string)
					allowed := []string{"persistent-volume-claim", "persistent-volume", "volume-attachment"}
					for _, a := range allowed {
						if v == a {
							return
						}
					}
					errs = append(errs, fmt.Errorf("%q must be one of: %v", key, allowed))
					return
				},
				Description: "Type of Kubernetes volume resource: `persistent-volume-claim`, `persistent-volume`, or `volume-attachment`.",
			},
			"manifest": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "YAML or JSON manifest describing the Kubernetes volume resource to deploy.",
			},
		},
	}
}

func resourceKubernetesVolumesCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	endpointID := d.Get("endpoint_id").(int)
	namespace := d.Get("namespace").(string)
	manifest := d.Get("manifest").(string)
	volType := d.Get("type").(string)

	parsed, err := parseManifest(manifest)
	if err != nil {
		return diag.FromErr(fmt.Errorf("manifest must be valid JSON or YAML: %w", err))
	}

	metadata, ok := parsed["metadata"].(map[string]interface{})
	if !ok {
		return diag.FromErr(fmt.Errorf("missing metadata in manifest"))
	}
	name, ok := metadata["name"].(string)
	if !ok || name == "" {
		return diag.FromErr(fmt.Errorf("missing metadata.name in manifest"))
	}

	jsonBody, err := json.Marshal(parsed)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to encode manifest body: %w", err))
	}

	url, err := volumeAPIURL(client.Endpoint, endpointID, namespace, volType, false)
	if err != nil {
		return diag.FromErr(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return diag.FromErr(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return diag.FromErr(fmt.Errorf("no valid authentication method provided (api_key or jwt token)"))
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create Kubernetes volume: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to create volume (%d): %s", resp.StatusCode, string(body)))
	}

	d.SetId(fmt.Sprintf("%d:%s:%s:%s", endpointID, namespace, volType, name))
	return nil
}

func resourceKubernetesVolumesDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	endpointID, namespace, volType, name := parseVolumesID(d.Id())

	url, err := volumeAPIURL(client.Endpoint, endpointID, namespace, volType, true, name)
	if err != nil {
		return diag.FromErr(err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return diag.FromErr(err)
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return diag.FromErr(fmt.Errorf("no valid authentication method provided (api_key or jwt token)"))
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to delete volume: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to delete volume: %s", string(body)))
	}

	d.SetId("")
	return nil
}

func resourceKubernetesVolumesUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if diags := resourceKubernetesVolumesDelete(ctx, d, meta); diags.HasError() {
		return diags
	}
	return resourceKubernetesVolumesCreate(ctx, d, meta)
}

func resourceKubernetesVolumesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// Optional: implement if needed
	return nil
}

func parseVolumesID(id string) (endpointID int, namespace, volType, name string) {
	parts := strings.SplitN(id, ":", 4)
	if len(parts) != 4 {
		return 0, "", "", ""
	}
	fmt.Sscanf(parts[0], "%d", &endpointID)
	namespace = parts[1]
	volType = parts[2]
	name = parts[3]
	return
}

// volumeAPIURL builds the correct URL for the volume type
func volumeAPIURL(base string, endpointID int, namespace string, volType string, withName bool, name ...string) (string, error) {
	var path string

	switch volType {
	case "persistent-volume-claim":
		if withName {
			path = fmt.Sprintf("/endpoints/%d/kubernetes/api/v1/namespaces/%s/persistentvolumeclaims/%s", endpointID, namespace, name[0])
		} else {
			path = fmt.Sprintf("/endpoints/%d/kubernetes/api/v1/namespaces/%s/persistentvolumeclaims", endpointID, namespace)
		}
	case "persistent-volume":
		if withName {
			path = fmt.Sprintf("/endpoints/%d/kubernetes/api/v1/persistentvolumes/%s", endpointID, name[0])
		} else {
			path = fmt.Sprintf("/endpoints/%d/kubernetes/api/v1/persistentvolumes", endpointID)
		}
	case "volume-attachment":
		if withName {
			path = fmt.Sprintf("/endpoints/%d/kubernetes/apis/storage.k8s.io/v1/volumeattachments/%s", endpointID, name[0])
		} else {
			path = fmt.Sprintf("/endpoints/%d/kubernetes/apis/storage.k8s.io/v1/volumeattachments", endpointID)
		}
	default:
		return "", fmt.Errorf("unsupported volume type: %s", volType)
	}

	return base + path, nil
}
