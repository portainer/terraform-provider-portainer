package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceKubernetesNamespaceIngressControllers() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesNamespaceIngressControllersCreate,
		ReadContext:   resourceKubernetesNamespaceIngressControllersRead,
		DeleteContext: resourceKubernetesNamespaceIngressControllersDelete,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer Kubernetes environment hosting the namespace.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Kubernetes namespace whose ingress controller allow-list is being configured.",
			},
			"controllers": {
				Type:        schema.TypeList,
				ForceNew:    true,
				Required:    true,
				Description: "List of ingress controllers allowed for the namespace.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Display name of the ingress controller in Portainer.",
						},
						"class_name": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Kubernetes IngressClass name associated with the controller.",
						},
						"type": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Type of the ingress controller (for example `nginx`, `traefik`, `custom`).",
						},
						"availability": {
							Type:        schema.TypeBool,
							Required:    true,
							Description: "Whether the controller is available for selection in the namespace.",
						},
						"used": {
							Type:        schema.TypeBool,
							Required:    true,
							Description: "Whether the controller is currently in use by workloads in the namespace.",
						},
						"new": {
							Type:        schema.TypeBool,
							Required:    true,
							Description: "Whether Portainer should treat this entry as a newly-registered controller.",
						},
					},
				},
			},
		},
	}
}

func resourceKubernetesNamespaceIngressControllersCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
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

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/ingresscontrollers", client.Endpoint, endpointID, namespace)

	if err := doJSON(ctx, client, http.MethodPut, url, controllers, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update namespace ingress controllers: %w", err))
	}

	d.SetId(fmt.Sprintf("%d:%s", endpointID, namespace))
	return resourceKubernetesNamespaceIngressControllersRead(ctx, d, meta)
}

func resourceKubernetesNamespaceIngressControllersRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/ingresscontrollers", client.Endpoint, endpointID, namespace)

	var controllers []map[string]interface{}
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &controllers); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read namespace ingress controllers: %w", err))
	}

	controllersList := make([]map[string]interface{}, len(controllers))
	for i, c := range controllers {
		controllersList[i] = map[string]interface{}{
			"name":         c["Name"],
			"class_name":   c["ClassName"],
			"type":         c["Type"],
			"availability": c["Availability"],
			"used":         c["Used"],
			"new":          c["New"],
		}
	}
	if err := d.Set("controllers", controllersList); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceKubernetesNamespaceIngressControllersDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)

	// No DELETE endpoint exists; disable all controllers via PUT to clean up.
	controllers := make([]map[string]interface{}, 0)
	for _, c := range d.Get("controllers").([]interface{}) {
		cMap := c.(map[string]interface{})
		controller := map[string]interface{}{
			"Name":         cMap["name"].(string),
			"ClassName":    cMap["class_name"].(string),
			"Type":         cMap["type"].(string),
			"Availability": false,
			"Used":         cMap["used"].(bool),
			"New":          cMap["new"].(bool),
		}
		controllers = append(controllers, controller)
	}

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/ingresscontrollers", client.Endpoint, endpointID, namespace)

	if err := doJSON(ctx, client, http.MethodPut, url, controllers, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to disable namespace ingress controllers: %w", err))
	}

	return nil
}
