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

func newKubernetesNodesDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_nodes", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"environment_id": k8sDSRequiredInt("Environment (endpoint) identifier of the Kubernetes environment to query."),
			// Computed attributes
			"nodes": k8sDSObjectList("Nodes of the cluster, with their capacity, remaining allocatable headroom and current usage.",
				k8sDSField{"name", types.StringType, "Name of the node."},
				k8sDSField{"ready", types.BoolType, "Whether the node's `Ready` condition is true."},
				k8sDSField{"unschedulable", types.BoolType, "Whether the node is cordoned, which is what `portainer_kubernetes_node_drain` leaves behind."},
				k8sDSField{"kubelet_version", types.StringType, "Kubelet version running on the node."},
				k8sDSField{"os_image", types.StringType, "Operating system image of the node."},
				k8sDSField{"internal_ip", types.StringType, "Internal address of the node."},
				k8sDSField{"labels", k8sDSStringMap, "Labels set on the node, including its role labels."},
				k8sDSField{"capacity_cpu", types.StringType, "Total CPU capacity as a Kubernetes quantity string."},
				k8sDSField{"capacity_memory", types.StringType, "Total memory capacity as a Kubernetes quantity string."},
				k8sDSField{"available_cpu", types.Int64Type, "CPU still schedulable on the node, in millicores, as Portainer computes it."},
				k8sDSField{"available_memory", types.Int64Type, "Memory still schedulable on the node, in bytes, as Portainer computes it."},
				k8sDSField{"usage_cpu", types.StringType, "Current CPU usage from the metrics server, empty when metrics-server is not installed."},
				k8sDSField{"usage_memory", types.StringType, "Current memory usage from the metrics server, empty when metrics-server is not installed."},
			),
		},
	}, nil, kubernetesNodesRead)
}

func kubernetesNodesRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
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
		return fmt.Errorf("failed to list the nodes of environment %d: %w", envID, err)
	}

	// K8sNodesLimits is map[nodeName]{CPU, Memory} — Portainer's own view of what
	// is still schedulable, which is not the same as capacity minus usage.
	var limits map[string]struct {
		CPU    int `json:"CPU"`
		Memory int `json:"Memory"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/kubernetes/%d/nodes_limits", client.Endpoint, envID), nil, &limits); err != nil {
		return fmt.Errorf("failed to read the node limits of environment %d: %w", envID, err)
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
		return err
	}

	d.SetID(strconv.Itoa(envID) + "/nodes")
	return nil
}
