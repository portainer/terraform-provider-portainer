package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceMOTD() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceMOTDRead,

		Schema: map[string]*schema.Schema{
			// Computed attributes
			"title": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Title of the message of the day, empty when Portainer is serving none.",
			},
			"message": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Body of the message of the day.",
			},
			"style": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Inline CSS Portainer's UI applies to the message.",
			},
			"hash": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Hash of the message, which the UI uses to remember that a user has dismissed this particular one. Changes whenever the message changes.",
			},
		},
	}
}

func dataSourceMOTDRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// Hash is []byte on the Portainer side, so it arrives base64-encoded and is
	// kept as the opaque string it is.
	var motd struct {
		Title   string `json:"Title"`
		Message string `json:"Message"`
		Style   string `json:"Style"`
		Hash    string `json:"Hash"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/motd", nil, &motd); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the message of the day: %w", err))
	}

	if err := setFields(d, map[string]interface{}{
		"title":   motd.Title,
		"message": motd.Message,
		"style":   motd.Style,
		"hash":    motd.Hash,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("portainer-motd")
	return nil
}
