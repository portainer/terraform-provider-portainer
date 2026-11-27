package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func newKubernetesPodsDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_pods", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"environment_id": k8sDSRequiredInt("Environment (endpoint) identifier of the Kubernetes environment to query."),
			"namespace":      k8sDSOptionalString("Namespace to list pods from. Leave unset to list pods across every namespace the API token can access."),
			"label_selector": k8sDSOptionalString("Kubernetes label selector used to filter the pods (for example `app=nginx`)."),
			"field_selector": k8sDSOptionalString("Kubernetes field selector used to filter the pods (for example `status.phase=Running`)."),
			// Computed attributes
			"pods": k8sDSObjectList("Pods matching the query, in the order returned by the Kubernetes API.",
				k8sDSField{"name", types.StringType, "Name of the pod."},
				k8sDSField{"namespace", types.StringType, "Namespace the pod lives in."},
				k8sDSField{"node_name", types.StringType, "Node the pod is scheduled on, empty while the pod is still pending."},
				k8sDSField{"phase", types.StringType, "Pod lifecycle phase: `Pending`, `Running`, `Succeeded`, `Failed` or `Unknown`."},
				k8sDSField{"pod_ip", types.StringType, "IP address assigned to the pod, empty before it is scheduled."},
				k8sDSField{"host_ip", types.StringType, "IP address of the node hosting the pod."},
				k8sDSField{"service_account", types.StringType, "Service account the pod runs as."},
				k8sDSField{"labels", k8sDSStringMap, "Labels set on the pod object."},
				k8sDSField{"ready", types.BoolType, "Whether every container in the pod reports ready."},
				k8sDSField{"restart_count", types.Int64Type, "Total container restarts across the pod."},
				k8sDSField{"containers", types.ListType{ElemType: k8sDSObject(
					k8sDSField{"name", types.StringType, "Name of the container."},
					k8sDSField{"image", types.StringType, "Image the container is declared with."},
					k8sDSField{"ready", types.BoolType, "Whether the container currently reports ready."},
					k8sDSField{"restart_count", types.Int64Type, "Number of times the container has restarted."},
				)}, "Containers declared by the pod, in container order."},
				k8sDSField{"start_time", types.StringType, "RFC3339 timestamp at which the kubelet acknowledged the pod, empty while it is pending."},
			),
		},
	}, nil, kubernetesPodsRead)
}

func kubernetesPodsRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	envID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)

	path := fmt.Sprintf("/kubernetes/%d/pods", envID)
	if namespace != "" {
		path = fmt.Sprintf("/kubernetes/%d/namespaces/%s/pods", envID, namespace)
	}

	var result struct {
		Items []struct {
			Metadata struct {
				Name      string            `json:"name"`
				Namespace string            `json:"namespace"`
				Labels    map[string]string `json:"labels"`
			} `json:"metadata"`
			Spec struct {
				NodeName           string `json:"nodeName"`
				ServiceAccountName string `json:"serviceAccountName"`
				Containers         []struct {
					Name  string `json:"name"`
					Image string `json:"image"`
				} `json:"containers"`
			} `json:"spec"`
			Status struct {
				Phase             string `json:"phase"`
				PodIP             string `json:"podIP"`
				HostIP            string `json:"hostIP"`
				StartTime         string `json:"startTime"`
				ContainerStatuses []struct {
					Name         string `json:"name"`
					Ready        bool   `json:"ready"`
					RestartCount int    `json:"restartCount"`
				} `json:"containerStatuses"`
			} `json:"status"`
		} `json:"items"`
	}

	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+path+kubernetesListQueryFW(d, nil), nil, &result); err != nil {
		return fmt.Errorf("failed to list Kubernetes pods: %w", err)
	}

	pods := make([]map[string]interface{}, len(result.Items))
	for i, p := range result.Items {
		// Readiness and restart counts live in status.containerStatuses, which is
		// keyed by container name and absent until the kubelet reports in.
		type containerState struct {
			ready        bool
			restartCount int
			reported     bool
		}
		states := make(map[string]containerState, len(p.Status.ContainerStatuses))
		restarts := 0
		for _, cs := range p.Status.ContainerStatuses {
			states[cs.Name] = containerState{ready: cs.Ready, restartCount: cs.RestartCount, reported: true}
			restarts += cs.RestartCount
		}

		containers := make([]map[string]interface{}, len(p.Spec.Containers))
		allReady := len(p.Spec.Containers) > 0
		for j, c := range p.Spec.Containers {
			st := states[c.Name]
			if !st.reported || !st.ready {
				allReady = false
			}
			containers[j] = map[string]interface{}{
				"name":          c.Name,
				"image":         c.Image,
				"ready":         st.ready,
				"restart_count": st.restartCount,
			}
		}

		labels := p.Metadata.Labels
		if labels == nil {
			labels = map[string]string{}
		}

		pods[i] = map[string]interface{}{
			"name":            p.Metadata.Name,
			"namespace":       p.Metadata.Namespace,
			"node_name":       p.Spec.NodeName,
			"phase":           p.Status.Phase,
			"pod_ip":          p.Status.PodIP,
			"host_ip":         p.Status.HostIP,
			"service_account": p.Spec.ServiceAccountName,
			"labels":          labels,
			"ready":           allReady,
			"restart_count":   restarts,
			"containers":      containers,
			"start_time":      p.Status.StartTime,
		}
	}

	if err := d.Set("pods", pods); err != nil {
		return err
	}

	d.SetID(fmt.Sprintf("%d/%s/pods", envID, namespace))
	return nil
}
