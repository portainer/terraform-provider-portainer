package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceKubernetesIngressClasses() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesIngressClassesRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier of the Kubernetes environment to query.",
			},
			// Computed attributes
			"ingress_classes": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Ingress classes available in the cluster.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the ingress class, as referenced by an Ingress `ingressClassName`.",
						},
						"controller": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Controller implementing this ingress class (for example `k8s.io/ingress-nginx`).",
						},
						"is_default": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether this class is the cluster default, applied to Ingresses that name no class.",
						},
						"annotations": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Annotations set on the ingress class.",
						},
					},
				},
			},
			"default_ingress_class": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Name of the cluster's default ingress class, empty when no class is marked as default.",
			},
		},
	}
}

func dataSourceKubernetesIngressClassesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)

	// K8sIngressClass is serialized with PascalCase json tags, unlike most of
	// the Portainer API.
	var result []struct {
		Name        string            `json:"Name"`
		Controller  string            `json:"Controller"`
		IsDefault   bool              `json:"IsDefault"`
		Annotations map[string]string `json:"Annotations"`
	}

	url := fmt.Sprintf("%s/kubernetes/%d/ingressclasses", client.Endpoint, envID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list Kubernetes ingress classes: %w", err))
	}

	classes := make([]map[string]interface{}, len(result))
	defaultClass := ""
	for i, c := range result {
		annotations := c.Annotations
		if annotations == nil {
			annotations = map[string]string{}
		}
		classes[i] = map[string]interface{}{
			"name":        c.Name,
			"controller":  c.Controller,
			"is_default":  c.IsDefault,
			"annotations": annotations,
		}
		if c.IsDefault && defaultClass == "" {
			defaultClass = c.Name
		}
	}

	if err := setFields(d, map[string]interface{}{
		"ingress_classes":       classes,
		"default_ingress_class": defaultClass,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(envID) + "/ingressclasses")
	return nil
}
