package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceKubernetesNamespaceSystem() *schema.Resource {
	return &schema.Resource{
		Create: resourceKubernetesNamespaceSystemToggle,
		Read:   resourceKubernetesNamespaceSystemRead,
		Update: resourceKubernetesNamespaceSystemToggle,
		Delete: resourceKubernetesNamespaceSystemUnset,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"namespace": {
				Type:     schema.TypeString,
				Required: true,
			},
			"system": {
				Type:     schema.TypeBool,
				Required: true,
			},
		},
	}
}

func resourceKubernetesNamespaceSystemToggle(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)
	system := d.Get("system").(bool)

	body := map[string]interface{}{
		"system": system,
	}

	jsonBody, _ := json.Marshal(body)
	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/system", client.Endpoint, id, namespace)

	req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonBody))
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
		return fmt.Errorf("failed to toggle namespace system state: %s", string(data))
	}

	d.SetId(fmt.Sprintf("%d:%s", id, namespace))
	return nil
}

func resourceKubernetesNamespaceSystemRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceKubernetesNamespaceSystemUnset(d *schema.ResourceData, meta interface{}) error {
	d.Set("system", false)
	return resourceKubernetesNamespaceSystemToggle(d, meta)
}
