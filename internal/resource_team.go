package internal

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/portainer/client-api-go/v2/pkg/client/teams"
	"github.com/portainer/client-api-go/v2/pkg/models"
)

func resourceTeam() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceTeamCreate,
		ReadContext:   resourceTeamRead,
		DeleteContext: resourceTeamDelete,
		UpdateContext: resourceTeamUpdate,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
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

func resourceTeamCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	teamName := d.Get("name").(string)

	// Check if team already exists
	listCtx, listErrBody := withErrorCapture(ctx)
	paramsList := teams.NewTeamListParams()
	paramsList.SetContext(listCtx)
	respList, err := client.Client.Teams.TeamList(paramsList, client.AuthInfo)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to list teams: %w", decorateSDKError(err, listErrBody)))
	}

	for _, t := range respList.Payload {
		if t.Name == teamName {
			// Team already exists, perform update
			d.SetId(strconv.FormatInt(t.ID, 10))

			return resourceTeamUpdate(ctx, d, meta)
		}
	}

	// Team not found, create new
	createCtx, createErrBody := withErrorCapture(ctx)
	paramsCreate := teams.NewTeamCreateParams()
	paramsCreate.SetContext(createCtx)
	paramsCreate.Body = &models.TeamsTeamCreatePayload{
		Name: &teamName,
	}

	respCreate, err := client.Client.Teams.TeamCreate(paramsCreate, client.AuthInfo)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create team: %w", decorateSDKError(err, createErrBody)))
	}

	d.SetId(strconv.FormatInt(respCreate.Payload.ID, 10))
	return resourceTeamRead(ctx, d, meta)
}

func resourceTeamRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	ctx, errBody := withErrorCapture(ctx)
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
		return diag.FromErr(fmt.Errorf("failed to read team: %w", decorateSDKError(err, errBody)))
	}

	if err := d.Set("name", resp.Payload.Name); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceTeamUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)
	name := d.Get("name").(string)

	ctx, errBody := withErrorCapture(ctx)
	params := teams.NewTeamUpdateParams()
	params.SetContext(ctx)
	params.ID = id
	params.Body = &models.TeamsTeamUpdatePayload{
		Name: name,
	}

	_, err := client.Client.Teams.TeamUpdate(params, client.AuthInfo)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to update team: %w", decorateSDKError(err, errBody)))
	}

	return resourceTeamRead(ctx, d, meta)
}

func resourceTeamDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	ctx, errBody := withErrorCapture(ctx)
	params := teams.NewTeamDeleteParams()
	params.SetContext(ctx)
	params.ID = id

	_, err := client.Client.Teams.TeamDelete(params, client.AuthInfo)
	if err != nil {
		var notFoundDel *teams.TeamDeleteNotFound
		if errors.As(err, &notFoundDel) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete team: %w", decorateSDKError(err, errBody)))
	}

	return nil
}
