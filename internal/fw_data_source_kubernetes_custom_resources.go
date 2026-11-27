package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The custom resource endpoints are read-only here on purpose. Portainer
// exposes list, inspect and delete for them but no create or update: custom
// resources come from applying manifests, which the provider already does
// through portainer_kubernetes_manifest. A resource that could only delete
// what something else created would have no lifecycle Terraform can manage.

func newKubernetesCustomResourceDefinitionsDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_custom_resource_definitions", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint_id": k8sDSRequiredInt("Identifier of the Kubernetes environment to query."),
			// Computed attributes
			"definitions": k8sDSObjectList("The custom resource definitions installed in the cluster.",
				k8sDSField{"name", types.StringType, "Name of the definition, which is what `portainer_kubernetes_custom_resources` takes as its `definition`."},
				k8sDSField{"group", types.StringType, "API group the definition belongs to."},
				k8sDSField{"scope", types.StringType, "Whether the resources are `Namespaced` or `Cluster` scoped."},
				k8sDSField{"creation_date", types.StringType, "When the definition was created."},
				k8sDSField{"release_name", types.StringType, "Helm release that installed the definition, empty when it was not installed by one."},
				k8sDSField{"release_namespace", types.StringType, "Namespace of the Helm release that installed the definition."},
				k8sDSField{"release_version", types.StringType, "Version of the Helm release that installed the definition."},
			),
		},
	}, nil, kubernetesCustomResourceDefinitionsRead)
}

// k8sCustomResourceDefinition is the definition shape both the list and the
// inspect endpoint return.
type k8sCustomResourceDefinition struct {
	Name             string `json:"name"`
	Group            string `json:"group"`
	Scope            string `json:"scope"`
	CreationDate     string `json:"creationDate"`
	ReleaseName      string `json:"releaseName"`
	ReleaseNamespace string `json:"releaseNamespace"`
	ReleaseVersion   string `json:"releaseVersion"`
}

func (c k8sCustomResourceDefinition) fields() map[string]interface{} {
	return map[string]interface{}{
		"name":              c.Name,
		"group":             c.Group,
		"scope":             c.Scope,
		"creation_date":     c.CreationDate,
		"release_name":      c.ReleaseName,
		"release_namespace": c.ReleaseNamespace,
		"release_version":   c.ReleaseVersion,
	}
}

func kubernetesCustomResourceDefinitionsRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	endpointID := d.Get("endpoint_id").(int)

	var definitions []k8sCustomResourceDefinition
	listURL := fmt.Sprintf("%s/kubernetes/%d/customresourcedefinitions", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodGet, listURL, nil, &definitions); err != nil {
		return fmt.Errorf("failed to list the custom resource definitions of environment %d: %w", endpointID, err)
	}

	entries := make([]interface{}, 0, len(definitions))
	for _, definition := range definitions {
		entries = append(entries, definition.fields())
	}

	if err := d.SetFields(map[string]interface{}{"definitions": entries}); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("portainer-kubernetes-crds-%d", endpointID))
	return nil
}

func newKubernetesCustomResourceDefinitionDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_custom_resource_definition", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint_id": k8sDSRequiredInt("Identifier of the Kubernetes environment to query."),
			"name":        k8sDSRequiredString("Name of the custom resource definition to inspect."),
			// Computed attributes
			"group":             k8sDSComputedString("API group the definition belongs to."),
			"scope":             k8sDSComputedString("Whether the resources are `Namespaced` or `Cluster` scoped. This is what decides whether `portainer_kubernetes_custom_resource` needs a namespace."),
			"creation_date":     k8sDSComputedString("When the definition was created."),
			"release_name":      k8sDSComputedString("Helm release that installed the definition, empty when it was not installed by one."),
			"release_namespace": k8sDSComputedString("Namespace of the Helm release that installed the definition."),
			"release_version":   k8sDSComputedString("Version of the Helm release that installed the definition."),
		},
	}, nil, kubernetesCustomResourceDefinitionRead)
}

func kubernetesCustomResourceDefinitionRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	var definition k8sCustomResourceDefinition
	inspectURL := fmt.Sprintf("%s/kubernetes/%d/customresourcedefinitions/%s",
		client.Endpoint, endpointID, url.PathEscape(name))
	if err := doJSON(ctx, client, http.MethodGet, inspectURL, nil, &definition); err != nil {
		return fmt.Errorf("failed to read custom resource definition %s: %w", name, err)
	}

	fields := definition.fields()
	// The name is an argument here, so it is not written back from a response
	// that may spell it differently.
	delete(fields, "name")
	if err := d.SetFields(fields); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("portainer-kubernetes-crd-%d-%s", endpointID, name))
	return nil
}

func newKubernetesCustomResourcesDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_custom_resources", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint_id": k8sDSRequiredInt("Identifier of the Kubernetes environment to query."),
			"definition":  k8sDSRequiredString("Name of the custom resource definition whose resources are listed, as reported by `portainer_kubernetes_custom_resource_definitions`."),
			// Computed attributes
			"resources": k8sDSObjectList("The custom resources of that definition.",
				k8sDSField{"name", types.StringType, "Name of the resource."},
				k8sDSField{"namespace", types.StringType, "Namespace of the resource, empty for a cluster-scoped one."},
				k8sDSField{"definition_name", types.StringType, "Definition the resource belongs to."},
				k8sDSField{"uid", types.StringType, "Kubernetes UID of the resource."},
				k8sDSField{"creation_date", types.StringType, "When the resource was created."},
			),
		},
	}, nil, kubernetesCustomResourcesRead)
}

func kubernetesCustomResourcesRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	endpointID := d.Get("endpoint_id").(int)
	definition := d.Get("definition").(string)

	var resources []struct {
		Name           string `json:"name"`
		Namespace      string `json:"namespace"`
		DefinitionName string `json:"definitionName"`
		UID            string `json:"uid"`
		CreationDate   string `json:"creationDate"`
	}
	listURL := fmt.Sprintf("%s/kubernetes/%d/customresources?definition=%s",
		client.Endpoint, endpointID, url.QueryEscape(definition))
	if err := doJSON(ctx, client, http.MethodGet, listURL, nil, &resources); err != nil {
		return fmt.Errorf("failed to list the custom resources of definition %s: %w", definition, err)
	}

	entries := make([]interface{}, 0, len(resources))
	for _, resource := range resources {
		entries = append(entries, map[string]interface{}{
			"name":            resource.Name,
			"namespace":       resource.Namespace,
			"definition_name": resource.DefinitionName,
			"uid":             resource.UID,
			"creation_date":   resource.CreationDate,
		})
	}

	if err := d.SetFields(map[string]interface{}{"resources": entries}); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("portainer-kubernetes-custom-resources-%d-%s", endpointID, definition))
	return nil
}

func newKubernetesCustomResourceDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_custom_resource", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint_id": k8sDSRequiredInt("Identifier of the Kubernetes environment to query."),
			"definition":  k8sDSRequiredString("Name of the custom resource definition the resource belongs to."),
			"name":        k8sDSRequiredString("Name of the resource to fetch."),
			// Portainer has a separate path for cluster-scoped resources.
			// Leaving the namespace out is what selects it, which mirrors the
			// definition's own `scope`.
			"namespace": k8sDSOptionalString("Namespace of the resource. Leave unset for a resource whose definition is `Cluster` scoped."),
			// Computed attributes
			"manifest": k8sDSComputedString("The resource as Portainer returns it, encoded as JSON. Decode it with `jsondecode()` to read individual fields."),
		},
	}, nil, kubernetesCustomResourceRead)
}

func kubernetesCustomResourceRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	endpointID := d.Get("endpoint_id").(int)
	definition := d.Get("definition").(string)
	name := d.Get("name").(string)
	namespace := d.Get("namespace").(string)

	resourceURL := fmt.Sprintf("%s/kubernetes/%d/customresources/%s",
		client.Endpoint, endpointID, url.PathEscape(name))
	if namespace != "" {
		resourceURL = fmt.Sprintf("%s/kubernetes/%d/customresources/%s/%s",
			client.Endpoint, endpointID, url.PathEscape(namespace), url.PathEscape(name))
	}
	resourceURL += "?definition=" + url.QueryEscape(definition)

	// A custom resource has no fixed shape, so it is carried through as JSON
	// rather than flattened into attributes that could never cover every CRD.
	var body json.RawMessage
	if err := doJSON(ctx, client, http.MethodGet, resourceURL, nil, &body); err != nil {
		return fmt.Errorf("failed to read custom resource %s: %w", name, err)
	}

	if err := d.SetFields(map[string]interface{}{"manifest": string(body)}); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("portainer-kubernetes-custom-resource-%d-%s-%s-%s", endpointID, definition, namespace, name))
	return nil
}

func newKubernetesGPUDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_gpu", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"endpoint_id": k8sDSRequiredInt("Identifier of the Kubernetes environment to query."),
			// Computed attributes
			"gpu_node_count":         k8sDSComputedInt("Nodes in the cluster that report GPUs."),
			"degraded_count":         k8sDSComputedInt("GPU nodes Portainer considers degraded. A non-zero value is the one worth alerting on."),
			"operator_detected":      k8sDSComputedBool("Whether a GPU operator was found in the cluster."),
			"device_plugin_detected": k8sDSComputedBool("Whether a GPU device plugin was found in the cluster."),
			"total_capacity":         schema.MapAttribute{Computed: true, ElementType: types.Int64Type, Description: "Total GPU capacity across the cluster, keyed by resource name."},
			"total_allocatable":      schema.MapAttribute{Computed: true, ElementType: types.Int64Type, Description: "Total allocatable GPUs across the cluster, keyed by resource name."},
			"total_allocated":        schema.MapAttribute{Computed: true, ElementType: types.Int64Type, Description: "Total allocated GPUs across the cluster, keyed by resource name."},
			"nodes": k8sDSObjectList("The GPU nodes and what each of them holds.",
				k8sDSField{"name", types.StringType, "Name of the node."},
				k8sDSField{"status", types.StringType, "GPU status Portainer reports for the node."},
				k8sDSField{"status_reason", types.StringType, "Why the node is in that status, empty when it is healthy."},
				k8sDSField{"capacity", k8sDSIntMap, "GPU capacity of the node, keyed by resource name."},
				k8sDSField{"allocatable", k8sDSIntMap, "Allocatable GPUs on the node, keyed by resource name."},
				k8sDSField{"allocated", k8sDSIntMap, "Allocated GPUs on the node, keyed by resource name."},
			),
			"workloads": k8sDSObjectList("The workloads asking for GPUs, including any that cannot be scheduled.",
				k8sDSField{"pod_name", types.StringType, "Name of the pod."},
				k8sDSField{"namespace", types.StringType, "Namespace of the pod."},
				k8sDSField{"node_name", types.StringType, "Node the pod runs on, empty while it is unscheduled."},
				k8sDSField{"pod_phase", types.StringType, "Phase of the pod."},
				k8sDSField{"owner_kind", types.StringType, "Kind of the workload that owns the pod."},
				k8sDSField{"owner_name", types.StringType, "Name of the workload that owns the pod."},
				k8sDSField{"scheduling_issue", types.StringType, "Why the pod could not be scheduled, empty when it was."},
				k8sDSField{"gpu_requests", k8sDSIntMap, "GPUs the pod asks for, keyed by resource name."},
			),
		},
	}, nil, kubernetesGPURead)
}

