package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceSystem() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceSystemRead,

		Schema: map[string]*schema.Schema{
			// Computed attributes
			"instance_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Unique identifier of this Portainer instance.",
			},
			"version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Version of the running Portainer instance, as reported by its status endpoint.",
			},
			"server_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Server version reported by the version endpoint. Use it to gate configuration on the Portainer release, for example a resource that needs 2.45 or newer.",
			},
			"server_edition": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Portainer edition, `CE` or `BE`.",
			},
			"database_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Schema version of Portainer's database.",
			},
			"latest_version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Newest Portainer version available upstream, empty when the instance cannot reach the update service.",
			},
			"update_available": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether a newer Portainer version than the running one is available.",
			},
			"version_support": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Support status of the running version, such as `STS` or `LTS`.",
			},
			"agents": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of standard agents connected to this instance.",
			},
			"edge_agents": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of Edge agents connected to this instance.",
			},
			"platform": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Container platform Portainer itself runs on, such as `docker` or `kubernetes`.",
			},
			"nodes": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Total number of nodes across every environment, which is what Portainer licences against.",
			},
			"admin_initialized": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether an administrator account already exists. False means the instance is still awaiting its initial setup.",
			},
		},
	}
}

func dataSourceSystemRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// /status is the deprecated alias of /system/status and returns the same
	// object, so only the current path is called.
	var status struct {
		InstanceID string `json:"InstanceID"`
		Version    string `json:"Version"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/system/status", nil, &status); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the Portainer status: %w", err))
	}

	var version struct {
		ServerVersion   string `json:"ServerVersion"`
		ServerEdition   string `json:"ServerEdition"`
		DatabaseVersion string `json:"DatabaseVersion"`
		LatestVersion   string `json:"LatestVersion"`
		UpdateAvailable bool   `json:"UpdateAvailable"`
		VersionSupport  string `json:"VersionSupport"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/system/version", nil, &version); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the Portainer version: %w", err))
	}

	var info struct {
		Agents     int    `json:"agents"`
		EdgeAgents int    `json:"edgeAgents"`
		Platform   string `json:"platform"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/system/info", nil, &info); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the Portainer system info: %w", err))
	}

	var nodes struct {
		Nodes int `json:"nodes"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/system/nodes", nil, &nodes); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the Portainer node count: %w", err))
	}

	// The admin check answers 204 when an administrator exists and 404 when the
	// instance is still uninitialised, so the status code is the answer.
	adminErr := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/users/admin/check", nil, nil)
	adminInitialized := adminErr == nil
	if adminErr != nil && !isAPINotFound(adminErr) {
		return diag.FromErr(fmt.Errorf("failed to check for an administrator account: %w", adminErr))
	}

	if err := setFields(d, map[string]interface{}{
		"instance_id":       status.InstanceID,
		"version":           status.Version,
		"server_version":    version.ServerVersion,
		"server_edition":    version.ServerEdition,
		"database_version":  version.DatabaseVersion,
		"latest_version":    version.LatestVersion,
		"update_available":  version.UpdateAvailable,
		"version_support":   version.VersionSupport,
		"agents":            info.Agents,
		"edge_agents":       info.EdgeAgents,
		"platform":          info.Platform,
		"nodes":             nodes.Nodes,
		"admin_initialized": adminInitialized,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("portainer-system-" + status.InstanceID)
	return nil
}
