package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var ErrEndpointGroupNotFound = errors.New("endpoint group not found")

func resourceEndpointGroupAccess() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceEndpointGroupAccessCreate,
		ReadContext:   resourceEndpointGroupAccessRead,
		UpdateContext: resourceEndpointGroupAccessUpdate,
		DeleteContext: resourceEndpointGroupAccessDelete,

		Schema: map[string]*schema.Schema{
			"endpoint_group_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer endpoint group to which the access policy applies.",
			},
			"team_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    true,
				Description: "Identifier of the team granted access to the endpoint group. Mutually exclusive with `user_id`.",
			},
			"user_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    true,
				Description: "Identifier of the user granted access to the endpoint group. Mutually exclusive with `team_id`.",
			},
			"role_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     0,
				Description: "Role identifier assigned to the user or team for this endpoint group.",
			},
		},
	}
}

type EndpointGroupAccessPolicies struct {
	UserAccessPolicies map[string]map[string]int `json:"UserAccessPolicies"`
	TeamAccessPolicies map[string]map[string]int `json:"TeamAccessPolicies"`
}

func getEndpointGroupPolicies(ctx context.Context, client *APIClient, endpointGroupID int) (*EndpointGroupAccessPolicies, error) {
	var policies EndpointGroupAccessPolicies
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/endpoint_groups/%d", client.Endpoint, endpointGroupID), nil, &policies); err != nil {
		if isAPINotFound(err) {
			return nil, ErrEndpointGroupNotFound
		}
		return nil, fmt.Errorf("failed to fetch endpoint group: %w", err)
	}

	if policies.UserAccessPolicies == nil {
		policies.UserAccessPolicies = make(map[string]map[string]int)
	}
	if policies.TeamAccessPolicies == nil {
		policies.TeamAccessPolicies = make(map[string]map[string]int)
	}

	return &policies, nil
}

func resourceEndpointGroupAccessCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointGroupID := d.Get("endpoint_group_id").(int)
	teamID, hasTeam := d.GetOk("team_id")
	userID, hasUser := d.GetOk("user_id")
	roleID := d.Get("role_id").(int)

	if !hasTeam && !hasUser {
		return diag.FromErr(fmt.Errorf("either team_id or user_id must be provided"))
	}

	policies, err := getEndpointGroupPolicies(ctx, client, endpointGroupID)
	if err != nil {
		return diag.FromErr(err)
	}

	if hasTeam {
		tidStr := strconv.Itoa(teamID.(int))
		policies.TeamAccessPolicies[tidStr] = map[string]int{"RoleId": roleID}
	}
	if hasUser {
		uidStr := strconv.Itoa(userID.(int))
		policies.UserAccessPolicies[uidStr] = map[string]int{"RoleId": roleID}
	}

	// For Update, we need to send the full object (or at least the fields we want to update, usually Portainer PUT replaces the object or merges, but commonly we need to be careful).
	// However, the PUT /endpoint_groups/{id} endpoint documentation (or standard behavior) suggests we can just send the fields we want to update if it's a merge, OR we might need to send everything.
	// But `resource_endpoint_group.go` sends `name`, `description`, `tagIDs`.
	// If we only send access policies, we might wipe out name/desc/tags if the API is a full replacement.
	// BUT, typically Portainer API is partial update for some endpoints.
	// Let's check `resource_endpoint_group.go` again. It sends what it has in state.
	// Here we are a separate resource. If we do a PUT with JUST access policies, we risk clearing other fields if it's a replace.
	// Ideally we should read the FULL object and write it back with updated policies.
	// `getEndpointGroupPolicies` only reads policies.
	// Let's modify `getEndpointGroupPolicies` or the Create function to read 'Current State' as a generic map or struct to preserve other fields.

	// Better approach: Read the full JSON into a map[string]interface{}, modify the policies, and write it back.
	fullObject, err := getEndpointGroupMap(ctx, client, endpointGroupID)
	if err != nil {
		return diag.FromErr(err)
	}

	// Modify policies in the map
	userPolicies := make(map[string]interface{})
	if up, ok := fullObject["UserAccessPolicies"].(map[string]interface{}); ok {
		userPolicies = up
	}
	teamPolicies := make(map[string]interface{})
	if tp, ok := fullObject["TeamAccessPolicies"].(map[string]interface{}); ok {
		teamPolicies = tp
	}

	if hasTeam {
		tidStr := strconv.Itoa(teamID.(int))
		teamPolicies[tidStr] = map[string]int{"RoleId": roleID}
	}
	if hasUser {
		uidStr := strconv.Itoa(userID.(int))
		userPolicies[uidStr] = map[string]int{"RoleId": roleID}
	}

	fullObject["UserAccessPolicies"] = userPolicies
	fullObject["TeamAccessPolicies"] = teamPolicies

	return diag.FromErr(updateEndpointGroup(ctx, client, endpointGroupID, fullObject, d, hasTeam, teamID, hasUser, userID))
}

