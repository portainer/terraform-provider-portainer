package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceTag() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceTagRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Portainer tag to look up. The data source will fail if no matching tag is found.",
			},
		},
	}
}

func dataSourceTagRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	tagList, err := listTags(ctx, client)
	if err != nil {
		return diag.FromErr(err)
	}

	for _, t := range tagList {
		if t.Name == name {
			d.SetId(strconv.FormatInt(t.ID, 10))
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("tag %s not found", name))
}
