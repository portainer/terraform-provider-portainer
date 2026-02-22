package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

var ErrEndpointGroupNotFound = errors.New("endpoint group not found")

func resourceEndpointGroupAccess() *schema.Resource {
	return &schema.Resource{
		Create: resourceEndpointGroupAccessCreate,
		Read:   resourceEndpointGroupAccessRead,
		Update: resourceEndpointGroupAccessUpdate,
		Delete: resourceEndpointGroupAccessDelete,

		Schema: map[string]*schema.Schema{
			"endpoint_group_id": {
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"team_id": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
			},
			"user_id": {
				Type:     schema.TypeInt,
				Optional: true,
				ForceNew: true,
			},
			"role_id": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  0,
			},
		},
	}
}

type EndpointGroupAccessPolicies struct {
	UserAccessPolicies map[string]map[string]int `json:"UserAccessPolicies"`
	TeamAccessPolicies map[string]map[string]int `json:"TeamAccessPolicies"`
}

func getEndpointGroupPolicies(client *APIClient, endpointGroupID int) (*EndpointGroupAccessPolicies, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/endpoint_groups/%d", client.Endpoint, endpointGroupID), nil)
	if err != nil {
		return nil, err
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, ErrEndpointGroupNotFound
	}

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch endpoint group: %s", string(data))
	}

	var policies EndpointGroupAccessPolicies
	if err := json.NewDecoder(resp.Body).Decode(&policies); err != nil {
		return nil, err
	}

	if policies.UserAccessPolicies == nil {
		policies.UserAccessPolicies = make(map[string]map[string]int)
	}
	if policies.TeamAccessPolicies == nil {
		policies.TeamAccessPolicies = make(map[string]map[string]int)
	}

	return &policies, nil
}

func resourceEndpointGroupAccessCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointGroupID := d.Get("endpoint_group_id").(int)
	teamID, hasTeam := d.GetOk("team_id")
	userID, hasUser := d.GetOk("user_id")
	roleID := d.Get("role_id").(int)

	if !hasTeam && !hasUser {
		return fmt.Errorf("either team_id or user_id must be provided")
	}

	policies, err := getEndpointGroupPolicies(client, endpointGroupID)
	if err != nil {
		return err
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
	fullObject, err := getEndpointGroupMap(client, endpointGroupID)
	if err != nil {
		return err
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

	return updateEndpointGroup(client, endpointGroupID, fullObject, d, hasTeam, teamID, hasUser, userID)
}

func getEndpointGroupMap(client *APIClient, endpointGroupID int) (map[string]interface{}, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/endpoint_groups/%d", client.Endpoint, endpointGroupID), nil)
	if err != nil {
		return nil, err
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, ErrEndpointGroupNotFound
	}
	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to fetch endpoint group: %s", string(data))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}

func updateEndpointGroup(client *APIClient, endpointGroupID int, payload map[string]interface{}, d *schema.ResourceData, hasTeam bool, teamID interface{}, hasUser bool, userID interface{}) error {
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/endpoint_groups/%d", client.Endpoint, endpointGroupID), bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update endpoint group access: %s", string(data))
	}

	id := fmt.Sprintf("%d/", endpointGroupID)
	if hasTeam {
		id += fmt.Sprintf("team/%d", teamID.(int))
	} else {
		id += fmt.Sprintf("user/%d", userID.(int))
	}
	d.SetId(id)

	return resourceEndpointGroupAccessRead(d, client)
}

func resourceEndpointGroupAccessRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointGroupID := d.Get("endpoint_group_id").(int)
	teamID, hasTeam := d.GetOk("team_id")
	userID, hasUser := d.GetOk("user_id")

	policies, err := getEndpointGroupPolicies(client, endpointGroupID)
	if err != nil {
		if errors.Is(err, ErrEndpointGroupNotFound) {
			d.SetId("")
			return nil
		}
		return err
	}

	found := false
	if hasTeam {
		tidStr := strconv.Itoa(teamID.(int))
		if p, ok := policies.TeamAccessPolicies[tidStr]; ok {
			d.Set("role_id", p["RoleId"])
			found = true
		}
	} else if hasUser {
		uidStr := strconv.Itoa(userID.(int))
		if p, ok := policies.UserAccessPolicies[uidStr]; ok {
			d.Set("role_id", p["RoleId"])
			found = true
		}
	}

	if !found {
		d.SetId("")
	}

	return nil
}

func resourceEndpointGroupAccessUpdate(d *schema.ResourceData, meta interface{}) error {
	return resourceEndpointGroupAccessCreate(d, meta)
}

func resourceEndpointGroupAccessDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointGroupID := d.Get("endpoint_group_id").(int)
	teamID, hasTeam := d.GetOk("team_id")
	userID, hasUser := d.GetOk("user_id")

	fullObject, err := getEndpointGroupMap(client, endpointGroupID)
	if err != nil {
		if errors.Is(err, ErrEndpointGroupNotFound) {
			return nil
		}
		return err
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

	jsonBody, err := json.Marshal(fullObject)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/endpoint_groups/%d", client.Endpoint, endpointGroupID), bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil
	}
	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete endpoint group access: %s", string(data))
	}

	return nil
}
