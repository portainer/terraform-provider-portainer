package internal

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/teams"
	"github.com/portainer/client-api-go/v2/pkg/models"
)

func resourceTeam() *schema.Resource {
	return &schema.Resource{
		Create: resourceTeamCreate,
		Read:   resourceTeamRead,
		Delete: resourceTeamDelete,
		Update: resourceTeamUpdate,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func resourceTeamCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	teamName := d.Get("name").(string)

	// Check if team already exists
	paramsList := teams.NewTeamListParams()
	respList, err := client.Client.Teams.TeamList(paramsList, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to list teams: %w", err)
	}

	for _, t := range respList.Payload {
		if t.Name == teamName {
			// Team already exists, perform update
			d.SetId(strconv.FormatInt(t.ID, 10))

			return resourceTeamUpdate(d, meta)
		}
	}

	// Team not found, create new
	paramsCreate := teams.NewTeamCreateParams()
	paramsCreate.Body = &models.TeamsTeamCreatePayload{
		Name: &teamName,
	}

	respCreate, err := client.Client.Teams.TeamCreate(paramsCreate, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to create team: %w", err)
	}

	d.SetId(strconv.FormatInt(respCreate.Payload.ID, 10))
	return resourceTeamRead(d, meta)
}

func resourceTeamRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	params := teams.NewTeamInspectParams()
	params.ID = id

	resp, err := client.Client.Teams.TeamInspect(params, client.AuthInfo)
	if err != nil {
		if _, ok := err.(*teams.TeamInspectNotFound); ok {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("failed to read team: %w", err)
	}

	d.Set("name", resp.Payload.Name)
	return nil
}

func resourceTeamUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)
	name := d.Get("name").(string)

	params := teams.NewTeamUpdateParams()
	params.ID = id
	params.Body = &models.TeamsTeamUpdatePayload{
		Name: name,
	}

	_, err := client.Client.Teams.TeamUpdate(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to update team: %w", err)
	}

	return resourceTeamRead(d, meta)
}

func resourceTeamDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	params := teams.NewTeamDeleteParams()
	params.ID = id

	_, err := client.Client.Teams.TeamDelete(params, client.AuthInfo)
	if err != nil {
		if _, ok := err.(*teams.TeamDeleteNotFound); ok {
			return nil
		}
		return fmt.Errorf("failed to delete team: %w", err)
	}

	return nil
}
