package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceKubernetesDeployments() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesDeploymentsRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier of the Kubernetes environment to query.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Namespace to list deployments from. Leave unset to list deployments across every namespace the API token can access.",
			},
			"label_selector": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Kubernetes label selector used to filter the deployments (for example `app=nginx`).",
			},
			"field_selector": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Kubernetes field selector used to filter the deployments (for example `metadata.name=web`).",
			},
			// Computed attributes
			"deployments": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Deployments matching the query, in the order returned by the Kubernetes API.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the deployment.",
						},
						"namespace": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Namespace the deployment lives in.",
						},
						"labels": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Labels set on the deployment object.",
						},
						"images": {
							Type:        schema.TypeList,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Container images declared by the deployment's pod template, in container order.",
						},
						"replicas": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Desired replica count from the deployment spec.",
						},
						"ready_replicas": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Number of replicas passing their readiness checks.",
						},
						"available_replicas": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Number of replicas available for at least the configured minimum ready seconds.",
						},
						"updated_replicas": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Number of replicas already running the desired pod template.",
						},
						"unavailable_replicas": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Number of replicas that are still unavailable.",
						},
						"generation": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Metadata generation of the deployment, bumped on every spec change.",
						},
						"observed_generation": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Generation the deployment controller has already acted on. A value below `generation` means a rollout is still pending.",
						},
						"creation_timestamp": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "RFC3339 creation timestamp of the deployment.",
						},
					},
				},
			},
		},
	}
}

// kubernetesListQuery builds the label/field selector query string shared by the
// Kubernetes list endpoints, which all accept the same two parameters.
func kubernetesListQuery(d *schema.ResourceData, extra map[string]string) string {
	q := url.Values{}
	if v, ok := d.GetOk("label_selector"); ok {
		q.Set("labelSelector", v.(string))
	}
	if v, ok := d.GetOk("field_selector"); ok {
		q.Set("fieldSelector", v.(string))
	}
	for k, v := range extra {
		if v != "" {
			q.Set(k, v)
		}
	}
	if len(q) == 0 {
		return ""
	}
	return "?" + q.Encode()
}

// k8sDeploymentList is the subset of appsv1.DeploymentList the data source maps.
type k8sDeploymentList struct {
	Items []struct {
		Metadata struct {
			Name              string            `json:"name"`
			Namespace         string            `json:"namespace"`
			Labels            map[string]string `json:"labels"`
			Generation        int               `json:"generation"`
			CreationTimestamp string            `json:"creationTimestamp"`
		} `json:"metadata"`
		Spec struct {
			Replicas *int `json:"replicas"`
			Template struct {
				Spec struct {
					Containers []struct {
						Image string `json:"image"`
					} `json:"containers"`
				} `json:"spec"`
			} `json:"template"`
		} `json:"spec"`
		Status struct {
			ReadyReplicas       int `json:"readyReplicas"`
			AvailableReplicas   int `json:"availableReplicas"`
			UpdatedReplicas     int `json:"updatedReplicas"`
			UnavailableReplicas int `json:"unavailableReplicas"`
			ObservedGeneration  int `json:"observedGeneration"`
		} `json:"status"`
	} `json:"items"`
}

func dataSourceKubernetesDeploymentsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)

	path := fmt.Sprintf("/kubernetes/%d/deployments", envID)
	if namespace != "" {
		path = fmt.Sprintf("/kubernetes/%d/namespaces/%s/deployments", envID, namespace)
	}

	var result k8sDeploymentList
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+path+kubernetesListQuery(d, nil), nil, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list Kubernetes deployments: %w", err))
	}

	deployments := make([]map[string]interface{}, len(result.Items))
	for i, dep := range result.Items {
		images := make([]string, len(dep.Spec.Template.Spec.Containers))
		for j, c := range dep.Spec.Template.Spec.Containers {
			images[j] = c.Image
		}
		labels := dep.Metadata.Labels
		if labels == nil {
			labels = map[string]string{}
		}
		// spec.replicas is a pointer in the Kubernetes API: absent means the
		// default of 1, which is not the same as an explicit 0.
		replicas := 1
		if dep.Spec.Replicas != nil {
			replicas = *dep.Spec.Replicas
		}

		deployments[i] = map[string]interface{}{
			"name":                 dep.Metadata.Name,
			"namespace":            dep.Metadata.Namespace,
			"labels":               labels,
			"images":               images,
			"replicas":             replicas,
			"ready_replicas":       dep.Status.ReadyReplicas,
			"available_replicas":   dep.Status.AvailableReplicas,
			"updated_replicas":     dep.Status.UpdatedReplicas,
			"unavailable_replicas": dep.Status.UnavailableReplicas,
			"generation":           dep.Metadata.Generation,
			"observed_generation":  dep.Status.ObservedGeneration,
			"creation_timestamp":   dep.Metadata.CreationTimestamp,
		}
	}

	if err := d.Set("deployments", deployments); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%d/%s/deployments", envID, namespace))
	return nil
}
