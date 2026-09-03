package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceKubernetesCluster() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesClusterRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier of the Kubernetes environment to describe.",
			},
			// Computed attributes
			"version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Full Kubernetes version of the cluster, as reported by the API server (for example `v1.31.4+k3s1`).",
			},
			"major": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Major version component of the cluster.",
			},
			"minor": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Minor version component of the cluster.",
			},
			"platform": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Platform the API server runs on, such as `linux/amd64`.",
			},
			"supports_pod_restart": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the cluster is new enough for Portainer's pod restart action.",
			},
			"rbac_enabled": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether RBAC is enabled on the cluster. Roles, role bindings and namespace access policies only take effect when it is.",
			},
			"applications_count": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of applications Portainer sees in the cluster.",
			},
			"namespaces_count": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of namespaces visible to the API token.",
			},
			"services_count": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of services in the cluster.",
			},
			"ingresses_count": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of ingresses in the cluster.",
			},
			"config_maps_count": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of ConfigMaps in the cluster.",
			},
			"secrets_count": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of secrets in the cluster.",
			},
			"volumes_count": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of volumes in the cluster.",
			},
			"max_cpu": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Largest CPU allocation a single workload could still request, in millicores, across the cluster's nodes.",
			},
			"max_memory": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Largest memory allocation a single workload could still request, in bytes, across the cluster's nodes.",
			},
		},
	}
}

func dataSourceKubernetesClusterRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID := d.Get("environment_id").(int)

	var version struct {
		GitVersion         string `json:"gitVersion"`
		Major              string `json:"major"`
		Minor              string `json:"minor"`
		Platform           string `json:"platform"`
		SupportsPodRestart bool   `json:"supportsPodRestart"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/kubernetes/%d/version", client.Endpoint, envID), nil, &version); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the Kubernetes version of environment %d: %w", envID, err))
	}

	// The endpoint answers with a bare JSON boolean rather than an object.
	var rbacEnabled bool
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/kubernetes/%d/rbac_enabled", client.Endpoint, envID), nil, &rbacEnabled); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the RBAC status of environment %d: %w", envID, err))
	}

	// The dashboard endpoint returns a single-element list of counters.
	var dashboard []struct {
		ApplicationsCount int `json:"applicationsCount"`
		NamespacesCount   int `json:"namespacesCount"`
		ServicesCount     int `json:"servicesCount"`
		IngressesCount    int `json:"ingressesCount"`
		ConfigMapsCount   int `json:"configMapsCount"`
		SecretsCount      int `json:"secretsCount"`
		VolumesCount      int `json:"volumesCount"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/kubernetes/%d/dashboard", client.Endpoint, envID), nil, &dashboard); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the Kubernetes dashboard of environment %d: %w", envID, err))
	}

	var maxLimits struct {
		CPU    int `json:"CPU"`
		Memory int `json:"Memory"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/kubernetes/%d/max_resource_limits", client.Endpoint, envID), nil, &maxLimits); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the resource limits of environment %d: %w", envID, err))
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
	if err := setFields(d, fields); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(envID) + "/cluster")
	return nil
}
