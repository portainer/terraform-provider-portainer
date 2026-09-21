package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// The custom resource endpoints are read-only here on purpose. Portainer
// exposes list, inspect and delete for them but no create or update: custom
// resources come from applying manifests, which the provider already does
// through portainer_kubernetes_manifest. A resource that could only delete
// what something else created would have no lifecycle Terraform can manage.

func dataSourceKubernetesCustomResourceDefinitions() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesCustomResourceDefinitionsRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Kubernetes environment to query.",
			},
			// Computed attributes
			"definitions": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The custom resource definitions installed in the cluster.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the definition, which is what `portainer_kubernetes_custom_resources` takes as its `definition`.",
						},
						"group": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "API group the definition belongs to.",
						},
						"scope": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Whether the resources are `Namespaced` or `Cluster` scoped.",
						},
						"creation_date": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "When the definition was created.",
						},
						"release_name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Helm release that installed the definition, empty when it was not installed by one.",
						},
						"release_namespace": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Namespace of the Helm release that installed the definition.",
						},
						"release_version": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Version of the Helm release that installed the definition.",
						},
					},
				},
			},
		},
	}
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

func dataSourceKubernetesCustomResourceDefinitionsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	var definitions []k8sCustomResourceDefinition
	listURL := fmt.Sprintf("%s/kubernetes/%d/customresourcedefinitions", client.Endpoint, endpointID)
	if err := doJSON(ctx, client, http.MethodGet, listURL, nil, &definitions); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the custom resource definitions of environment %d: %w", endpointID, err))
	}

	entries := make([]interface{}, 0, len(definitions))
	for _, definition := range definitions {
		entries = append(entries, definition.fields())
	}

	if err := setFields(d, map[string]interface{}{"definitions": entries}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-kubernetes-crds-%d", endpointID))
	return nil
}

func dataSourceKubernetesCustomResourceDefinition() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesCustomResourceDefinitionRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Kubernetes environment to query.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the custom resource definition to inspect.",
			},
			// Computed attributes
			"group": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "API group the definition belongs to.",
			},
			"scope": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Whether the resources are `Namespaced` or `Cluster` scoped. This is what decides whether `portainer_kubernetes_custom_resource` needs a namespace.",
			},
			"creation_date": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "When the definition was created.",
			},
			"release_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Helm release that installed the definition, empty when it was not installed by one.",
			},
			"release_namespace": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Namespace of the Helm release that installed the definition.",
			},
			"release_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Version of the Helm release that installed the definition.",
			},
		},
	}
}

func dataSourceKubernetesCustomResourceDefinitionRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	name := d.Get("name").(string)

	var definition k8sCustomResourceDefinition
	inspectURL := fmt.Sprintf("%s/kubernetes/%d/customresourcedefinitions/%s",
		client.Endpoint, endpointID, url.PathEscape(name))
	if err := doJSON(ctx, client, http.MethodGet, inspectURL, nil, &definition); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read custom resource definition %s: %w", name, err))
	}

	fields := definition.fields()
	// The name is an argument here, so it is not written back from a response
	// that may spell it differently.
	delete(fields, "name")
	if err := setFields(d, fields); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-kubernetes-crd-%d-%s", endpointID, name))
	return nil
}

func dataSourceKubernetesCustomResources() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesCustomResourcesRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Kubernetes environment to query.",
			},
			"definition": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the custom resource definition whose resources are listed, as reported by `portainer_kubernetes_custom_resource_definitions`.",
			},
			// Computed attributes
			"resources": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The custom resources of that definition.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the resource.",
						},
						"namespace": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Namespace of the resource, empty for a cluster-scoped one.",
						},
						"definition_name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Definition the resource belongs to.",
						},
						"uid": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Kubernetes UID of the resource.",
						},
						"creation_date": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "When the resource was created.",
						},
					},
				},
			},
		},
	}
}

func dataSourceKubernetesCustomResourcesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
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
		return diag.FromErr(fmt.Errorf("failed to list the custom resources of definition %s: %w", definition, err))
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

	if err := setFields(d, map[string]interface{}{"resources": entries}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-kubernetes-custom-resources-%d-%s", endpointID, definition))
	return nil
}

func dataSourceKubernetesCustomResource() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesCustomResourceRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Kubernetes environment to query.",
			},
			"definition": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the custom resource definition the resource belongs to.",
			},
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the resource to fetch.",
			},
			"namespace": {
				Type:     schema.TypeString,
				Optional: true,
				// Portainer has a separate path for cluster-scoped resources.
				// Leaving the namespace out is what selects it, which mirrors
				// the definition's own `scope`.
				Description: "Namespace of the resource. Leave unset for a resource whose definition is `Cluster` scoped.",
			},
			// Computed attributes
			"manifest": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "The resource as Portainer returns it, encoded as JSON. Decode it with `jsondecode()` to read individual fields.",
			},
		},
	}
}

func dataSourceKubernetesCustomResourceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
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
		return diag.FromErr(fmt.Errorf("failed to read custom resource %s: %w", name, err))
	}

	if err := setFields(d, map[string]interface{}{"manifest": string(body)}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-kubernetes-custom-resource-%d-%s-%s-%s", endpointID, definition, namespace, name))
	return nil
}

func dataSourceKubernetesGPU() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesGPURead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Kubernetes environment to query.",
			},
			// Computed attributes
			"gpu_node_count": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Nodes in the cluster that report GPUs.",
			},
			"degraded_count": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "GPU nodes Portainer considers degraded. A non-zero value is the one worth alerting on.",
			},
			"operator_detected": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether a GPU operator was found in the cluster.",
			},
			"device_plugin_detected": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether a GPU device plugin was found in the cluster.",
			},
			"total_capacity": {
				Type:        schema.TypeMap,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "Total GPU capacity across the cluster, keyed by resource name.",
			},
			"total_allocatable": {
				Type:        schema.TypeMap,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "Total allocatable GPUs across the cluster, keyed by resource name.",
			},
			"total_allocated": {
				Type:        schema.TypeMap,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "Total allocated GPUs across the cluster, keyed by resource name.",
			},
			"nodes": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The GPU nodes and what each of them holds.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the node.",
						},
						"status": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "GPU status Portainer reports for the node.",
						},
						"status_reason": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Why the node is in that status, empty when it is healthy.",
						},
						"capacity": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeInt},
							Description: "GPU capacity of the node, keyed by resource name.",
						},
						"allocatable": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeInt},
							Description: "Allocatable GPUs on the node, keyed by resource name.",
						},
						"allocated": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeInt},
							Description: "Allocated GPUs on the node, keyed by resource name.",
						},
					},
				},
			},
			"workloads": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The workloads asking for GPUs, including any that cannot be scheduled.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"pod_name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the pod.",
						},
						"namespace": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Namespace of the pod.",
						},
						"node_name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Node the pod runs on, empty while it is unscheduled.",
						},
						"pod_phase": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Phase of the pod.",
						},
						"owner_kind": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Kind of the workload that owns the pod.",
						},
						"owner_name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the workload that owns the pod.",
						},
						"scheduling_issue": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Why the pod could not be scheduled, empty when it was.",
						},
						"gpu_requests": {
							Type:        schema.TypeMap,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeInt},
							Description: "GPUs the pod asks for, keyed by resource name.",
						},
					},
				},
			},
		},
	}
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

func dataSourceKubernetesGPURead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
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
		return diag.FromErr(fmt.Errorf("failed to read the GPU information of environment %d: %w", endpointID, err))
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

	if err := setFields(d, map[string]interface{}{
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
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-kubernetes-gpu-%d", endpointID))
	return nil
}
