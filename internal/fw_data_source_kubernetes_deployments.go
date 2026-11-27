package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func newKubernetesDeploymentsDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_deployments", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"environment_id": k8sDSRequiredInt("Environment (endpoint) identifier of the Kubernetes environment to query."),
			"namespace":      k8sDSOptionalString("Namespace to list deployments from. Leave unset to list deployments across every namespace the API token can access."),
			"label_selector": k8sDSOptionalString("Kubernetes label selector used to filter the deployments (for example `app=nginx`)."),
			"field_selector": k8sDSOptionalString("Kubernetes field selector used to filter the deployments (for example `metadata.name=web`)."),
			// Computed attributes
			"deployments": k8sDSObjectList("Deployments matching the query, in the order returned by the Kubernetes API.",
				k8sDSField{"name", types.StringType, "Name of the deployment."},
				k8sDSField{"namespace", types.StringType, "Namespace the deployment lives in."},
				k8sDSField{"labels", k8sDSStringMap, "Labels set on the deployment object."},
				k8sDSField{"images", k8sDSStringList, "Container images declared by the deployment's pod template, in container order."},
				k8sDSField{"replicas", types.Int64Type, "Desired replica count from the deployment spec."},
				k8sDSField{"ready_replicas", types.Int64Type, "Number of replicas passing their readiness checks."},
				k8sDSField{"available_replicas", types.Int64Type, "Number of replicas available for at least the configured minimum ready seconds."},
				k8sDSField{"updated_replicas", types.Int64Type, "Number of replicas already running the desired pod template."},
				k8sDSField{"unavailable_replicas", types.Int64Type, "Number of replicas that are still unavailable."},
				k8sDSField{"generation", types.Int64Type, "Metadata generation of the deployment, bumped on every spec change."},
				k8sDSField{"observed_generation", types.Int64Type, "Generation the deployment controller has already acted on. A value below `generation` means a rollout is still pending."},
				k8sDSField{"creation_timestamp", types.StringType, "RFC3339 creation timestamp of the deployment."},
			),
		},
	}, nil, kubernetesDeploymentsRead)
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

func kubernetesDeploymentsRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	envID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)

	path := fmt.Sprintf("/kubernetes/%d/deployments", envID)
	if namespace != "" {
		path = fmt.Sprintf("/kubernetes/%d/namespaces/%s/deployments", envID, namespace)
	}

	var result k8sDeploymentList
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+path+kubernetesListQueryFW(d, nil), nil, &result); err != nil {
		return fmt.Errorf("failed to list Kubernetes deployments: %w", err)
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
		return err
	}

	d.SetID(fmt.Sprintf("%d/%s/deployments", envID, namespace))
	return nil
}
