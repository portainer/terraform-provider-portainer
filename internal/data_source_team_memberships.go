package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceTeamMemberships() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceTeamMembershipsRead,

		Schema: map[string]*schema.Schema{
			"team_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the team whose members are listed.",
			},
			// Computed attributes
			"memberships": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Members of the team.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id":      {Type: schema.TypeInt, Computed: true, Description: "Identifier of the membership record."},
						"user_id": {Type: schema.TypeInt, Computed: true, Description: "Identifier of the user."},
						"role":    {Type: schema.TypeInt, Computed: true, Description: "Role within the team: 1 = team leader, 2 = team member."},
					},
				},
			},
			"leader_user_ids": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "Identifiers of the team's leaders, who may manage its membership.",
			},
		},
	}
}

func dataSourceTeamMembershipsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	teamID := d.Get("team_id").(int)

	var list []struct {
		ID     int `json:"Id"`
		UserID int `json:"UserID"`
		Role   int `json:"Role"`
	}
	url := fmt.Sprintf("%s/teams/%d/memberships", client.Endpoint, teamID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &list); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the memberships of team %d: %w", teamID, err))
	}

	// Role 1 is the team leader in Portainer's membership enum.
	const roleTeamLeader = 1

	memberships := make([]map[string]interface{}, len(list))
	leaders := []int{}
	for i, m := range list {
		memberships[i] = map[string]interface{}{"id": m.ID, "user_id": m.UserID, "role": m.Role}
		if m.Role == roleTeamLeader {
			leaders = append(leaders, m.UserID)
		}
	}

	if err := setFields(d, map[string]interface{}{
		"memberships":     memberships,
		"leader_user_ids": leaders,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(teamID) + "/memberships")
	return nil
}
