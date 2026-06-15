package internal

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceEndpointGroupAccess() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEndpointGroupAccessRead,

		Schema: map[string]*schema.Schema{
			"endpoint_group_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer endpoint group whose access policy is being looked up.",
			},
			"team_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Identifier of the Portainer team for which the access policy on the endpoint group is returned. Either team_id or user_id must be provided.",
			},
			"user_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Identifier of the Portainer user for which the access policy on the endpoint group is returned. Either team_id or user_id must be provided.",
			},
			"role_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Identifier of the Portainer role granted to the team or user on the endpoint group.",
			},
		},
	}
}

func dataSourceEndpointGroupAccessRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointGroupID := d.Get("endpoint_group_id").(int)
	teamID, hasTeam := d.GetOk("team_id")
	userID, hasUser := d.GetOk("user_id")

	if !hasTeam && !hasUser {
		return diag.FromErr(fmt.Errorf("either team_id or user_id must be provided"))
	}

	policies, err := getEndpointGroupPolicies(ctx, client, endpointGroupID)
	if err != nil {
		if errors.Is(err, ErrEndpointGroupNotFound) {
			return diag.FromErr(fmt.Errorf("endpoint group %d not found", endpointGroupID))
		}
		return diag.FromErr(err)
	}

	found := false
	idStr := fmt.Sprintf("%d/", endpointGroupID)

	if hasTeam {
		tidStr := strconv.Itoa(teamID.(int))
		if p, ok := policies.TeamAccessPolicies[tidStr]; ok {
			if err := d.Set("role_id", p["RoleId"]); err != nil {
				return diag.FromErr(err)
			}
			idStr += fmt.Sprintf("team/%s", tidStr)
			found = true
		}
	} else if hasUser {
		uidStr := strconv.Itoa(userID.(int))
		if p, ok := policies.UserAccessPolicies[uidStr]; ok {
			if err := d.Set("role_id", p["RoleId"]); err != nil {
				return diag.FromErr(err)
			}
			idStr += fmt.Sprintf("user/%s", uidStr)
			found = true
		}
	}

	if !found {
		return diag.FromErr(fmt.Errorf("access policy not found for the given team_id/user_id on endpoint group %d", endpointGroupID))
	}

	d.SetId(idStr)
	return nil
}
