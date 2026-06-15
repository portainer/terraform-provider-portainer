package internal

import (
	"context"
	"errors"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/portainer/client-api-go/v2/pkg/client/team_memberships"
	"github.com/portainer/client-api-go/v2/pkg/models"
)

func resourceTeamMembership() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceTeamMembershipCreate,
		ReadContext:   resourceTeamMembershipRead,
		UpdateContext: resourceTeamMembershipUpdate,
		DeleteContext: resourceTeamMembershipDelete,

		Importer: &schema.ResourceImporter{
			StateContext: resourceTeamMembershipImport,
		},

		Schema: map[string]*schema.Schema{
			"role": {
				Type:         schema.TypeInt,
				Required:     true,
				Description:  "Membership role: 1 = team leader, 2 = regular member",
				ValidateFunc: validation.IntBetween(1, 2),
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

func resourceTeamMembershipCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	role := int64(d.Get("role").(int))
	teamID := int64(d.Get("team_id").(int))
	userID := int64(d.Get("user_id").(int))

	ctx, errBody := withErrorCapture(ctx)
	params := team_memberships.NewTeamMembershipCreateParams()
	params.SetContext(ctx)
	params.Body = &models.TeammembershipsTeamMembershipCreatePayload{
		Role:   &role,
		TeamID: &teamID,
		UserID: &userID,
	}

	resp, err := client.Client.TeamMemberships.TeamMembershipCreate(params, client.AuthInfo)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create team membership: %w", decorateSDKError(err, errBody)))
	}

	d.SetId(strconv.FormatInt(resp.Payload.ID, 10))
	return resourceTeamMembershipRead(ctx, d, meta)
}

func resourceTeamMembershipRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	ctx, errBody := withErrorCapture(ctx)
	params := team_memberships.NewTeamMembershipListParams()
	params.SetContext(ctx)
	resp, err := client.Client.TeamMemberships.TeamMembershipList(params, client.AuthInfo)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to fetch team memberships list: %w", decorateSDKError(err, errBody)))
	}

	for _, m := range resp.Payload {
		if m.ID == id {
			if err := d.Set("role", m.Role); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set("team_id", m.TeamID); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set("user_id", m.UserID); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	d.SetId("")
	return nil
}

func resourceTeamMembershipUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)
	role := int64(d.Get("role").(int))
	teamID := int64(d.Get("team_id").(int))
	userID := int64(d.Get("user_id").(int))

	ctx, errBody := withErrorCapture(ctx)
	params := team_memberships.NewTeamMembershipUpdateParams()
	params.SetContext(ctx)
	params.ID = id
	params.Body = &models.TeammembershipsTeamMembershipUpdatePayload{
		Role:   &role,
		TeamID: &teamID,
		UserID: &userID,
	}

	_, err := client.Client.TeamMemberships.TeamMembershipUpdate(params, client.AuthInfo)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to update team membership: %w", decorateSDKError(err, errBody)))
	}

	return resourceTeamMembershipRead(ctx, d, meta)
}

func resourceTeamMembershipImport(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
	if diags := resourceTeamMembershipRead(ctx, d, meta); diags.HasError() {
		return nil, fmt.Errorf("failed to import team membership: %s", diags[0].Summary)
	}
	return []*schema.ResourceData{d}, nil
}

func resourceTeamMembershipDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	ctx, errBody := withErrorCapture(ctx)
	params := team_memberships.NewTeamMembershipDeleteParams()
	params.SetContext(ctx)
	params.ID = id

	_, err := client.Client.TeamMemberships.TeamMembershipDelete(params, client.AuthInfo)
	if err != nil {
		var notFound *team_memberships.TeamMembershipDeleteNotFound
		if errors.As(err, &notFound) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete team membership: %w", decorateSDKError(err, errBody)))
	}

	return nil
}
