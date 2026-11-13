package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// dataSourceObservabilityEnvironment reports whether observability is on for
// an environment and which provider serves it - the address of the OneUptime
// instance an Observability policy deployed or connected to, for instance,
// which a configuration can hand to dashboards or links.
func dataSourceObservabilityEnvironment() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceObservabilityEnvironmentRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "ID of the environment to report on.",
			},
			// Computed attributes
			"enabled": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether observability is enabled for the environment.",
			},
			"provider_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Observability provider serving the environment, empty when none does.",
			},
			"url": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "URL of the observability provider.",
			},
			"project_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Project of the environment at the provider.",
			},
			"cluster_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Provider's internal identifier of the cluster (OneUptime's cluster ID, used for deep links). Empty when Portainer could not resolve it.",
			},
			"cluster_name": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Name of the cluster at the provider.",
			},
		},
	}
}

func dataSourceObservabilityEnvironmentRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID := d.Get("environment_id").(int)

	var resp struct {
		Enabled     bool   `json:"enabled"`
		Provider    string `json:"provider"`
		URL         string `json:"url"`
		ProjectID   string `json:"projectId"`
		ClusterID   string `json:"clusterId"`
		ClusterName string `json:"clusterName"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/observability/environments/%d", client.Endpoint, envID), nil, &resp); err != nil {
		// A 404 is also what a missing environment gets; explainUnsupported
		// only rewrites it when the server is too old or not BE.
		err = explainUnsupported(ctx, client, err, "portainer_observability_environment", requiresBE245)
		return diag.FromErr(fmt.Errorf("failed to read the observability of environment %d: %w", envID, err))
	}

	if err := setFields(d, map[string]interface{}{
		"enabled":       resp.Enabled,
		"provider_name": resp.Provider,
		"url":           resp.URL,
		"project_id":    resp.ProjectID,
		"cluster_id":    resp.ClusterID,
		"cluster_name":  resp.ClusterName,
	}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId(fmt.Sprintf("portainer-observability-environment-%d", envID))
	return nil
}
