package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceKubernetesGatewayClasses() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesGatewayClassesRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier of the Kubernetes environment to query.",
			},
			// Computed attributes
			"gateway_classes": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Gateway API GatewayClasses available in the cluster. Empty when no Gateway API implementation is installed.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the gateway class, as referenced by a Gateway's `class_name`.",
						},
						"controller_name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Controller implementing this gateway class, for example `gateway.envoyproxy.io/gatewayclass-controller`.",
						},
						"uid": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Kubernetes UID of the gateway class.",
						},
						"labels": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Labels set on the gateway class.",
						},
						"annotations": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Annotations set on the gateway class.",
						},
					},
				},
			},
		},
	}
}

func dataSourceKubernetesGatewayClassesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)

	// K8sGatewayClass is serialized with PascalCase json tags, like the other
	// Portainer-native Kubernetes models.
	var result []struct {
		Name           string            `json:"Name"`
		ControllerName string            `json:"ControllerName"`
		UID            string            `json:"UID"`
		Labels         map[string]string `json:"Labels"`
		Annotations    map[string]string `json:"Annotations"`
	}

	url := fmt.Sprintf("%s/kubernetes/%d/gatewayclasses", client.Endpoint, envID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &result); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_kubernetes_gateway_classes", requiresBE30)
		return diag.FromErr(fmt.Errorf("failed to list Kubernetes gateway classes: %w", err))
	}

	classes := make([]map[string]interface{}, len(result))
	for i, c := range result {
		classes[i] = map[string]interface{}{
			"name":            c.Name,
			"controller_name": c.ControllerName,
			"uid":             c.UID,
			"labels":          stringMap(c.Labels),
			"annotations":     stringMap(c.Annotations),
		}
	}

	if err := d.Set("gateway_classes", classes); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(envID) + "/gatewayclasses")
	return nil
}
