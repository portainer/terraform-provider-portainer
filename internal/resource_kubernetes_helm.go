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

func resourceKubernetesHelm() *schema.Resource {
	return &schema.Resource{
		Create: resourceKubernetesHelmCreate,
		Read:   resourceKubernetesHelmRead,
		Delete: resourceKubernetesHelmDelete,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"chart": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"namespace": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"repo": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"values": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "",
				ForceNew: true,
			},
		},
	}
}

func resourceKubernetesHelmCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Get("environment_id").(int)

	body := map[string]interface{}{
		"chart":     d.Get("chart").(string),
		"name":      d.Get("name").(string),
		"namespace": d.Get("namespace").(string),
		"repo":      d.Get("repo").(string),
		"values":    d.Get("values").(string),
	}

	jsonBody, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/endpoints/%d/kubernetes/helm", client.Endpoint, id)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", client.APIKey)
	req.Header.Set("Content-Type", "application/json")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to install helm chart: %s", string(data))
	}

	d.SetId(fmt.Sprintf("%d:%s:%s", id, d.Get("namespace").(string), d.Get("name").(string)))
	return resourceKubernetesHelmRead(d, meta)
}

func resourceKubernetesHelmRead(d *schema.ResourceData, meta interface{}) error {
	// No-op for now
	return nil
}

func resourceKubernetesHelmDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	idParts := strings.SplitN(d.Id(), ":", 3)
	if len(idParts) != 3 {
		return fmt.Errorf("invalid ID format, expected 'envID:namespace:release': %s", d.Id())
	}

	envID := idParts[0]
	namespace := idParts[1]
	release := idParts[2]

	url := fmt.Sprintf("%s/endpoints/%s/kubernetes/helm/%s?namespace=%s", client.Endpoint, envID, release, namespace)

	req, err := http.NewRequest("DELETE", url, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-API-Key", client.APIKey)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete helm release: %s", string(data))
	}

	d.SetId("")
	return nil
}
