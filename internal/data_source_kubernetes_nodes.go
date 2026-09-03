package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceKubernetesNodes() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesNodesRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier of the Kubernetes environment to query.",
			},
			// Computed attributes
			"nodes": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Nodes of the cluster, with their capacity, remaining allocatable headroom and current usage.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the node.",
						},
						"ready": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether the node's `Ready` condition is true.",
						},
						"unschedulable": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether the node is cordoned, which is what `portainer_kubernetes_node_drain` leaves behind.",
						},
						"kubelet_version": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Kubelet version running on the node.",
						},
						"os_image": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Operating system image of the node.",
						},
						"internal_ip": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Internal address of the node.",
						},
						"labels": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Labels set on the node, including its role labels.",
						},
						"capacity_cpu": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Total CPU capacity as a Kubernetes quantity string.",
						},
						"capacity_memory": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Total memory capacity as a Kubernetes quantity string.",
						},
						"available_cpu": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "CPU still schedulable on the node, in millicores, as Portainer computes it.",
						},
						"available_memory": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Memory still schedulable on the node, in bytes, as Portainer computes it.",
						},
						"usage_cpu": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Current CPU usage from the metrics server, empty when metrics-server is not installed.",
						},
						"usage_memory": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Current memory usage from the metrics server, empty when metrics-server is not installed.",
						},
					},
				},
			},
		},
	}
}

func dataSourceKubernetesNodesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID := d.Get("environment_id").(int)

	var nodes []struct {
		Metadata struct {
			Name   string            `json:"name"`
			Labels map[string]string `json:"labels"`
		} `json:"metadata"`
		Spec struct {
			Unschedulable bool `json:"unschedulable"`
		} `json:"spec"`
		Status struct {
			Capacity   map[string]string `json:"capacity"`
			Conditions []struct {
				Type   string `json:"type"`
				Status string `json:"status"`
			} `json:"conditions"`
			NodeInfo struct {
				KubeletVersion string `json:"kubeletVersion"`
				OSImage        string `json:"osImage"`
			} `json:"nodeInfo"`
			Addresses []struct {
				Type    string `json:"type"`
				Address string `json:"address"`
			} `json:"addresses"`
		} `json:"status"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/kubernetes/%d/nodes", client.Endpoint, envID), nil, &nodes); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the nodes of environment %d: %w", envID, err))
	}

	// K8sNodesLimits is map[nodeName]{CPU, Memory} — Portainer's own view of what
	// is still schedulable, which is not the same as capacity minus usage.
	var limits map[string]struct {
		CPU    int `json:"CPU"`
		Memory int `json:"Memory"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/kubernetes/%d/nodes_limits", client.Endpoint, envID), nil, &limits); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the node limits of environment %d: %w", envID, err))
	}

	// Metrics need metrics-server in the cluster; without it the endpoint fails
	// and usage is simply reported as unknown rather than failing the read.
	usage := map[string]struct{ cpu, memory string }{}
	var metrics struct {
		Items []struct {
			Metadata struct {
				Name string `json:"name"`
			} `json:"metadata"`
			Usage map[string]string `json:"usage"`
		} `json:"items"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/kubernetes/%d/metrics/nodes", client.Endpoint, envID), nil, &metrics); err == nil {
		for _, m := range metrics.Items {
			usage[m.Metadata.Name] = struct{ cpu, memory string }{m.Usage["cpu"], m.Usage["memory"]}
		}
	}

	out := make([]map[string]interface{}, len(nodes))
	for i, n := range nodes {
		ready := false
		for _, c := range n.Status.Conditions {
			if c.Type == "Ready" {
				ready = c.Status == "True"
				break
			}
		}
		internalIP := ""
		for _, a := range n.Status.Addresses {
			if a.Type == "InternalIP" {
				internalIP = a.Address
				break
			}
		}
		labels := n.Metadata.Labels
		if labels == nil {
			labels = map[string]string{}
		}

		out[i] = map[string]interface{}{
			"name":             n.Metadata.Name,
			"ready":            ready,
			"unschedulable":    n.Spec.Unschedulable,
			"kubelet_version":  n.Status.NodeInfo.KubeletVersion,
			"os_image":         n.Status.NodeInfo.OSImage,
			"internal_ip":      internalIP,
			"labels":           labels,
			"capacity_cpu":     n.Status.Capacity["cpu"],
			"capacity_memory":  n.Status.Capacity["memory"],
			"available_cpu":    limits[n.Metadata.Name].CPU,
			"available_memory": limits[n.Metadata.Name].Memory,
			"usage_cpu":        usage[n.Metadata.Name].cpu,
			"usage_memory":     usage[n.Metadata.Name].memory,
		}
	}

	if err := d.Set("nodes", out); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(envID) + "/nodes")
	return nil
}
