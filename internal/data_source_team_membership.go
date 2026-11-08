package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceTeamMembership() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceTeamMembershipRead,

		Schema: map[string]*schema.Schema{
			"team_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer team the membership belongs to. Combined with user_id to find a specific membership.",
			},
			"user_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer user whose membership in the team is being looked up.",
			},
			"role": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Role of the user within the team as returned by Portainer (e.g. team leader, member).",
			},
		},
	}
}

func dataSourceTeamMembershipRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	teamID := int64(d.Get("team_id").(int))
	userID := int64(d.Get("user_id").(int))

	memberships, err := listTeamMemberships(ctx, client)
	if err != nil {
		return diag.FromErr(err)
	}

	for _, m := range memberships {
		if m.TeamID == teamID && m.UserID == userID {
			d.SetId(strconv.FormatInt(m.ID, 10))
			if err := d.Set("role", m.Role); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("team membership not found for team_id %d and user_id %d", teamID, userID))
}
