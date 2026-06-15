package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
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

	resp, err := client.DoRequest("POST", "/backup/s3/execute", nil, body)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to execute S3 backup: %s", string(data)))
	}

	d.SetId("portainer_backup_s3")
	return nil
}

func resourceBackupS3Read(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	resp, err := client.DoRequest("GET", "/backup/s3/settings", nil, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to fetch S3 backup settings: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to read S3 backup settings: %s", string(data)))
	}

	var result struct {
		AccessKeyID      string `json:"accessKeyID"`
		SecretAccessKey  string `json:"secretAccessKey"`
		BucketName       string `json:"bucketName"`
		Region           string `json:"region"`
		S3CompatibleHost string `json:"s3CompatibleHost"`
		Password         string `json:"password"`
		CronRule         string `json:"cronRule"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode S3 settings: %w", err))
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

	d.SetId("portainer_backup_s3")
	return nil
}

func resourceBackupS3Delete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	// This operation cannot be undone via API; just remove from state.
	d.SetId("")
	return nil
}
