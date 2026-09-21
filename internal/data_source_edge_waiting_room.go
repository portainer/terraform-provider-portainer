package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// dataSourceEdgeWaitingRoom lists the edge environments that have called home
// but have not been trusted yet - what the Portainer UI calls the waiting
// room. It is the companion to portainer_endpoint_trust: the environments are
// created by the agents, so their identifiers are not known to a configuration
// until they are looked up.
func dataSourceEdgeWaitingRoom() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEdgeWaitingRoomRead,

		Schema: map[string]*schema.Schema{
			"group_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Only list environments in this endpoint group. Leave unset to list the whole waiting room.",
			},
			// Computed attributes
			"endpoint_ids": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "Identifiers of the environments awaiting trust, which is what `portainer_endpoint_trust` takes.",
			},
			"environments": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The environments awaiting trust.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Identifier of the environment.",
						},
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the environment.",
						},
						"type": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Type of the environment: 4 = Edge Agent, 7 = Kubernetes Edge Agent.",
						},
						"edge_id": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Edge identifier the agent reports.",
						},
						"group_id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Identifier of the endpoint group the environment currently sits in.",
						},
						"last_check_in": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Unix timestamp of the agent's most recent check-in, which is how to tell a live agent from an abandoned entry.",
						},
					},
				},
			},
		},
	}
}

func dataSourceEdgeWaitingRoomRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	url := client.Endpoint + "/endpoints?edgeDeviceUntrusted=true"
	if v, ok := d.GetOk("group_id"); ok && v.(int) != 0 {
		url += fmt.Sprintf("&groupIds=%d", v.(int))
	}

	var endpoints []struct {
		ID            int    `json:"Id"`
		Name          string `json:"Name"`
		Type          int    `json:"Type"`
		EdgeID        string `json:"EdgeID"`
		GroupID       int    `json:"GroupId"`
		LastCheckInAt int64  `json:"LastCheckInDate"`
	}
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &endpoints); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the edge environments awaiting trust: %w", err))
	}

	ids := make([]interface{}, 0, len(endpoints))
	environments := make([]interface{}, 0, len(endpoints))
	for _, endpoint := range endpoints {
		ids = append(ids, endpoint.ID)
		environments = append(environments, map[string]interface{}{
			"id":            endpoint.ID,
			"name":          endpoint.Name,
			"type":          endpoint.Type,
			"edge_id":       endpoint.EdgeID,
			"group_id":      endpoint.GroupID,
			"last_check_in": int(endpoint.LastCheckInAt),
		})
	}

	if err := setFields(d, map[string]interface{}{
		"endpoint_ids": ids,
		"environments": environments,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("portainer-edge-waiting-room")
	return nil
}
