package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceKubernetesResourceQuotas() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesResourceQuotasRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier of the Kubernetes environment to query.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Namespace whose resource quotas are listed.",
			},
			// Computed attributes
			"resource_quotas": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Resource quotas defined in the namespace.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the resource quota object.",
						},
						"namespace": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Namespace the quota applies to.",
						},
						"hard": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Enforced limit per resource name (for example `limits.cpu` or `requests.memory`), as Kubernetes quantity strings.",
						},
						"used": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Current usage per resource name, as Kubernetes quantity strings.",
						},
						"scopes": {
							Type:        schema.TypeList,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Scopes the quota is restricted to (for example `BestEffort` or `NotTerminating`), empty when the quota applies to every object.",
						},
						"creation_timestamp": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "RFC3339 creation timestamp of the quota object.",
						},
					},
				},
			},
		},
	}
}

func dataSourceKubernetesResourceQuotasRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)

	// The endpoint returns a corev1.ResourceQuotaList verbatim, so quantities
	// arrive as Kubernetes quantity strings ("2", "500m", "1Gi").
	var result struct {
		Items []struct {
			Metadata struct {
				Name              string `json:"name"`
				Namespace         string `json:"namespace"`
				CreationTimestamp string `json:"creationTimestamp"`
			} `json:"metadata"`
			Spec struct {
				Hard   map[string]string `json:"hard"`
				Scopes []string          `json:"scopes"`
			} `json:"spec"`
			Status struct {
				Hard map[string]string `json:"hard"`
				Used map[string]string `json:"used"`
			} `json:"status"`
		} `json:"items"`
	}

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/resource_quotas", client.Endpoint, envID, namespace)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list Kubernetes resource quotas in namespace %q: %w", namespace, err))
	}

	quotas := make([]map[string]interface{}, len(result.Items))
	for i, q := range result.Items {
		// status.hard is what the quota controller actually enforces; it matches
		// spec.hard once observed, so spec is only the fallback before then.
		hard := q.Status.Hard
		if len(hard) == 0 {
			hard = q.Spec.Hard
		}
		used := q.Status.Used
		if used == nil {
			used = map[string]string{}
		}
		if hard == nil {
			hard = map[string]string{}
		}
		scopes := q.Spec.Scopes
		if scopes == nil {
			scopes = []string{}
		}

		quotas[i] = map[string]interface{}{
			"name":               q.Metadata.Name,
			"namespace":          q.Metadata.Namespace,
			"hard":               hard,
			"used":               used,
			"scopes":             scopes,
			"creation_timestamp": q.Metadata.CreationTimestamp,
		}
	}

	if err := d.Set("resource_quotas", quotas); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%d/%s/resource_quotas", envID, namespace))
	return nil
}
