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
		// The PUT replaces the namespace's list, so a change applies in place.
		UpdateContext: resourceKubernetesNamespaceIngressControllersCreate,
		DeleteContext: resourceKubernetesNamespaceIngressControllersDelete,
		Importer: &schema.ResourceImporter{
			StateContext: importKubernetesNamespaceIngressControllers,
		},

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
				Required:    true,
				Description: "List of ingress controllers allowed for the namespace.",
				Elem:        ingressControllerElem(),
			},
		},
	}
}

func importKubernetesNamespaceIngressControllers(_ context.Context, d *schema.ResourceData, _ interface{}) ([]*schema.ResourceData, error) {
	const format = "<environment_id>:<namespace>"
	parts, err := importIDParts(d.Id(), ":", 2, format)
	if err != nil {
		return nil, err
	}
	if err := setImportInts(d, d.Id(), format, map[string]string{"environment_id": parts[0]}); err != nil {
		return nil, err
	}
	if err := d.Set("namespace", parts[1]); err != nil {
		return nil, err
	}
	return []*schema.ResourceData{d}, nil
}

func resourceKubernetesNamespaceIngressControllersCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)

	controllers := make([]IngressController, 0, len(d.Get("controllers").([]interface{})))
	for _, c := range d.Get("controllers").([]interface{}) {
		cMap := c.(map[string]interface{})
		controllers = append(controllers, IngressController{
			Name:         cMap["name"].(string),
			ClassName:    cMap["class_name"].(string),
			Type:         cMap["type"].(string),
			Availability: cMap["availability"].(bool),
			Used:         cMap["used"].(bool),
			New:          cMap["new"].(bool),
		})
	}

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/ingresscontrollers", client.Endpoint, endpointID, namespace)

	if err := checkIngressClassesExist(ctx, client, url, controllers); err != nil {
		return diag.FromErr(err)
	}
	unlock := lockKubernetesConfig(endpointID)
	err := doJSON(ctx, client, http.MethodPut, url, controllers, nil)
	unlock()
	if err != nil {
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

	var controllers []IngressController
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &controllers); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read namespace ingress controllers: %w", err))
	}

	if err := d.Set("controllers", refreshIngressControllers(controllers, d.Get("controllers").([]interface{}))); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceKubernetesNamespaceIngressControllersDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)

	// No DELETE endpoint exists. Make the controllers available in the
	// namespace again, which is Portainer's default, rather than leaving them
	// blocked once the resource is gone.
	controllers := make([]IngressController, 0, len(d.Get("controllers").([]interface{})))
	for _, c := range d.Get("controllers").([]interface{}) {
		cMap := c.(map[string]interface{})
		controllers = append(controllers, IngressController{
			Name:         cMap["name"].(string),
			ClassName:    cMap["class_name"].(string),
			Type:         cMap["type"].(string),
			Availability: true,
			Used:         cMap["used"].(bool),
			New:          cMap["new"].(bool),
		})
	}

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/ingresscontrollers", client.Endpoint, endpointID, namespace)

	unlock := lockKubernetesConfig(endpointID)
	defer unlock()
	if err := doJSON(ctx, client, http.MethodPut, url, controllers, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to restore namespace ingress controllers: %w", err))
	}

	return nil
}
