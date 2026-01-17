package internal

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/team_memberships"
	"github.com/portainer/client-api-go/v2/pkg/models"
)

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
	role := int64(d.Get("role").(int))
	teamID := int64(d.Get("team_id").(int))
	userID := int64(d.Get("user_id").(int))

	params := team_memberships.NewTeamMembershipCreateParams()
	params.Body = &models.TeammembershipsTeamMembershipCreatePayload{
		Role:   &role,
		TeamID: &teamID,
		UserID: &userID,
	}

	resp, err := client.Client.TeamMemberships.TeamMembershipCreate(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to create team membership: %w", err)
	}

	d.SetId(strconv.FormatInt(resp.Payload.ID, 10))
	return resourceTeamMembershipRead(d, meta)
}

func resourceTeamMembershipRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	params := team_memberships.NewTeamMembershipListParams()
	resp, err := client.Client.TeamMemberships.TeamMembershipList(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to fetch team memberships list: %w", err)
	}

	for _, m := range resp.Payload {
		if m.ID == id {
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
	id, _ := strconv.ParseInt(d.Id(), 10, 64)
	role := int64(d.Get("role").(int))
	teamID := int64(d.Get("team_id").(int))
	userID := int64(d.Get("user_id").(int))

	params := team_memberships.NewTeamMembershipUpdateParams()
	params.ID = id
	params.Body = &models.TeammembershipsTeamMembershipUpdatePayload{
		Role:   &role,
		TeamID: &teamID,
		UserID: &userID,
	}

	_, err := client.Client.TeamMemberships.TeamMembershipUpdate(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to update team membership: %w", err)
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
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	params := team_memberships.NewTeamMembershipDeleteParams()
	params.ID = id

	_, err := client.Client.TeamMemberships.TeamMembershipDelete(params, client.AuthInfo)
	if err != nil {
		if _, ok := err.(*team_memberships.TeamMembershipDeleteNotFound); ok {
			return nil
		}
		return fmt.Errorf("failed to delete team membership: %w", err)
	}

	return nil
}
