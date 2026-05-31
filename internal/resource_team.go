package internal

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
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
				Type:         schema.TypeString,
				Required:     true,
				ValidateFunc: validation.NoZeroValues,
				Description:  "Name of the Portainer team. Must be unique within the Portainer instance.",
			},
		},
	}
}

func resourceTeamCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	teamName := d.Get("name").(string)

	// Check if team already exists
	listCtx, listErrBody := withErrorCapture(context.Background())
	paramsList := teams.NewTeamListParams()
	paramsList.SetContext(listCtx)
	respList, err := client.Client.Teams.TeamList(paramsList, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to list teams: %w", decorateSDKError(err, listErrBody))
	}

	for _, t := range respList.Payload {
		if t.Name == teamName {
			// Team already exists, perform update
			d.SetId(strconv.FormatInt(t.ID, 10))

			return resourceTeamUpdate(d, meta)
		}
	}

	// Team not found, create new
	createCtx, createErrBody := withErrorCapture(context.Background())
	paramsCreate := teams.NewTeamCreateParams()
	paramsCreate.SetContext(createCtx)
	paramsCreate.Body = &models.TeamsTeamCreatePayload{
		Name: &teamName,
	}

	respCreate, err := client.Client.Teams.TeamCreate(paramsCreate, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to create team: %w", decorateSDKError(err, createErrBody))
	}

	d.SetId(strconv.FormatInt(respCreate.Payload.ID, 10))
	return resourceTeamRead(d, meta)
}

func resourceTeamRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	ctx, errBody := withErrorCapture(context.Background())
	params := teams.NewTeamInspectParams()
	params.SetContext(ctx)
	params.ID = id

	resp, err := client.Client.Teams.TeamInspect(params, client.AuthInfo)
	if err != nil {
		var notFound *teams.TeamInspectNotFound
		if errors.As(err, &notFound) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("failed to read team: %w", decorateSDKError(err, errBody))
	}

	d.Set("name", resp.Payload.Name)
	return nil
}

func resourceTeamUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)
	name := d.Get("name").(string)

	ctx, errBody := withErrorCapture(context.Background())
	params := teams.NewTeamUpdateParams()
	params.SetContext(ctx)
	params.ID = id
	params.Body = &models.TeamsTeamUpdatePayload{
		Name: name,
	}

	_, err := client.Client.Teams.TeamUpdate(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to update team: %w", decorateSDKError(err, errBody))
	}

	return resourceTeamRead(d, meta)
}

func resourceTeamDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	ctx, errBody := withErrorCapture(context.Background())
	params := teams.NewTeamDeleteParams()
	params.SetContext(ctx)
	params.ID = id

	_, err := client.Client.Teams.TeamDelete(params, client.AuthInfo)
	if err != nil {
		var notFoundDel *teams.TeamDeleteNotFound
		if errors.As(err, &notFoundDel) {
			return nil
		}
		return fmt.Errorf("failed to delete team: %w", decorateSDKError(err, errBody))
	}

	return nil
}
