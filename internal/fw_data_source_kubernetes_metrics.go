package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// k8sContainerUsageField is the shared shape of the metrics-server container
// entry.
func k8sContainerUsageField() k8sDSField {
	return k8sDSField{"containers", types.ListType{ElemType: k8sDSObject(
		k8sDSField{"name", types.StringType, "Name of the container."},
		k8sDSField{"cpu", types.StringType, "CPU usage as a Kubernetes quantity string, such as `250m`."},
		k8sDSField{"memory", types.StringType, "Memory usage as a Kubernetes quantity string, such as `1Gi`."},
	)}, "Per-container usage, as reported by metrics-server."}
}

func newKubernetesPodMetricsDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_pod_metrics", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"environment_id": k8sDSRequiredInt("Environment (endpoint) identifier of the Kubernetes environment to query."),
			"namespace":      k8sDSRequiredString("Namespace whose pod metrics are read."),
			"pod":            k8sDSOptionalString("Single pod to read metrics for. Leave unset to read every pod in the namespace."),
			// Computed attributes
			"pods": k8sDSObjectList("Pod metrics matching the query. Requires metrics-server in the cluster.",
				k8sDSField{"name", types.StringType, "Name of the pod."},
				k8sDSField{"timestamp", types.StringType, "When the sample was taken."},
				k8sDSField{"window", types.StringType, "Length of the sampling window, such as `30s`."},
				k8sContainerUsageField(),
			),
		},
	}, nil, kubernetesPodMetricsRead)
}

type k8sPodMetrics struct {
	Metadata struct {
		Name string `json:"name"`
	} `json:"metadata"`
	Timestamp  string `json:"timestamp"`
	Window     string `json:"window"`
	Containers []struct {
		Name  string            `json:"name"`
		Usage map[string]string `json:"usage"`
	} `json:"containers"`
}

func (p k8sPodMetrics) toMap() map[string]interface{} {
	containers := make([]map[string]interface{}, len(p.Containers))
	for i, c := range p.Containers {
		containers[i] = map[string]interface{}{
			"name": c.Name, "cpu": c.Usage["cpu"], "memory": c.Usage["memory"],
		}
	}
	return map[string]interface{}{
		"name": p.Metadata.Name, "timestamp": p.Timestamp,
		"window": p.Window, "containers": containers,
	}
}

func kubernetesPodMetricsRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	envID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)

	var pods []map[string]interface{}

	if pod, ok := d.GetOk("pod"); ok {
		// The single-pod endpoint returns one object rather than a list.
		var single k8sPodMetrics
		url := fmt.Sprintf("%s/kubernetes/%d/metrics/pods/%s/%s", client.Endpoint, envID, namespace, pod.(string))
		if err := doJSON(ctx, client, http.MethodGet, url, nil, &single); err != nil {
			return fmt.Errorf("failed to read the metrics of pod %s/%s: %w", namespace, pod.(string), err)
		}
		pods = []map[string]interface{}{single.toMap()}
	} else {
		var list struct {
			Items []k8sPodMetrics `json:"items"`
		}
		url := fmt.Sprintf("%s/kubernetes/%d/metrics/pods/namespace/%s", client.Endpoint, envID, namespace)
		if err := doJSON(ctx, client, http.MethodGet, url, nil, &list); err != nil {
			return fmt.Errorf("failed to read the pod metrics of namespace %s: %w", namespace, err)
		}
		pods = make([]map[string]interface{}, len(list.Items))
		for i, p := range list.Items {
			pods[i] = p.toMap()
		}
	}

	if err := d.Set("pods", pods); err != nil {
		return err
	}

	d.SetID(fmt.Sprintf("%d/%s/%s/pod-metrics", envID, namespace, d.Get("pod").(string)))
	return nil
}

func newKubernetesNodeMetricsDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_node_metrics", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"environment_id": k8sDSRequiredInt("Environment (endpoint) identifier of the Kubernetes environment to query."),
			"node":           k8sDSRequiredString("Name of the node to read metrics for. Use `portainer_kubernetes_nodes` for every node at once."),
			// Computed attributes
			"cpu":       k8sDSComputedString("CPU usage of the node as a Kubernetes quantity string."),
			"memory":    k8sDSComputedString("Memory usage of the node as a Kubernetes quantity string."),
			"timestamp": k8sDSComputedString("When the sample was taken."),
			"window":    k8sDSComputedString("Length of the sampling window, such as `30s`."),
		},
	}, nil, kubernetesNodeMetricsRead)
}

func kubernetesNodeMetricsRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	envID := d.Get("environment_id").(int)
	node := d.Get("node").(string)

	var metrics struct {
		Timestamp string            `json:"timestamp"`
		Window    string            `json:"window"`
		Usage     map[string]string `json:"usage"`
	}
	url := fmt.Sprintf("%s/kubernetes/%d/metrics/nodes/%s", client.Endpoint, envID, node)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &metrics); err != nil {
		return fmt.Errorf("failed to read the metrics of node %s: %w", node, err)
	}

	if err := d.SetFields(map[string]interface{}{
		"cpu": metrics.Usage["cpu"], "memory": metrics.Usage["memory"],
		"timestamp": metrics.Timestamp, "window": metrics.Window,
	}); err != nil {
		return err
	}

	d.SetID(fmt.Sprintf("%d/%s/node-metrics", envID, node))
	return nil
}

func newKubernetesApplicationResourcesDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_application_resources", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"environment_id": k8sDSRequiredInt("Environment (endpoint) identifier of the Kubernetes environment to query."),
			// Computed attributes
			"cpu_request":    k8sDSComputedFloat("Total CPU requested by the cluster's applications, in cores."),
			"cpu_limit":      k8sDSComputedFloat("Total CPU limit across the cluster's applications, in cores."),
			"memory_request": k8sDSComputedInt("Total memory requested by the cluster's applications, in bytes."),
			"memory_limit":   k8sDSComputedInt("Total memory limit across the cluster's applications, in bytes."),
		},
	}, nil, kubernetesApplicationResourcesRead)
}

func kubernetesApplicationResourcesRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	envID := d.Get("environment_id").(int)

	var resources struct {
		CPURequest    float64 `json:"CpuRequest"`
		CPULimit      float64 `json:"CpuLimit"`
		MemoryRequest int     `json:"MemoryRequest"`
		MemoryLimit   int     `json:"MemoryLimit"`
	}
	url := fmt.Sprintf("%s/kubernetes/%d/metrics/applications_resources", client.Endpoint, envID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &resources); err != nil {
		return fmt.Errorf("failed to read the application resources of environment %d: %w", envID, err)
	}

	if err := d.SetFields(map[string]interface{}{
		"cpu_request": resources.CPURequest, "cpu_limit": resources.CPULimit,
		"memory_request": resources.MemoryRequest, "memory_limit": resources.MemoryLimit,
	}); err != nil {
		return err
	}

	d.SetID(fmt.Sprintf("%d/application-resources", envID))
	return nil
}
