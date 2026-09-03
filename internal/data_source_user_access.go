package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceUserAccess() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceUserAccessRead,

		Schema: map[string]*schema.Schema{
			"user_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Identifier of the user to inspect. Leave unset to describe the user the provider authenticates as.",
			},
			// Computed attributes
			"username": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Username of the inspected user.",
			},
			"role": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Portainer role of the user: 1 = administrator, 2 = standard user.",
			},
			"memberships": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Team memberships of the user.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id":      {Type: schema.TypeInt, Computed: true, Description: "Identifier of the membership record."},
						"team_id": {Type: schema.TypeInt, Computed: true, Description: "Identifier of the team."},
						"role":    {Type: schema.TypeInt, Computed: true, Description: "Role within the team: 1 = team leader, 2 = team member."},
					},
				},
			},
			"effective_access": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Access the user effectively has on every environment, with the policy each entry comes from. This is the resolved result of user policies, team policies and group inheritance — the answer to \"why can this person see that environment\".",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"endpoint_id":     {Type: schema.TypeInt, Computed: true, Description: "Identifier of the environment."},
						"endpoint_name":   {Type: schema.TypeString, Computed: true, Description: "Name of the environment."},
						"group_id":        {Type: schema.TypeInt, Computed: true, Description: "Identifier of the environment group the access was inherited from, zero when it was granted directly."},
						"group_name":      {Type: schema.TypeString, Computed: true, Description: "Name of that environment group."},
						"team_id":         {Type: schema.TypeInt, Computed: true, Description: "Identifier of the team the access came through, zero when granted to the user directly."},
						"team_name":       {Type: schema.TypeString, Computed: true, Description: "Name of that team."},
						"role_id":         {Type: schema.TypeInt, Computed: true, Description: "Identifier of the effective role."},
						"role_name":       {Type: schema.TypeString, Computed: true, Description: "Name of the effective role, such as `Environment administrator`."},
						"role_priority":   {Type: schema.TypeInt, Computed: true, Description: "Priority of the role, which is how Portainer resolves a conflict between several grants."},
						"access_location": {Type: schema.TypeString, Computed: true, Description: "Where the access was granted: the environment itself, its group, or a team policy."},
					},
				},
			},
		},
	}
}

func dataSourceUserAccessRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var user struct {
		ID       int    `json:"Id"`
		Username string `json:"Username"`
		Role     int    `json:"Role"`
	}

	userID, explicit := d.GetOk("user_id")
	if explicit {
		url := fmt.Sprintf("%s/users/%d", client.Endpoint, userID.(int))
		if err := doJSON(ctx, client, http.MethodGet, url, nil, &user); err != nil {
			return diag.FromErr(fmt.Errorf("failed to read user %d: %w", userID.(int), err))
		}
	} else {
		// Without an explicit ID the endpoint resolves the caller, which is how
		// a configuration learns which account its API key belongs to.
		if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/users/me", nil, &user); err != nil {
			return diag.FromErr(fmt.Errorf("failed to read the current user: %w", err))
		}
	}
	if user.ID == 0 {
		return diag.FromErr(fmt.Errorf("failed to resolve the user: Portainer returned no identifier"))
	}

	var rawMemberships []struct {
		ID     int `json:"Id"`
		TeamID int `json:"TeamID"`
		Role   int `json:"Role"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/users/%d/memberships", client.Endpoint, user.ID), nil, &rawMemberships); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the memberships of user %d: %w", user.ID, err))
	}
	memberships := make([]map[string]interface{}, len(rawMemberships))
	for i, m := range rawMemberships {
		memberships[i] = map[string]interface{}{"id": m.ID, "team_id": m.TeamID, "role": m.Role}
	}

	var rawAccess []struct {
		EndpointID     int    `json:"endpointId"`
		EndpointName   string `json:"endpointName"`
		GroupID        int    `json:"groupId"`
		GroupName      string `json:"groupName"`
		TeamID         int    `json:"teamId"`
		TeamName       string `json:"teamName"`
		RoleID         int    `json:"roleId"`
		RoleName       string `json:"roleName"`
		RolePriority   int    `json:"rolePriority"`
		AccessLocation string `json:"accessLocation"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/users/%d/effective-access", client.Endpoint, user.ID), nil, &rawAccess); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the effective access of user %d: %w", user.ID, err))
	}
	access := make([]map[string]interface{}, len(rawAccess))
	for i, a := range rawAccess {
		access[i] = map[string]interface{}{
			"endpoint_id": a.EndpointID, "endpoint_name": a.EndpointName,
			"group_id": a.GroupID, "group_name": a.GroupName,
			"team_id": a.TeamID, "team_name": a.TeamName,
			"role_id": a.RoleID, "role_name": a.RoleName, "role_priority": a.RolePriority,
			"access_location": a.AccessLocation,
		}
	}

	if err := setFields(d, map[string]interface{}{
		"user_id":          user.ID,
		"username":         user.Username,
		"role":             user.Role,
		"memberships":      memberships,
		"effective_access": access,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(user.ID) + "/access")
	return nil
}
