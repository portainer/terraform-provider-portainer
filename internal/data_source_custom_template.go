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
				Type:     schema.TypeString,
				Required: true,
			},
			"description": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"type": {
				Type:     schema.TypeInt,
				Computed: true,
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
			d.Set("description", t.Description)
			d.Set("type", int(t.Type))
			return nil
		}
	}

	return fmt.Errorf("custom template %s not found", title)
}
