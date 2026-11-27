package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
)

func newKubernetesClusterDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_cluster", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"environment_id": k8sDSRequiredInt("Environment (endpoint) identifier of the Kubernetes environment to describe."),
			// Computed attributes
			"version":              k8sDSComputedString("Full Kubernetes version of the cluster, as reported by the API server (for example `v1.31.4+k3s1`)."),
			"major":                k8sDSComputedString("Major version component of the cluster."),
			"minor":                k8sDSComputedString("Minor version component of the cluster."),
			"platform":             k8sDSComputedString("Platform the API server runs on, such as `linux/amd64`."),
			"supports_pod_restart": k8sDSComputedBool("Whether the cluster is new enough for Portainer's pod restart action."),
			"rbac_enabled":         k8sDSComputedBool("Whether RBAC is enabled on the cluster. Roles, role bindings and namespace access policies only take effect when it is."),
			"applications_count":   k8sDSComputedInt("Number of applications Portainer sees in the cluster."),
			"namespaces_count":     k8sDSComputedInt("Number of namespaces visible to the API token."),
			"services_count":       k8sDSComputedInt("Number of services in the cluster."),
			"ingresses_count":      k8sDSComputedInt("Number of ingresses in the cluster."),
			"config_maps_count":    k8sDSComputedInt("Number of ConfigMaps in the cluster."),
			"secrets_count":        k8sDSComputedInt("Number of secrets in the cluster."),
			"volumes_count":        k8sDSComputedInt("Number of volumes in the cluster."),
			"max_cpu":              k8sDSComputedInt("Largest CPU allocation a single workload could still request, in millicores, across the cluster's nodes."),
			"max_memory":           k8sDSComputedInt("Largest memory allocation a single workload could still request, in bytes, across the cluster's nodes."),
		},
	}, nil, kubernetesClusterRead)
}

func kubernetesClusterRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
	envID := d.Get("environment_id").(int)

	var version struct {
		GitVersion         string `json:"gitVersion"`
		Major              string `json:"major"`
		Minor              string `json:"minor"`
		Platform           string `json:"platform"`
		SupportsPodRestart bool   `json:"supportsPodRestart"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/kubernetes/%d/version", client.Endpoint, envID), nil, &version); err != nil {
		return fmt.Errorf("failed to read the Kubernetes version of environment %d: %w", envID, err)
	}

	// The endpoint answers with a bare JSON boolean rather than an object.
	var rbacEnabled bool
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/kubernetes/%d/rbac_enabled", client.Endpoint, envID), nil, &rbacEnabled); err != nil {
		return fmt.Errorf("failed to read the RBAC status of environment %d: %w", envID, err)
	}

	// Portainer's API specification types this response as a list, but the
	// server answers with a single object. Both shapes are accepted rather
	// than trusting either one: a decode failure here used to take the whole
	// data source down over a handful of counters.
	type dashboardCounters struct {
		ApplicationsCount int `json:"applicationsCount"`
		NamespacesCount   int `json:"namespacesCount"`
		ServicesCount     int `json:"servicesCount"`
		IngressesCount    int `json:"ingressesCount"`
		ConfigMapsCount   int `json:"configMapsCount"`
		SecretsCount      int `json:"secretsCount"`
		VolumesCount      int `json:"volumesCount"`
	}

	dashboardURL := fmt.Sprintf("%s/kubernetes/%d/dashboard", client.Endpoint, envID)
	body, err := apiGETRaw(ctx, client, dashboardURL)
	if err != nil {
		return fmt.Errorf("failed to read the Kubernetes dashboard of environment %d: %w", envID, err)
	}

	var dashboard []dashboardCounters
	var single dashboardCounters
	if err := json.Unmarshal(body, &single); err == nil {
		dashboard = []dashboardCounters{single}
	} else if err := json.Unmarshal(body, &dashboard); err != nil {
		return fmt.Errorf("failed to parse the Kubernetes dashboard of environment %d: %w", envID, err)
	}

	var maxLimits struct {
		CPU    int `json:"CPU"`
		Memory int `json:"Memory"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/kubernetes/%d/max_resource_limits", client.Endpoint, envID), nil, &maxLimits); err != nil {
		return fmt.Errorf("failed to read the resource limits of environment %d: %w", envID, err)
	}

	fields := map[string]interface{}{
		"version":              version.GitVersion,
		"major":                version.Major,
		"minor":                version.Minor,
		"platform":             version.Platform,
		"supports_pod_restart": version.SupportsPodRestart,
		"rbac_enabled":         rbacEnabled,
		"max_cpu":              maxLimits.CPU,
		"max_memory":           maxLimits.Memory,
	}
	if len(dashboard) > 0 {
		dash := dashboard[0]
		fields["applications_count"] = dash.ApplicationsCount
		fields["namespaces_count"] = dash.NamespacesCount
		fields["services_count"] = dash.ServicesCount
		fields["ingresses_count"] = dash.IngressesCount
		fields["config_maps_count"] = dash.ConfigMapsCount
		fields["secrets_count"] = dash.SecretsCount
		fields["volumes_count"] = dash.VolumesCount
	}
	if err := d.SetFields(fields); err != nil {
		return err
	}

	d.SetID(strconv.Itoa(envID) + "/cluster")
	return nil
}
