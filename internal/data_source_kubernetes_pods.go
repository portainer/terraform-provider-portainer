package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceKubernetesPods() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesPodsRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier of the Kubernetes environment to query.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Namespace to list pods from. Leave unset to list pods across every namespace the API token can access.",
			},
			"label_selector": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Kubernetes label selector used to filter the pods (for example `app=nginx`).",
			},
			"field_selector": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Kubernetes field selector used to filter the pods (for example `status.phase=Running`).",
			},
			// Computed attributes
			"pods": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Pods matching the query, in the order returned by the Kubernetes API.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the pod.",
						},
						"namespace": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Namespace the pod lives in.",
						},
						"node_name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Node the pod is scheduled on, empty while the pod is still pending.",
						},
						"phase": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Pod lifecycle phase: `Pending`, `Running`, `Succeeded`, `Failed` or `Unknown`.",
						},
						"pod_ip": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "IP address assigned to the pod, empty before it is scheduled.",
						},
						"host_ip": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "IP address of the node hosting the pod.",
						},
						"service_account": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Service account the pod runs as.",
						},
						"labels": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Labels set on the pod object.",
						},
						"ready": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether every container in the pod reports ready.",
						},
						"restart_count": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Total container restarts across the pod.",
						},
						"containers": {
							Type:        schema.TypeList,
							Computed:    true,
							Description: "Containers declared by the pod, in container order.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"name": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Name of the container.",
									},
									"image": {
										Type:        schema.TypeString,
										Computed:    true,
										Description: "Image the container is declared with.",
									},
									"ready": {
										Type:        schema.TypeBool,
										Computed:    true,
										Description: "Whether the container currently reports ready.",
									},
									"restart_count": {
										Type:        schema.TypeInt,
										Computed:    true,
										Description: "Number of times the container has restarted.",
									},
								},
							},
						},
						"start_time": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "RFC3339 timestamp at which the kubelet acknowledged the pod, empty while it is pending.",
						},
					},
				},
			},
		},
	}
}

func dataSourceKubernetesPodsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

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

	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+path+kubernetesListQuery(d, nil), nil, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list Kubernetes pods: %w", err))
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
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%d/%s/pods", envID, namespace))
	return nil
}
