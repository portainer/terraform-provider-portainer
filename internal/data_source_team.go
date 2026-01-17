package internal

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/teams"
)

func dataSourceTeam() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceTeamRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func dataSourceTeamRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	teamName := d.Get("name").(string)

	params := teams.NewTeamListParams()
	resp, err := client.Client.Teams.TeamList(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to list teams: %w", err)
	}

	for _, t := range resp.Payload {
		if t.Name == teamName {
			d.SetId(strconv.FormatInt(t.ID, 10))
			return nil
		}
	}

	return fmt.Errorf("team %s not found", teamName)
}
