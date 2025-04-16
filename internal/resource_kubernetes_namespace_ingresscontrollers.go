package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceKubernetesNamespaceIngressControllers() *schema.Resource {
	return &schema.Resource{
		Create: resourceKubernetesNamespaceIngressControllersCreate,
		Read:   schema.Noop,
		Delete: schema.Noop,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"namespace": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"controllers": {
				Type:     schema.TypeList,
				ForceNew: true,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"class_name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"type": {
							Type:     schema.TypeString,
							Required: true,
						},
						"availability": {
							Type:     schema.TypeBool,
							Required: true,
						},
						"used": {
							Type:     schema.TypeBool,
							Required: true,
						},
						"new": {
							Type:     schema.TypeBool,
							Required: true,
						},
					},
				},
			},
		},
	}
}

func resourceKubernetesNamespaceIngressControllersCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)

	controllers := make([]map[string]interface{}, 0)
	for _, c := range d.Get("controllers").([]interface{}) {
		cMap := c.(map[string]interface{})
		controller := map[string]interface{}{
			"Name":         cMap["name"].(string),
			"ClassName":    cMap["class_name"].(string),
			"Type":         cMap["type"].(string),
			"Availability": cMap["availability"].(bool),
			"Used":         cMap["used"].(bool),
			"New":          cMap["new"].(bool),
		}
		controllers = append(controllers, controller)
	}

	jsonBody, _ := json.Marshal(controllers)
	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/ingresscontrollers", client.Endpoint, endpointID, namespace)

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
		return fmt.Errorf("failed to update namespace ingress controllers: %s", string(data))
	}

	d.SetId(fmt.Sprintf("%d:%s", endpointID, namespace))
	return resourceKubernetesNamespaceIngressControllersRead(d, meta)
}

func resourceKubernetesNamespaceIngressControllersRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceKubernetesNamespaceIngressControllersDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
