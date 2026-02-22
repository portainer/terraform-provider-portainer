package internal

import (
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceRegistryAccess() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceRegistryAccessRead,

		Schema: map[string]*schema.Schema{
			"registry_id": {
				Type:     schema.TypeInt,
				Required: true,
			},
			"endpoint_id": {
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

func dataSourceRegistryAccessRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	registryID := d.Get("registry_id").(int)
	endpointID := d.Get("endpoint_id").(int)
	teamID, hasTeam := d.GetOk("team_id")
	userID, hasUser := d.GetOk("user_id")

	if !hasTeam && !hasUser {
		return fmt.Errorf("either team_id or user_id must be provided")
	}

	policies, err := getRegistryPolicies(client, registryID, endpointID)
	if err != nil {
		if errors.Is(err, ErrRegistryNotFound) {
			return fmt.Errorf("registry %d not found", registryID)
		}
		return err
	}

	found := false
	idStr := fmt.Sprintf("%d/%d/", registryID, endpointID)

	if hasTeam {
		tidStr := strconv.Itoa(teamID.(int))
		if p, ok := policies.TeamAccessPolicies[tidStr]; ok {
			d.Set("role_id", int(p.RoleID))
			idStr += fmt.Sprintf("team/%s", tidStr)
			found = true
		}
	} else if hasUser {
		uidStr := strconv.Itoa(userID.(int))
		if p, ok := policies.UserAccessPolicies[uidStr]; ok {
			d.Set("role_id", int(p.RoleID))
			idStr += fmt.Sprintf("user/%s", uidStr)
			found = true
		}
	}

	if !found {
		return fmt.Errorf("access policy not found for the given team_id/user_id on registry %d and endpoint %d", registryID, endpointID)
	}

	d.SetId(idStr)
	return nil
}
