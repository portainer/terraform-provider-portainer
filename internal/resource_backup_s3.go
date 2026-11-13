package internal

import (
	"context"
	"fmt"
	"net/http"
	"strings"

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
				Type:         schema.TypeString,
				Optional:     true,
				ExactlyOneOf: []string{"secret_access_key", "secret_access_key_wo"},
				Sensitive:    true,
				ForceNew:     true,
				Description:  "Sensitive S3 secret access key paired with `access_key_id`. Stored in state as a sensitive value; use `secret_access_key_wo` to keep it out of state.",
			},
			"secret_access_key_wo":         writeOnlySecretSchema("secret_access_key", "secret_access_key_wo_version", "S3 secret access key"),
			"secret_access_key_wo_version": writeOnlyVersionSchema("secret_access_key_wo", true),
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
				Type:         schema.TypeString,
				Optional:     true,
				ExactlyOneOf: []string{"password", "password_wo"},
				Sensitive:    true,
				ForceNew:     true,
				Description:  "Sensitive password used to encrypt the Portainer backup archive before upload. Stored in state as a sensitive value; use `password_wo` to keep it out of state.",
			},
			"password_wo":         writeOnlySecretSchema("password", "password_wo_version", "password used to encrypt the backup archive"),
			"password_wo_version": writeOnlyVersionSchema("password_wo", true),
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

	secretAccessKey, err := twinSecret(d, "secret_access_key")
	if err != nil {
		return diag.FromErr(err)
	}
	password, err := twinSecret(d, "password")
	if err != nil {
		return diag.FromErr(err)
	}
	body := map[string]interface{}{
		"accessKeyID":      d.Get("access_key_id").(string),
		"secretAccessKey":  secretAccessKey,
		"bucketName":       d.Get("bucket_name").(string),
		"region":           d.Get("region").(string),
		"s3CompatibleHost": d.Get("s3_compatible_host").(string),
		"password":         password,
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

// isRedacted reports a secret the API no longer returns: empty, or masked
// with asterisks.
func isRedacted(s string) bool {
	return strings.Trim(s, "*") == ""
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
	// A secret given write-only is not read back: storing it is what the
	// write-only twin avoids, and the configuration has no value to compare.
	// Portainer 3.0 redacts both secrets in this response; a redacted value
	// keeps the one in state rather than showing as a change on every plan.
	if d.Get("secret_access_key_wo_version").(int) == 0 && !isRedacted(result.SecretAccessKey) {
		if err := d.Set("secret_access_key", result.SecretAccessKey); err != nil {
			return diag.FromErr(err)
		}
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
	if d.Get("password_wo_version").(int) == 0 && !isRedacted(result.Password) {
		if err := d.Set("password", result.Password); err != nil {
			return diag.FromErr(err)
		}
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
