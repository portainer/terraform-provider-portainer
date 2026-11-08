package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceCustomTemplate() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceCustomTemplateRead,

		Schema: map[string]*schema.Schema{
			"title": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Title of the custom template to look up in Portainer.",
			},
			"description": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Description of the matched custom template.",
			},
			"type": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Template type: 1 = Swarm stack, 2 = Compose stack, 3 = Kubernetes manifest.",
			},
		},
	}
}

func dataSourceCustomTemplateRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	title := d.Get("title").(string)

	templates, err := listCustomTemplates(ctx, client)
	if err != nil {
		return diag.FromErr(err)
	}

	for _, t := range templates {
		if t.Title == title {
			d.SetId(strconv.FormatInt(t.ID, 10))
			if err := setFields(d, map[string]interface{}{
				"description": t.Description,
				"type":        int(t.Type),
			}); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("custom template %s not found", title))
}
