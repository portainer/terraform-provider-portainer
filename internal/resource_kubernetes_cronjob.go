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

func resourceKubernetesCronJob() *schema.Resource {
	return &schema.Resource{
		Create: resourceKubernetesCronJobCreate,
		Read:   resourceKubernetesCronJobRead,
		Update: resourceKubernetesCronJobUpdate,
		Delete: resourceKubernetesCronJobDelete,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"namespace": {
				Type:     schema.TypeString,
				Required: true,
			},
			"manifest": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func resourceKubernetesCronJobCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	endpointID := d.Get("endpoint_id").(int)
	namespace := d.Get("namespace").(string)
	manifest := d.Get("manifest").(string)

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

	url := fmt.Sprintf("%s/endpoints/%d/kubernetes/apis/batch/v1/namespaces/%s/cronjobs", client.Endpoint, endpointID, namespace)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-API-Key", client.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create Kubernetes CronJob: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create CronJob (%d): %s", resp.StatusCode, string(body))
	}

	d.SetId(fmt.Sprintf("%d:%s:%s", endpointID, namespace, name))
	return nil
}

func resourceKubernetesCronJobDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	endpointID, namespace, name := parseCronJobID(d.Id())

	url := fmt.Sprintf("%s/endpoints/%d/kubernetes/apis/batch/v1/namespaces/%s/cronjobs/%s", client.Endpoint, endpointID, namespace, name)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", client.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to delete CronJob: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 && resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete CronJob: %s", string(body))
	}

	d.SetId("")
	return nil
}

func resourceKubernetesCronJobUpdate(d *schema.ResourceData, meta interface{}) error {
	if err := resourceKubernetesCronJobDelete(d, meta); err != nil {
		return fmt.Errorf("delete during update failed: %w", err)
	}
	return resourceKubernetesCronJobCreate(d, meta)
}

func resourceKubernetesCronJobRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func parseCronJobID(id string) (endpointID int, namespace string, name string) {
	parts := strings.SplitN(id, ":", 3)
	if len(parts) != 3 {
		return 0, "", ""
	}
	fmt.Sscanf(parts[0], "%d", &endpointID)
	namespace = parts[1]
	name = parts[2]
	return
}
