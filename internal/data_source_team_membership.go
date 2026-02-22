package internal

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/team_memberships"
)

func dataSourceTeamMembership() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceTeamMembershipRead,

		Schema: map[string]*schema.Schema{
			"team_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"user_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"role": {
				Type:     schema.TypeInt,
				Computed: true,
			},
		},
	}
}

func dataSourceTeamMembershipRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	teamID := int64(d.Get("team_id").(int))
	userID := int64(d.Get("user_id").(int))

	params := team_memberships.NewTeamMembershipListParams()
	resp, err := client.Client.TeamMemberships.TeamMembershipList(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to fetch team memberships list: %w", err)
	}

	for _, m := range resp.Payload {
		if m.TeamID == teamID && m.UserID == userID {
			d.SetId(strconv.FormatInt(m.ID, 10))
			d.Set("role", m.Role)
			return nil
		}
	}

	return fmt.Errorf("team membership not found for team_id %d and user_id %d", teamID, userID)
}
