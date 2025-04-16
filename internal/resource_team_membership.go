package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

type TeamMembershipPayload struct {
	Role   int `json:"role"`
	TeamID int `json:"teamID"`
	UserID int `json:"userID"`
}

type TeamMembershipResponse struct {
	ID     int `json:"Id"`
	Role   int `json:"Role"`
	TeamID int `json:"TeamID"`
	UserID int `json:"UserID"`
}

func resourceTeamMembership() *schema.Resource {
	return &schema.Resource{
		Create: resourceTeamMembershipCreate,
		Read:   resourceTeamMembershipRead,
		Update: resourceTeamMembershipUpdate,
		Delete: resourceTeamMembershipDelete,

		Importer: &schema.ResourceImporter{
			State: resourceTeamMembershipImport,
		},

		Schema: map[string]*schema.Schema{
			"role": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Membership role: 1 = team leader, 2 = regular member",
			},
			"team_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "ID of the team",
			},
			"user_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "ID of the user",
			},
		},
	}
}

func resourceTeamMembershipCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	payload := TeamMembershipPayload{
		Role:   d.Get("role").(int),
		TeamID: d.Get("team_id").(int),
		UserID: d.Get("user_id").(int),
	}

	resp, err := client.DoRequest("POST", "/team_memberships", nil, payload)
	if err != nil {
		return fmt.Errorf("failed to create team membership: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create team membership: %s", body)
	}

	var result TeamMembershipResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	d.SetId(strconv.Itoa(result.ID))
	return resourceTeamMembershipRead(d, meta)
}

func resourceTeamMembershipRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	resp, err := client.DoRequest("GET", "/team_memberships", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch team memberships list: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to fetch team memberships list: %s", resp.Status)
	}

	var memberships []TeamMembershipResponse
	if err := json.NewDecoder(resp.Body).Decode(&memberships); err != nil {
		return err
	}

	for _, m := range memberships {
		if strconv.Itoa(m.ID) == d.Id() {
			d.Set("role", m.Role)
			d.Set("team_id", m.TeamID)
			d.Set("user_id", m.UserID)
			return nil
		}
	}

	d.SetId("")
	return nil
}

func resourceTeamMembershipUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Id()

	payload := TeamMembershipPayload{
		Role:   d.Get("role").(int),
		TeamID: d.Get("team_id").(int),
		UserID: d.Get("user_id").(int),
	}

	resp, err := client.DoRequest("PUT", fmt.Sprintf("/team_memberships/%s", id), nil, payload)
	if err != nil {
		return fmt.Errorf("failed to update team membership: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update team membership: %s", body)
	}

	return resourceTeamMembershipRead(d, meta)
}

func resourceTeamMembershipImport(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	if err := resourceTeamMembershipRead(d, meta); err != nil {
		return nil, err
	}
	return []*schema.ResourceData{d}, nil
}

func resourceTeamMembershipDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Id()

	resp, err := client.DoRequest("DELETE", fmt.Sprintf("/team_memberships/%s", id), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete team membership: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete team membership: %s", body)
	}

	d.SetId("")
	return nil
}
