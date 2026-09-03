package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// k8sContainerUsage is the shared shape of the metrics-server container entry.
func k8sContainerUsageSchema() *schema.Schema {
	return &schema.Schema{
		Type:        schema.TypeList,
		Computed:    true,
		Description: "Per-container usage, as reported by metrics-server.",
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"name":   {Type: schema.TypeString, Computed: true, Description: "Name of the container."},
				"cpu":    {Type: schema.TypeString, Computed: true, Description: "CPU usage as a Kubernetes quantity string, such as `250m`."},
				"memory": {Type: schema.TypeString, Computed: true, Description: "Memory usage as a Kubernetes quantity string, such as `1Gi`."},
			},
		},
	}
}

func dataSourceKubernetesPodMetrics() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesPodMetricsRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier of the Kubernetes environment to query.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Namespace whose pod metrics are read.",
			},
			"pod": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Single pod to read metrics for. Leave unset to read every pod in the namespace.",
			},
			// Computed attributes
			"pods": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Pod metrics matching the query. Requires metrics-server in the cluster.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name":       {Type: schema.TypeString, Computed: true, Description: "Name of the pod."},
						"timestamp":  {Type: schema.TypeString, Computed: true, Description: "When the sample was taken."},
						"window":     {Type: schema.TypeString, Computed: true, Description: "Length of the sampling window, such as `30s`."},
						"containers": k8sContainerUsageSchema(),
					},
				},
			},
		},
	}
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

func dataSourceKubernetesPodMetricsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)

	var pods []map[string]interface{}

	if pod, ok := d.GetOk("pod"); ok {
		// The single-pod endpoint returns one object rather than a list.
		var single k8sPodMetrics
		url := fmt.Sprintf("%s/kubernetes/%d/metrics/pods/%s/%s", client.Endpoint, envID, namespace, pod.(string))
		if err := doJSON(ctx, client, http.MethodGet, url, nil, &single); err != nil {
			return diag.FromErr(fmt.Errorf("failed to read the metrics of pod %s/%s: %w", namespace, pod.(string), err))
		}
		pods = []map[string]interface{}{single.toMap()}
	} else {
		var list struct {
			Items []k8sPodMetrics `json:"items"`
		}
		url := fmt.Sprintf("%s/kubernetes/%d/metrics/pods/namespace/%s", client.Endpoint, envID, namespace)
		if err := doJSON(ctx, client, http.MethodGet, url, nil, &list); err != nil {
			return diag.FromErr(fmt.Errorf("failed to read the pod metrics of namespace %s: %w", namespace, err))
		}
		pods = make([]map[string]interface{}, len(list.Items))
		for i, p := range list.Items {
			pods[i] = p.toMap()
		}
	}

	if err := d.Set("pods", pods); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%d/%s/%s/pod-metrics", envID, namespace, d.Get("pod").(string)))
	return nil
}

func dataSourceKubernetesNodeMetrics() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesNodeMetricsRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier of the Kubernetes environment to query.",
			},
			"node": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the node to read metrics for. Use `portainer_kubernetes_nodes` for every node at once.",
			},
			// Computed attributes
			"cpu": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "CPU usage of the node as a Kubernetes quantity string.",
			},
			"memory": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Memory usage of the node as a Kubernetes quantity string.",
			},
			"timestamp": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "When the sample was taken.",
			},
			"window": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Length of the sampling window, such as `30s`.",
			},
		},
	}
}

func dataSourceKubernetesNodeMetricsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID := d.Get("environment_id").(int)
	node := d.Get("node").(string)

	var metrics struct {
		Timestamp string            `json:"timestamp"`
		Window    string            `json:"window"`
		Usage     map[string]string `json:"usage"`
	}
	url := fmt.Sprintf("%s/kubernetes/%d/metrics/nodes/%s", client.Endpoint, envID, node)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &metrics); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the metrics of node %s: %w", node, err))
	}

	if err := setFields(d, map[string]interface{}{
		"cpu": metrics.Usage["cpu"], "memory": metrics.Usage["memory"],
		"timestamp": metrics.Timestamp, "window": metrics.Window,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%d/%s/node-metrics", envID, node))
	return nil
}

func dataSourceKubernetesApplicationResources() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesApplicationResourcesRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier of the Kubernetes environment to query.",
			},
			// Computed attributes
			"cpu_request": {
				Type:        schema.TypeFloat,
				Computed:    true,
				Description: "Total CPU requested by the cluster's applications, in cores.",
			},
			"cpu_limit": {
				Type:        schema.TypeFloat,
				Computed:    true,
				Description: "Total CPU limit across the cluster's applications, in cores.",
			},
			"memory_request": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Total memory requested by the cluster's applications, in bytes.",
			},
			"memory_limit": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Total memory limit across the cluster's applications, in bytes.",
			},
		},
	}
}

func dataSourceKubernetesApplicationResourcesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID := d.Get("environment_id").(int)

	var resources struct {
		CPURequest    float64 `json:"CpuRequest"`
		CPULimit      float64 `json:"CpuLimit"`
		MemoryRequest int     `json:"MemoryRequest"`
		MemoryLimit   int     `json:"MemoryLimit"`
	}
	url := fmt.Sprintf("%s/kubernetes/%d/metrics/applications_resources", client.Endpoint, envID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &resources); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the application resources of environment %d: %w", envID, err))
	}

	if err := setFields(d, map[string]interface{}{
		"cpu_request": resources.CPURequest, "cpu_limit": resources.CPULimit,
		"memory_request": resources.MemoryRequest, "memory_limit": resources.MemoryLimit,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%d/application-resources", envID))
	return nil
}
