package internal

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/custom_templates"
)

func dataSourceCustomTemplate() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceCustomTemplateRead,

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

func dataSourceCustomTemplateRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	title := d.Get("title").(string)

	params := custom_templates.NewCustomTemplateListParams()
	resp, err := client.Client.CustomTemplates.CustomTemplateList(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to list custom templates: %w", err)
	}

	for _, t := range resp.Payload {
		if t.Title == title {
			d.SetId(strconv.FormatInt(t.ID, 10))
			_ = d.Set("description", t.Description)
			_ = d.Set("type", int(t.Type))
			return nil
		}
	}

	return fmt.Errorf("custom template %s not found", title)
}
