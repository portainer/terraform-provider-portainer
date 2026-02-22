package internal

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceEndpointGroupAccess() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceEndpointGroupAccessRead,

		Schema: map[string]*schema.Schema{
			"endpoint_group_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"team_id": {
				Type:     schema.TypeInt,
				Optional: true,
			},
			"user_id": {
				Type:     schema.TypeInt,
				Optional: true,
			},
			"role_id": {
				Type:     schema.TypeInt,
				Computed: true,
			},
		},
	}
}

func dataSourceEndpointGroupAccessRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointGroupID := d.Get("endpoint_group_id").(int)
	teamID, hasTeam := d.GetOk("team_id")
	userID, hasUser := d.GetOk("user_id")

	if !hasTeam && !hasUser {
		return fmt.Errorf("either team_id or user_id must be provided")
	}

	policies, err := getEndpointGroupPolicies(client, endpointGroupID)
	if err != nil {
		if errors.Is(err, ErrEndpointGroupNotFound) {
			return fmt.Errorf("endpoint group %d not found", endpointGroupID)
		}
		return err
	}

	found := false
	idStr := fmt.Sprintf("%d/", endpointGroupID)

	if hasTeam {
		tidStr := strconv.Itoa(teamID.(int))
		if p, ok := policies.TeamAccessPolicies[tidStr]; ok {
			d.Set("role_id", p["RoleId"])
			idStr += fmt.Sprintf("team/%s", tidStr)
			found = true
		}
	} else if hasUser {
		uidStr := strconv.Itoa(userID.(int))
		if p, ok := policies.UserAccessPolicies[uidStr]; ok {
			d.Set("role_id", p["RoleId"])
			idStr += fmt.Sprintf("user/%s", uidStr)
			found = true
		}
	}

	if !found {
		return fmt.Errorf("access policy not found for the given team_id/user_id on endpoint group %d", endpointGroupID)
	}

	d.SetId(idStr)
	return nil
}
