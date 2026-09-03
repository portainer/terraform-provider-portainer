package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceEndpointsSummary() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEndpointsSummaryRead,

		Schema: map[string]*schema.Schema{
			// Computed attributes
			"total": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Total number of environments.",
			},
			"up": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Environments Portainer can currently reach.",
			},
			"down": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Environments Portainer cannot reach.",
			},
			"heartbeat": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Edge environments whose agent is checking in.",
			},
			"outdated": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Environments running an agent older than the Portainer server.",
			},
			"unassigned": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Environments still in the Unassigned group.",
			},
			"docker": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of Docker environments.",
			},
			"kubernetes": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of Kubernetes environments.",
			},
			"azure": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of Azure ACI environments.",
			},
			"podman": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Number of Podman environments.",
			},
			"by_group": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Environment count per environment group.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"group_id":   {Type: schema.TypeInt, Computed: true, Description: "Identifier of the environment group."},
						"group_name": {Type: schema.TypeString, Computed: true, Description: "Name of the environment group."},
						"count":      {Type: schema.TypeInt, Computed: true, Description: "Number of environments in the group."},
					},
				},
			},
		},
	}
}

func dataSourceEndpointsSummaryRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var summary struct {
		Total      int `json:"total"`
		Up         int `json:"up"`
		Down       int `json:"down"`
		Outdated   int `json:"outdated"`
		Unassigned int `json:"unassigned"`
		ByHealth   struct {
			Up        int `json:"up"`
			Down      int `json:"down"`
			Heartbeat int `json:"heartbeat"`
			Outdated  int `json:"outdated"`
		} `json:"byHealth"`
		ByPlatformType struct {
			Docker     int `json:"docker"`
			Kubernetes int `json:"kubernetes"`
			Azure      int `json:"azure"`
			Podman     int `json:"podman"`
		} `json:"byPlatformType"`
		ByGroup []struct {
			GroupID   int    `json:"groupId"`
			GroupName string `json:"groupName"`
			Count     int    `json:"count"`
		} `json:"byGroup"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/endpoints/summary", nil, &summary); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the environment summary: %w", err))
	}

	groups := make([]map[string]interface{}, len(summary.ByGroup))
	for i, g := range summary.ByGroup {
		groups[i] = map[string]interface{}{"group_id": g.GroupID, "group_name": g.GroupName, "count": g.Count}
	}

	// The health counters live under byHealth; the top-level up/down/outdated
	// fields repeat them, so the nested ones win when they are populated.
	up, down, outdated := summary.Up, summary.Down, summary.Outdated
	if summary.ByHealth.Up != 0 || summary.ByHealth.Down != 0 || summary.ByHealth.Outdated != 0 {
		up, down, outdated = summary.ByHealth.Up, summary.ByHealth.Down, summary.ByHealth.Outdated
	}

	if err := setFields(d, map[string]interface{}{
		"total": summary.Total, "up": up, "down": down,
		"heartbeat": summary.ByHealth.Heartbeat, "outdated": outdated,
		"unassigned": summary.Unassigned,
		"docker":     summary.ByPlatformType.Docker, "kubernetes": summary.ByPlatformType.Kubernetes,
		"azure": summary.ByPlatformType.Azure, "podman": summary.ByPlatformType.Podman,
		"by_group": groups,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("portainer-endpoints-summary")
	return nil
}
