package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/teams"
)

func dataSourceTeam() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceTeamRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Portainer team to look up. The data source will fail if no matching team is found.",
			},
		},
	}
}

func dataSourceTeamRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	teamName := d.Get("name").(string)

	ctx, errBody := withErrorCapture(ctx)
	params := teams.NewTeamListParams()
	params.SetContext(ctx)
	resp, err := client.Client.Teams.TeamList(params, client.AuthInfo)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to list teams: %w", decorateSDKError(err, errBody)))
	}

	for _, t := range resp.Payload {
		if t.Name == teamName {
			d.SetId(strconv.FormatInt(t.ID, 10))
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("team %s not found", teamName))
}
