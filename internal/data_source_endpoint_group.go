package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceEndpointGroup() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEndpointGroupRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Portainer endpoint group to look up. The data source will fail if no matching endpoint group is found.",
			},
			"description": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Description of the Portainer endpoint group as configured in Portainer.",
			},
		},
	}
}

func dataSourceEndpointGroupRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	groups, err := listEndpointGroups(ctx, client)
	if err != nil {
		return diag.FromErr(err)
	}

	for _, g := range groups {
		if g.Name == name {
			d.SetId(strconv.FormatInt(g.ID, 10))
			if err := d.Set("description", g.Description); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("endpoint group %s not found", name))
}
