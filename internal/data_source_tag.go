package internal

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/tags"
)

func dataSourceTag() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceTagRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
		},
	}
}

func dataSourceTagRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	params := tags.NewTagListParams()
	resp, err := client.Client.Tags.TagList(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to list tags: %w", err)
	}

	for _, t := range resp.Payload {
		if t.Name == name {
			d.SetId(strconv.FormatInt(t.ID, 10))
			return nil
		}
	}

	return fmt.Errorf("tag %s not found", name)
}
