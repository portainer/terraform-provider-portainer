package internal

import (
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/registries"
)

func dataSourceRegistry() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceRegistryRead,

		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"url": {
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

func dataSourceRegistryRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	params := registries.NewRegistryListParams()
	resp, err := client.Client.Registries.RegistryList(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to list registries: %w", err)
	}

	for _, r := range resp.Payload {
		if r.Name == name {
			d.SetId(strconv.FormatInt(r.ID, 10))
			d.Set("url", r.URL)
			d.Set("type", int(r.Type))
			return nil
		}
	}

	return fmt.Errorf("registry %s not found", name)
}
