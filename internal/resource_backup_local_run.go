package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceBackupLocalRun() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceBackupLocalRunCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: schema.NoopContext,

		Schema: map[string]*schema.Schema{
			"triggers": {
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Arbitrary values that force another backup run when they change. A backup is a one-shot action, so without a trigger the resource runs once and then stays put.",
			},
			// Computed attributes
			"path": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Path on the Portainer host where the archive was written, as reported after the run.",
			},
			"failed": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Portainer reports the run as failed.",
			},
			"timestamp": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "UTC timestamp Portainer reports for the run.",
			},
		},
	}
}

func resourceBackupLocalRunCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/backup/local/run", nil, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to trigger a local backup: %w", err))
	}

	// The run endpoint answers without a body, so the outcome comes from the
	// status endpoint. It is informational: a backup that ran is not undone by
	// a status read that fails.
	var status struct {
		Failed       bool   `json:"failed"`
		TimestampUTC string `json:"timestampUTC"`
		Path         string `json:"path"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/backup/local/status", nil, &status); err == nil {
		if err := setFields(d, map[string]interface{}{
			"path": status.Path, "failed": status.Failed, "timestamp": status.TimestampUTC,
		}); err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId(fmt.Sprintf("portainer-backup-local-run-%d", makeTimestamp()))
	return nil
}