func getEndpointGroupMap(ctx context.Context, client *APIClient, endpointGroupID int) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/endpoint_groups/%d", client.Endpoint, endpointGroupID), nil, &result); err != nil {
		if isAPINotFound(err) {
			return nil, ErrEndpointGroupNotFound
		}
		return nil, fmt.Errorf("failed to fetch endpoint group: %w", err)
	}
	return result, nil
}

func updateEndpointGroup(ctx context.Context, client *APIClient, endpointGroupID int, payload map[string]interface{}, d *schema.ResourceData, hasTeam bool, teamID interface{}, hasUser bool, userID interface{}) error {
	if err := doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/endpoint_groups/%d", client.Endpoint, endpointGroupID), payload, nil); err != nil {
		return fmt.Errorf("failed to update endpoint group access: %w", err)
	}

	id := fmt.Sprintf("%d/", endpointGroupID)
	if hasTeam {
		id += fmt.Sprintf("team/%d", teamID.(int))
	} else {
		id += fmt.Sprintf("user/%d", userID.(int))
	}
	d.SetId(id)

	if diags := resourceEndpointGroupAccessRead(ctx, d, client); diags.HasError() {
		return fmt.Errorf("%s", diags[0].Summary)
	}
	return nil
}

func resourceEndpointGroupAccessRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointGroupID := d.Get("endpoint_group_id").(int)
	teamID, hasTeam := d.GetOk("team_id")
	userID, hasUser := d.GetOk("user_id")

	policies, err := getEndpointGroupPolicies(ctx, client, endpointGroupID)
	if err != nil {
		if errors.Is(err, ErrEndpointGroupNotFound) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(err)
	}

	found := false
	if hasTeam {
		tidStr := strconv.Itoa(teamID.(int))
		if p, ok := policies.TeamAccessPolicies[tidStr]; ok {
			if err := d.Set("role_id", p["RoleId"]); err != nil {
				return diag.FromErr(err)
			}
			found = true
		}
	} else if hasUser {
		uidStr := strconv.Itoa(userID.(int))
		if p, ok := policies.UserAccessPolicies[uidStr]; ok {
			if err := d.Set("role_id", p["RoleId"]); err != nil {
				return diag.FromErr(err)
			}
			found = true
		}
	}

	if !found {
		d.SetId("")
	}

	return nil
}

func resourceEndpointGroupAccessUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	return resourceEndpointGroupAccessCreate(ctx, d, meta)
}

func resourceEndpointGroupAccessDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointGroupID := d.Get("endpoint_group_id").(int)
	teamID, hasTeam := d.GetOk("team_id")
	userID, hasUser := d.GetOk("user_id")

	fullObject, err := getEndpointGroupMap(ctx, client, endpointGroupID)
	if err != nil {
		if errors.Is(err, ErrEndpointGroupNotFound) {
			return nil
		}
		return diag.FromErr(err)
	}

	// Modify policies in the map
	userPolicies := make(map[string]interface{})
	if up, ok := fullObject["UserAccessPolicies"].(map[string]interface{}); ok {
		userPolicies = up
	}
	teamPolicies := make(map[string]interface{})
	if tp, ok := fullObject["TeamAccessPolicies"].(map[string]interface{}); ok {
		teamPolicies = tp
	}

	if hasTeam {
		delete(teamPolicies, strconv.Itoa(teamID.(int)))
	}
	if hasUser {
		delete(userPolicies, strconv.Itoa(userID.(int)))
	}

	fullObject["UserAccessPolicies"] = userPolicies
	fullObject["TeamAccessPolicies"] = teamPolicies

	if err := doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/endpoint_groups/%d", client.Endpoint, endpointGroupID), fullObject, nil); err != nil {
		if isAPINotFound(err) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete endpoint group access: %w", err))
	}

	return nil
}
