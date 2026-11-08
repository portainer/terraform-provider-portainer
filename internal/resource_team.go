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
	teamList, err := listTeams(ctx, client)
	if err != nil {
		return diag.FromErr(err)
	}

	for _, t := range teamList {
		if t.Name == teamName {
			// Team already exists, perform update
			d.SetId(strconv.FormatInt(t.ID, 10))

			return resourceTeamUpdate(ctx, d, meta)
		}
	}

	// Team not found, create new
	var created portainerTeam
	body := struct {
		Name        string  `json:"name"`
		TeamLeaders []int64 `json:"teamLeaders"`
	}{Name: teamName}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/teams", body, &created); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create team: %w", err))
	}

	d.SetId(strconv.FormatInt(created.ID, 10))
	return resourceTeamRead(ctx, d, meta)
}

func resourceTeamRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	var team portainerTeam
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/teams/%d", client.Endpoint, id), nil, &team); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read team: %w", err))
	}

	if err := d.Set("name", team.Name); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceTeamUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)
	name := d.Get("name").(string)

	body := struct {
		Name string `json:"name,omitempty"`
	}{Name: name}
	if err := doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/teams/%d", client.Endpoint, id), body, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update team: %w", err))
	}

	return resourceTeamRead(ctx, d, meta)
}

func resourceTeamDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	if err := doJSON(ctx, client, http.MethodDelete, fmt.Sprintf("%s/teams/%d", client.Endpoint, id), nil, nil); err != nil {
		if isAPINotFound(err) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete team: %w", err))
	}

	return nil
}

type portainerTeam struct {
	ID   int64  `json:"Id"`
	Name string `json:"Name"`
}

// listTeams returns every team (GET /teams).
func listTeams(ctx context.Context, client *APIClient) ([]portainerTeam, error) {
	var teamList []portainerTeam
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/teams", nil, &teamList); err != nil {
		return nil, fmt.Errorf("failed to list teams: %w", err)
	}
	return teamList, nil
}
