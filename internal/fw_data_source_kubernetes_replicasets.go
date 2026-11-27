package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// deploymentRevisionAnnotation carries the rollout revision a ReplicaSet belongs
// to; it is what `kubectl rollout history` reports and what the deployment
// rollback endpoint expects as its target revision.
const deploymentRevisionAnnotation = "deployment.kubernetes.io/revision"

func newKubernetesReplicaSetsDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_replicasets", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"environment_id": k8sDSRequiredInt("Environment (endpoint) identifier of the Kubernetes environment to query."),
			"namespace":      k8sDSRequiredString("Namespace whose replica sets are listed."),
			"deployment":     k8sDSOptionalString("Only return replica sets owned by this deployment. Combined with `revision` on each result, this is how a rollback target is discovered."),
			"label_selector": k8sDSOptionalString("Kubernetes label selector used to filter the replica sets (for example `app=nginx`)."),
			"field_selector": k8sDSOptionalString("Kubernetes field selector used to filter the replica sets (for example `metadata.name=web-1234`)."),
			// Computed attributes
			"replicasets": k8sDSObjectList("Replica sets matching the query, in the order returned by the Kubernetes API.",
				k8sDSField{"name", types.StringType, "Name of the replica set."},
				k8sDSField{"namespace", types.StringType, "Namespace the replica set lives in."},
				k8sDSField{"revision", types.Int64Type, "Rollout revision this replica set represents, read from the `deployment.kubernetes.io/revision` annotation. Zero when the replica set is not owned by a deployment."},
				k8sDSField{"labels", k8sDSStringMap, "Labels set on the replica set object."},
				k8sDSField{"images", k8sDSStringList, "Container images declared by the replica set's pod template, in container order."},
				k8sDSField{"replicas", types.Int64Type, "Desired replica count from the replica set spec."},
				k8sDSField{"ready_replicas", types.Int64Type, "Number of replicas passing their readiness checks."},
				k8sDSField{"available_replicas", types.Int64Type, "Number of replicas available for at least the configured minimum ready seconds."},
				k8sDSField{"fully_labeled_replicas", types.Int64Type, "Number of replicas whose labels match the replica set's pod template exactly."},
				k8sDSField{"creation_timestamp", types.StringType, "RFC3339 creation timestamp of the replica set."},
			),
		},
	}, nil, kubernetesReplicaSetsRead)
}

func kubernetesReplicaSetsRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
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
	query := kubernetesListQueryFW(d, map[string]string{"deployment": d.Get("deployment").(string)})
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+path+query, nil, &result); err != nil {
		return fmt.Errorf("failed to list Kubernetes replica sets in namespace %q: %w", namespace, err)
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
		return err
	}

	d.SetID(fmt.Sprintf("%d/%s/replicasets", envID, namespace))
	return nil
}
