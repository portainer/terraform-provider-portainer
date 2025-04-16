package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type IngressController struct {
	Availability bool   `json:"Availability"`
	ClassName    string `json:"ClassName"`
	Name         string `json:"Name"`
	New          bool   `json:"New"`
	Type         string `json:"Type"`
	Used         bool   `json:"Used"`
}

func resourceKubernetesIngressControllers() *schema.Resource {
	return &schema.Resource{
		Create: resourceKubernetesIngressControllersCreate,
		Read:   schema.Noop,
		Update: resourceKubernetesIngressControllersCreate,
		Delete: schema.Noop,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"controllers": {
				Type:     schema.TypeList,
				Required: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"availability": {Type: schema.TypeBool, Required: true},
						"class_name":   {Type: schema.TypeString, Required: true},
						"name":         {Type: schema.TypeString, Required: true},
						"new":          {Type: schema.TypeBool, Required: true},
						"type":         {Type: schema.TypeString, Required: true},
						"used":         {Type: schema.TypeBool, Required: true},
					},
				},
			},
		},
	}
}

func resourceKubernetesIngressControllersCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Get("environment_id").(int)

	var controllers []IngressController
	for _, raw := range d.Get("controllers").([]interface{}) {
		data := raw.(map[string]interface{})
		controllers = append(controllers, IngressController{
			Availability: data["availability"].(bool),
			ClassName:    data["class_name"].(string),
			Name:         data["name"].(string),
			New:          data["new"].(bool),
			Type:         data["type"].(string),
			Used:         data["used"].(bool),
		})
	}

	jsonBody, _ := json.Marshal(controllers)
	url := fmt.Sprintf("%s/kubernetes/%d/ingresscontrollers", client.Endpoint, id)
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
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update ingress controllers: %s", body)
	}

	d.SetId(strconv.Itoa(id))
	return nil
}
