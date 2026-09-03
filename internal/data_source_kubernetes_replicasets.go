package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// deploymentRevisionAnnotation carries the rollout revision a ReplicaSet belongs
// to; it is what `kubectl rollout history` reports and what the deployment
// rollback endpoint expects as its target revision.
const deploymentRevisionAnnotation = "deployment.kubernetes.io/revision"

func dataSourceKubernetesReplicaSets() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesReplicaSetsRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier of the Kubernetes environment to query.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Namespace whose replica sets are listed.",
			},
			"deployment": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Only return replica sets owned by this deployment. Combined with `revision` on each result, this is how a rollback target is discovered.",
			},
			"label_selector": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Kubernetes label selector used to filter the replica sets (for example `app=nginx`).",
			},
			"field_selector": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Kubernetes field selector used to filter the replica sets (for example `metadata.name=web-1234`).",
			},
			// Computed attributes
			"replicasets": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Replica sets matching the query, in the order returned by the Kubernetes API.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the replica set.",
						},
						"namespace": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Namespace the replica set lives in.",
						},
						"revision": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Rollout revision this replica set represents, read from the `deployment.kubernetes.io/revision` annotation. Zero when the replica set is not owned by a deployment.",
						},
						"labels": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Labels set on the replica set object.",
						},
						"images": {
							Type:        schema.TypeList,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Container images declared by the replica set's pod template, in container order.",
						},
						"replicas": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Desired replica count from the replica set spec.",
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
						"fully_labeled_replicas": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Number of replicas whose labels match the replica set's pod template exactly.",
						},
						"creation_timestamp": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "RFC3339 creation timestamp of the replica set.",
						},
					},
				},
			},
		},
	}
}

func dataSourceKubernetesReplicaSetsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)

	var result struct {
		Items []struct {
			Metadata struct {
				Name              string            `json:"name"`
				Namespace         string            `json:"namespace"`
				Labels            map[string]string `json:"labels"`
				Annotations       map[string]string `json:"annotations"`
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
				ReadyReplicas        int `json:"readyReplicas"`
				AvailableReplicas    int `json:"availableReplicas"`
				FullyLabeledReplicas int `json:"fullyLabeledReplicas"`
			} `json:"status"`
		} `json:"items"`
	}

	path := fmt.Sprintf("/kubernetes/%d/namespaces/%s/replicasets", envID, namespace)
	query := kubernetesListQuery(d, map[string]string{"deployment": d.Get("deployment").(string)})
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+path+query, nil, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list Kubernetes replica sets in namespace %q: %w", namespace, err))
	}

	replicaSets := make([]map[string]interface{}, len(result.Items))
	for i, rs := range result.Items {
		images := make([]string, len(rs.Spec.Template.Spec.Containers))
		for j, c := range rs.Spec.Template.Spec.Containers {
			images[j] = c.Image
		}
		labels := rs.Metadata.Labels
		if labels == nil {
			labels = map[string]string{}
		}
		replicas := 1
		if rs.Spec.Replicas != nil {
			replicas = *rs.Spec.Replicas
		}
		revision := 0
		if raw, ok := rs.Metadata.Annotations[deploymentRevisionAnnotation]; ok {
			// A non-numeric annotation is not worth failing the read over; it
			// just means no usable rollback target for this replica set.
			if parsed, err := strconv.Atoi(raw); err == nil {
				revision = parsed
			}
		}

		replicaSets[i] = map[string]interface{}{
			"name":                   rs.Metadata.Name,
			"namespace":              rs.Metadata.Namespace,
			"revision":               revision,
			"labels":                 labels,
			"images":                 images,
			"replicas":               replicas,
			"ready_replicas":         rs.Status.ReadyReplicas,
			"available_replicas":     rs.Status.AvailableReplicas,
			"fully_labeled_replicas": rs.Status.FullyLabeledReplicas,
			"creation_timestamp":     rs.Metadata.CreationTimestamp,
		}
	}

	if err := d.Set("replicasets", replicaSets); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%d/%s/replicasets", envID, namespace))
	return nil
}
