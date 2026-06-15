package internal

import (
	"context"
	"fmt"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/tags"
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

	ctx, errBody := withErrorCapture(ctx)
	params := tags.NewTagListParams()
	params.SetContext(ctx)
	resp, err := client.Client.Tags.TagList(params, client.AuthInfo)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to list tags: %w", decorateSDKError(err, errBody)))
	}

	for _, t := range resp.Payload {
		if t.Name == name {
			d.SetId(strconv.FormatInt(t.ID, 10))
			return nil
		}
	}

	return diag.FromErr(fmt.Errorf("tag %s not found", name))
}
