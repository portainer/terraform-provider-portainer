package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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

	teamList, err := listTeams(ctx, client)
	if err != nil {
		return diag.FromErr(err)
	}

	for _, t := range teamList {
		if t.Name == teamName {
			d.SetId(strconv.FormatInt(t.ID, 10))
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("team %s not found", teamName))
}
