package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// dataSourceAutoUpdates reports Portainer's own automatic update runs. It is
// the history rather than the configuration: what schedules the updates lives
// in Portainer's auto-patch settings.
func dataSourceAutoUpdates() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceAutoUpdatesRead,

		Schema: map[string]*schema.Schema{
			// Computed attributes
			"auto_updates": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The automatic update runs Portainer has recorded.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"version": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Portainer version the run updated to.",
						},
						"status": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Outcome of the run: `inProgress`, `completed` or `failed`.",
						},
						"started_at": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Unix timestamp the run started at.",
						},
						"done_at": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Unix timestamp the run finished at, zero while it is still running.",
						},
					},
				},
			},
		},
	}
}

func dataSourceAutoUpdatesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var response struct {
		AutoUpdates []struct {
			Version       string `json:"version"`
			Status        string `json:"status"`
			StartedAtUnix int64  `json:"startedAtUnix"`
			DoneAtUnix    int64  `json:"doneAtUnix"`
		} `json:"autoUpdates"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/auto_updates", nil, &response); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the automatic updates: %w", err))
	}

	updates := make([]interface{}, 0, len(response.AutoUpdates))
	for _, update := range response.AutoUpdates {
		updates = append(updates, map[string]interface{}{
			"version":    update.Version,
			"status":     update.Status,
			"started_at": int(update.StartedAtUnix),
			"done_at":    int(update.DoneAtUnix),
		})
	}

	if err := setFields(d, map[string]interface{}{"auto_updates": updates}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("portainer-auto-updates")
	return nil
}
