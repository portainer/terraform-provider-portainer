package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceBackupS3() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceBackupS3Create,
		ReadContext:   resourceBackupS3Read,
		DeleteContext: resourceBackupS3Delete,
		Schema: map[string]*schema.Schema{
			"access_key_id": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				ForceNew:    true,
				Description: "Sensitive S3 access key identifier used to upload the Portainer backup archive.",
			},
			"secret_access_key": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				ForceNew:    true,
				Description: "Sensitive S3 secret access key paired with `access_key_id`.",
			},
			"bucket_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the S3 bucket where the Portainer backup archive will be stored.",
			},
			"region": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "AWS or S3-compatible region of the destination bucket.",
			},
			"s3_compatible_host": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Endpoint URL of the S3 or S3-compatible service used for the backup upload.",
			},
			"password": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				ForceNew:    true,
				Description: "Sensitive password used to encrypt the Portainer backup archive before upload.",
			},
			"cron_rule": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Optional cron expression that schedules recurring S3 backups in Portainer.",
			},
			"last_run_failed": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the most recent scheduled S3 backup failed.",
			},
			"last_run_timestamp": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "UTC timestamp of the most recent scheduled S3 backup, empty when none has run.",
			},
		},
	}
}

func resourceBackupS3Create(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	body := map[string]interface{}{
		"accessKeyID":      d.Get("access_key_id").(string),
		"secretAccessKey":  d.Get("secret_access_key").(string),
		"bucketName":       d.Get("bucket_name").(string),
		"region":           d.Get("region").(string),
		"s3CompatibleHost": d.Get("s3_compatible_host").(string),
		"password":         d.Get("password").(string),
	}

	if v, ok := d.GetOk("cron_rule"); ok {
		body["cronRule"] = v.(string)
	}

	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/backup/s3/execute", body, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to execute S3 backup: %w", err))
	}

	d.SetId("portainer_backup_s3")
	return nil
}

func resourceBackupS3Read(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var result struct {
		AccessKeyID      string `json:"accessKeyID"`
		SecretAccessKey  string `json:"secretAccessKey"`
		BucketName       string `json:"bucketName"`
		Region           string `json:"region"`
		S3CompatibleHost string `json:"s3CompatibleHost"`
		Password         string `json:"password"`
		CronRule         string `json:"cronRule"`
	}

	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/backup/s3/settings", nil, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read S3 backup settings: %w", err))
	}

	if err := d.Set("access_key_id", result.AccessKeyID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("secret_access_key", result.SecretAccessKey); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("bucket_name", result.BucketName); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("region", result.Region); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("s3_compatible_host", result.S3CompatibleHost); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("password", result.Password); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("cron_rule", result.CronRule); err != nil {
		return diag.FromErr(err)
	}

	// Informational only: an instance that has never run a scheduled backup
	// should not fail the read.
	var status struct {
		Failed       bool   `json:"Failed"`
		TimestampUTC string `json:"TimestampUTC"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/backup/s3/status", nil, &status); err == nil {
		if err := setFields(d, map[string]interface{}{
			"last_run_failed":    status.Failed,
			"last_run_timestamp": status.TimestampUTC,
		}); err != nil {
			return diag.FromErr(err)
		}
	}

	d.SetId("portainer_backup_s3")
	return nil
}

func resourceBackupS3Delete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// This operation cannot be undone via API; just remove from state.
	d.SetId("")
	return nil
}
