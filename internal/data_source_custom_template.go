package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/custom_templates"
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

	ctx, errBody := withErrorCapture(ctx)
	params := custom_templates.NewCustomTemplateListParams()
	params.SetContext(ctx)
	resp, err := client.Client.CustomTemplates.CustomTemplateList(params, client.AuthInfo)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to list custom templates: %w", decorateSDKError(err, errBody)))
	}

	for _, t := range resp.Payload {
		if t.Title == title {
			d.SetId(strconv.FormatInt(t.ID, 10))
			_ = d.Set("description", t.Description)
			_ = d.Set("type", int(t.Type))
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("custom template %s not found", title))
}
