package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
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
		CreateContext: resourceKubernetesIngressControllersCreate,
		ReadContext:   resourceKubernetesIngressControllersRead,
		UpdateContext: resourceKubernetesIngressControllersCreate,
		DeleteContext: resourceKubernetesIngressControllersDelete,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer Kubernetes environment whose ingress controllers are managed.",
			},
			"controllers": {
				Type:        schema.TypeList,
				Required:    true,
				Description: "List of ingress controller configurations to apply at the cluster level.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"availability": {Type: schema.TypeBool, Required: true, Description: "Whether the ingress controller is exposed for selection in the Portainer UI."},
						"class_name":   {Type: schema.TypeString, Required: true, Description: "Kubernetes IngressClass name associated with the controller."},
						"name":         {Type: schema.TypeString, Required: true, Description: "Display name of the ingress controller in Portainer."},
						"new":          {Type: schema.TypeBool, Required: true, Description: "Whether Portainer should treat this entry as a newly-registered controller."},
						"type":         {Type: schema.TypeString, Required: true, Description: "Type of the ingress controller (for example `nginx`, `traefik`, `custom`)."},
						"used":         {Type: schema.TypeBool, Required: true, Description: "Whether the ingress controller is currently in use by workloads."},
					},
				},
			},
		},
	}
}

func resourceKubernetesIngressControllersCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Get("environment_id").(int)

	controllers := make([]IngressController, 0, len(d.Get("controllers").([]interface{})))
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
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(jsonBody))
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
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to update ingress controllers: %s", body))
	}

	d.SetId(strconv.Itoa(id))
	return resourceKubernetesIngressControllersRead(ctx, d, meta)
}

func resourceKubernetesIngressControllersRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Get("environment_id").(int)

	url := fmt.Sprintf("%s/kubernetes/%d/ingresscontrollers", client.Endpoint, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
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
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		d.SetId("")
		return nil
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to read ingress controllers: %s", body))
	}

	controllers := make([]IngressController, 0, len(d.Get("controllers").([]interface{})))
	if err := json.NewDecoder(resp.Body).Decode(&controllers); err != nil {
		return diag.FromErr(err)
	}

	controllersList := make([]map[string]interface{}, len(controllers))
	for i, c := range controllers {
		controllersList[i] = map[string]interface{}{
			"availability": c.Availability,
			"class_name":   c.ClassName,
			"name":         c.Name,
			"new":          c.New,
			"type":         c.Type,
			"used":         c.Used,
		}
	}
	if err := d.Set("controllers", controllersList); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceKubernetesIngressControllersDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Get("environment_id").(int)

	// No DELETE endpoint exists; disable all controllers via PUT to clean up.
	controllers := make([]IngressController, 0, len(d.Get("controllers").([]interface{})))
	for _, raw := range d.Get("controllers").([]interface{}) {
		data := raw.(map[string]interface{})
		controllers = append(controllers, IngressController{
			Availability: false,
			ClassName:    data["class_name"].(string),
			Name:         data["name"].(string),
			New:          data["new"].(bool),
			Type:         data["type"].(string),
			Used:         data["used"].(bool),
		})
	}

	jsonBody, _ := json.Marshal(controllers)
	url := fmt.Sprintf("%s/kubernetes/%d/ingresscontrollers", client.Endpoint, id)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(jsonBody))
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
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to disable ingress controllers: %s", body))
	}

	return nil
}
