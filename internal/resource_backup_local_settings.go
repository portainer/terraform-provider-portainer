package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceBackupLocalSettings() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceBackupLocalSettingsWrite,
		ReadContext:   resourceBackupLocalSettingsRead,
		UpdateContext: resourceBackupLocalSettingsWrite,
		// Portainer has no endpoint to clear the settings; removing the resource
		// stops managing them and leaves the schedule as configured.
		DeleteContext: removeFromStateContext,

		Schema: map[string]*schema.Schema{
			"cron_rule": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Cron expression for the scheduled local backup (for example `0 2 * * *`). Leave unset to keep the schedule off and back up only on demand with `portainer_backup_local_run`.",
			},
			"retention_days": {
				Type:         schema.TypeInt,
				Optional:     true,
				ValidateFunc: validation.IntAtLeast(0),
				Description:  "How many days Portainer keeps local backup archives before pruning them. Zero keeps them indefinitely.",
			},
			// Computed attributes
			"last_run_failed": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the most recent scheduled local backup failed.",
			},
			"last_run_timestamp": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "UTC timestamp of the most recent scheduled local backup, empty when none has run.",
			},
			"last_run_path": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Path on the Portainer host where the most recent local backup archive was written.",
			},
		},
	}
}

func resourceBackupLocalSettingsWrite(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := map[string]interface{}{
		"cronRule":      d.Get("cron_rule").(string),
		"retentionDays": d.Get("retention_days").(int),
	}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/backup/local/settings", payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update the local backup settings: %w", err))
	}

	d.SetId("portainer-backup-local-settings")
	return resourceBackupLocalSettingsRead(ctx, d, meta)
}

func resourceBackupLocalSettingsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var settings struct {
		CronRule      string `json:"cronRule"`
		RetentionDays int    `json:"retentionDays"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/backup/local/settings", nil, &settings); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read the local backup settings: %w", err))
	}

	fields := map[string]interface{}{
		"cron_rule":      settings.CronRule,
		"retention_days": settings.RetentionDays,
	}

	// Informational only: an instance that has never run a scheduled backup
	// should not fail the read.
	var status struct {
		Failed       bool   `json:"failed"`
		TimestampUTC string `json:"timestampUTC"`
		Path         string `json:"path"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/backup/local/status", nil, &status); err == nil {
		fields["last_run_failed"] = status.Failed
		fields["last_run_timestamp"] = status.TimestampUTC
		fields["last_run_path"] = status.Path
	}

	if err := setFields(d, fields); err != nil {
		return diag.FromErr(err)
	}
	return nil
}