// gpuQuantities turns the int64 maps Portainer reports into the int maps the
// schema takes.
func gpuQuantities(source map[string]int64) map[string]interface{} {
	quantities := make(map[string]interface{}, len(source))
	for name, value := range source {
		quantities[name] = int(value)
	}
	return quantities
}

func kubernetesGPURead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	endpointID := d.Get("endpoint_id").(int)

	var info struct {
		Summary struct {
			GPUNodeCount         int              `json:"GPUNodeCount"`
			DegradedCount        int              `json:"DegradedCount"`
			OperatorDetected     bool             `json:"OperatorDetected"`
			DevicePluginDetected bool             `json:"DevicePluginDetected"`
			TotalCapacity        map[string]int64 `json:"TotalCapacity"`
			TotalAllocatable     map[string]int64 `json:"TotalAllocatable"`
			TotalAllocated       map[string]int64 `json:"TotalAllocated"`
		} `json:"Summary"`
		Nodes []struct {
			Name         string           `json:"Name"`
			Status       string           `json:"Status"`
			StatusReason string           `json:"StatusReason"`
			Capacity     map[string]int64 `json:"Capacity"`
			Allocatable  map[string]int64 `json:"Allocatable"`
			Allocated    map[string]int64 `json:"Allocated"`
		} `json:"Nodes"`
		Workloads []struct {
			PodName         string           `json:"PodName"`
			Namespace       string           `json:"Namespace"`
			NodeName        string           `json:"NodeName"`
			PodPhase        string           `json:"PodPhase"`
			OwnerKind       string           `json:"OwnerKind"`
			OwnerName       string           `json:"OwnerName"`
			SchedulingIssue string           `json:"SchedulingIssue"`
			GPURequests     map[string]int64 `json:"GPURequests"`
		} `json:"Workloads"`
	}

	gpuURL := fmt.Sprintf("%s/kubernetes/%d/gpu", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodGet, gpuURL, nil, &info); err != nil {
		return fmt.Errorf("failed to read the GPU information of environment %d: %w", endpointID, err)
	}

	nodes := make([]interface{}, 0, len(info.Nodes))
	for _, node := range info.Nodes {
		nodes = append(nodes, map[string]interface{}{
			"name": node.Name, "status": node.Status, "status_reason": node.StatusReason,
			"capacity":    gpuQuantities(node.Capacity),
			"allocatable": gpuQuantities(node.Allocatable),
			"allocated":   gpuQuantities(node.Allocated),
		})
	}

	workloads := make([]interface{}, 0, len(info.Workloads))
	for _, workload := range info.Workloads {
		workloads = append(workloads, map[string]interface{}{
			"pod_name": workload.PodName, "namespace": workload.Namespace,
			"node_name": workload.NodeName, "pod_phase": workload.PodPhase,
			"owner_kind": workload.OwnerKind, "owner_name": workload.OwnerName,
			"scheduling_issue": workload.SchedulingIssue,
			"gpu_requests":     gpuQuantities(workload.GPURequests),
		})
	}

	if err := d.SetFields(map[string]interface{}{
		"gpu_node_count":         info.Summary.GPUNodeCount,
		"degraded_count":         info.Summary.DegradedCount,
		"operator_detected":      info.Summary.OperatorDetected,
		"device_plugin_detected": info.Summary.DevicePluginDetected,
		"total_capacity":         gpuQuantities(info.Summary.TotalCapacity),
		"total_allocatable":      gpuQuantities(info.Summary.TotalAllocatable),
		"total_allocated":        gpuQuantities(info.Summary.TotalAllocated),
		"nodes":                  nodes,
		"workloads":              workloads,
	}); err != nil {
		return err
	}
	d.SetID(fmt.Sprintf("portainer-kubernetes-gpu-%d", endpointID))
	return nil
}
