package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceKubernetesVolumes() *schema.Resource {
	return &schema.Resource{
		Create: resourceKubernetesVolumesCreate,
		Read:   resourceKubernetesVolumesRead,
		Update: resourceKubernetesVolumesUpdate,
		Delete: resourceKubernetesVolumesDelete,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"namespace": {
				Type:     schema.TypeString,
				Optional: true,
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
			},
			"manifest": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func resourceKubernetesVolumesCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	endpointID := d.Get("endpoint_id").(int)
	namespace := d.Get("namespace").(string)
	manifest := d.Get("manifest").(string)
	volType := d.Get("type").(string)

	parsed, err := parseManifest(manifest)
	if err != nil {
		return fmt.Errorf("manifest must be valid JSON or YAML: %w", err)
	}

	metadata, ok := parsed["metadata"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("missing metadata in manifest")
	}
	name, ok := metadata["name"].(string)
	if !ok || name == "" {
		return fmt.Errorf("missing metadata.name in manifest")
	}

	jsonBody, err := json.Marshal(parsed)
	if err != nil {
		return fmt.Errorf("failed to encode manifest body: %w", err)
	}

	url, err := volumeAPIURL(client.Endpoint, endpointID, namespace, volType, false)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", client.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes volume: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create volume (%d): %s", resp.StatusCode, string(body))
	}

	d.SetId(fmt.Sprintf("%d:%s:%s:%s", endpointID, namespace, volType, name))
	return nil
}

func resourceKubernetesVolumesDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	endpointID, namespace, volType, name := parseVolumesID(d.Id())

	url, err := volumeAPIURL(client.Endpoint, endpointID, namespace, volType, true, name)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", client.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete volume: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 && resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete volume: %s", string(body))
	}

	d.SetId("")
	return nil
}

func resourceKubernetesVolumesUpdate(d *schema.ResourceData, meta interface{}) error {
	if err := resourceKubernetesVolumesDelete(d, meta); err != nil {
		return fmt.Errorf("delete during update failed: %w", err)
	}
	return resourceKubernetesVolumesCreate(d, meta)
}

func resourceKubernetesVolumesRead(d *schema.ResourceData, meta interface{}) error {
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
