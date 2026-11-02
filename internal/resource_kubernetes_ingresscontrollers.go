package internal

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strconv"
	"strings"

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
		Importer: &schema.ResourceImporter{
			StateContext: importKubernetesIngressControllers,
		},

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
				Elem:        ingressControllerElem(),
			},
		},
	}
}

// ingressControllerElem is a controller of the ingresscontrollers resources.
// Portainer takes only the class name and the availability; it fills name,
// type, used and new in itself, so a difference in those is not planned once
// the controller is in state (e.g. after an import).
func ingressControllerElem() *schema.Resource {
	return &schema.Resource{
		Schema: map[string]*schema.Schema{
			"availability": {Type: schema.TypeBool, Required: true, Description: "Whether the ingress controller is exposed for selection in the Portainer UI."},
			"class_name":   {Type: schema.TypeString, Required: true, Description: "Kubernetes IngressClass name associated with the controller. Must exist in the cluster."},
			"name":         {Type: schema.TypeString, Required: true, DiffSuppressFunc: ingressControllerInfoUnchanged, Description: "Display name of the ingress controller. Informational: Portainer ignores it."},
			"new":          {Type: schema.TypeBool, Required: true, DiffSuppressFunc: ingressControllerInfoUnchanged, Description: "Whether Portainer treats the controller as newly registered. Informational: Portainer ignores it."},
			"type":         {Type: schema.TypeString, Required: true, DiffSuppressFunc: ingressControllerInfoUnchanged, Description: "Type of the ingress controller (for example `nginx`, `traefik`, `custom`). Informational: Portainer ignores it."},
			"used":         {Type: schema.TypeBool, Required: true, DiffSuppressFunc: ingressControllerInfoUnchanged, Description: "Whether the ingress controller is in use by workloads. Informational: Portainer ignores it."},
		},
	}
}

func ingressControllerInfoUnchanged(_, old, _ string, d *schema.ResourceData) bool {
	return d.Id() != "" && old != ""
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

	url := fmt.Sprintf("%s/kubernetes/%d/ingresscontrollers", client.Endpoint, id)
	if err := checkIngressClassesExist(ctx, client, url, controllers); err != nil {
		return diag.FromErr(err)
	}
	unlock := lockKubernetesConfig(id)
	err := doJSON(ctx, client, http.MethodPut, url, controllers, nil)
	unlock()
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to update ingress controllers: %w", err))
	}

	d.SetId(strconv.Itoa(id))
	return resourceKubernetesIngressControllersRead(ctx, d, meta)
}

func resourceKubernetesIngressControllersRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Get("environment_id").(int)

	url := fmt.Sprintf("%s/kubernetes/%d/ingresscontrollers", client.Endpoint, id)
	controllers := make([]IngressController, 0, len(d.Get("controllers").([]interface{})))
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &controllers); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read ingress controllers: %w", err))
	}

	if err := d.Set("controllers", refreshIngressControllers(controllers, d.Get("controllers").([]interface{}))); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

// refreshIngressControllers returns the configured controllers, in their
// order, with the availability the cluster reports. Portainer lists every
// controller of the cluster and fills name, type, used and new in itself, and
// takes nothing but the class name and the availability on update, so reading
// the list back verbatim planned changes (or, per namespace, a replacement)
// whenever the cluster had other controllers or an ingress started using one.
// A configured class the cluster no longer has drops out, which shows as
// drift. An import, with nothing configured, takes every controller.
func refreshIngressControllers(actual []IngressController, configured []interface{}) []map[string]interface{} {
	byClass := make(map[string]IngressController, len(actual))
	for _, c := range actual {
		byClass[c.ClassName] = c
	}
	if len(configured) == 0 {
		list := make([]map[string]interface{}, 0, len(actual))
		for _, c := range actual {
			list = append(list, map[string]interface{}{
				"availability": c.Availability,
				"class_name":   c.ClassName,
				"name":         c.Name,
				"new":          c.New,
				"type":         c.Type,
				"used":         c.Used,
			})
		}
		return list
	}
	list := make([]map[string]interface{}, 0, len(configured))
	for _, raw := range configured {
		want := raw.(map[string]interface{})
		c, ok := byClass[want["class_name"].(string)]
		if !ok {
			continue
		}
		list = append(list, map[string]interface{}{
			"availability": c.Availability,
			"class_name":   c.ClassName,
			"name":         want["name"],
			"new":          want["new"],
			"type":         want["type"],
			"used":         want["used"],
		})
	}
	return list
}

// checkIngressClassesExist fails when a configured class is not in the
// cluster: Portainer ignores it on update, so the apply would succeed and
// every following plan would add it again. It runs before the update, so a
// rejected list changes nothing.
func checkIngressClassesExist(ctx context.Context, client *APIClient, url string, configured []IngressController) error {
	var actual []IngressController
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &actual); err != nil {
		return fmt.Errorf("failed to read ingress controllers: %w", err)
	}
	available := make([]string, 0, len(actual))
	for _, c := range actual {
		available = append(available, c.ClassName)
	}
	for _, c := range configured {
		if !slices.Contains(available, c.ClassName) {
			return fmt.Errorf("ingress class %q is not in the cluster; available classes: %s", c.ClassName, strings.Join(available, ", "))
		}
	}
	return nil
}

func resourceKubernetesIngressControllersDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Get("environment_id").(int)

	// No DELETE endpoint exists. Make the controllers available again, which
	// is Portainer's default: disabling them blocked the classes for the whole
	// cluster once the resource was gone.
	controllers := make([]IngressController, 0, len(d.Get("controllers").([]interface{})))
	for _, raw := range d.Get("controllers").([]interface{}) {
		data := raw.(map[string]interface{})
		controllers = append(controllers, IngressController{
			Availability: true,
			ClassName:    data["class_name"].(string),
			Name:         data["name"].(string),
			New:          data["new"].(bool),
			Type:         data["type"].(string),
			Used:         data["used"].(bool),
		})
	}

	url := fmt.Sprintf("%s/kubernetes/%d/ingresscontrollers", client.Endpoint, id)
	unlock := lockKubernetesConfig(id)
	defer unlock()
	if err := doJSON(ctx, client, http.MethodPut, url, controllers, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to restore ingress controllers: %w", err))
	}

	return nil
}
