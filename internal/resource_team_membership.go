package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
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

	var created portainerTeamMembership
	body := teamMembershipPayload{Role: role, TeamID: teamID, UserID: userID}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/team_memberships", body, &created); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create team membership: %w", err))
	}

	d.SetId(strconv.FormatInt(created.ID, 10))
	return resourceTeamMembershipRead(ctx, d, meta)
}

func resourceTeamMembershipRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	memberships, err := listTeamMemberships(ctx, client)
	if err != nil {
		return diag.FromErr(err)
	}

	for _, m := range memberships {
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

	body := teamMembershipPayload{Role: role, TeamID: teamID, UserID: userID}
	if err := doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/team_memberships/%d", client.Endpoint, id), body, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update team membership: %w", err))
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

	if err := doJSON(ctx, client, http.MethodDelete, fmt.Sprintf("%s/team_memberships/%d", client.Endpoint, id), nil, nil); err != nil {
		if isAPINotFound(err) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete team membership: %w", err))
	}

	return nil
}

// teamMembershipPayload is the POST /team_memberships and
// PUT /team_memberships/{id} body.
type teamMembershipPayload struct {
	Role   int64 `json:"role"`
	TeamID int64 `json:"teamID"`
	UserID int64 `json:"userID"`
}

type portainerTeamMembership struct {
	ID     int64 `json:"Id"`
	Role   int64 `json:"Role"`
	TeamID int64 `json:"TeamID"`
	UserID int64 `json:"UserID"`
}

// listTeamMemberships returns every team membership (GET /team_memberships).
func listTeamMemberships(ctx context.Context, client *APIClient) ([]portainerTeamMembership, error) {
	var memberships []portainerTeamMembership
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/team_memberships", nil, &memberships); err != nil {
		return nil, fmt.Errorf("failed to fetch team memberships list: %w", err)
	}
	return memberships, nil
}
