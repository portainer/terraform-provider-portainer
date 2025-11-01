package internal

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceBackupS3() *schema.Resource {
	return &schema.Resource{
		Create: resourceBackupS3Create,
		Read:   resourceBackupS3Read,
		Delete: resourceBackupS3Delete,
		Schema: map[string]*schema.Schema{
			"access_key_id": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				ForceNew:      true,
				ConflictsWith: []string{"access_key_id_wo", "backup_wo_version"},
				Description:   "S3 Access Key ID (stored in Terraform state).",
			},
			"secret_access_key": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				ForceNew:      true,
				ConflictsWith: []string{"secret_access_key_wo", "backup_wo_version"},
				Description:   "S3 Secret Access Key (stored in Terraform state).",
			},
			"password": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				ForceNew:      true,
				ConflictsWith: []string{"password_wo", "backup_wo_version"},
				Description:   "Encryption password (stored in Terraform state).",
			},
			"access_key_id_wo": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				WriteOnly:     true,
				Computed:      true,
				RequiredWith:  []string{"backup_wo_version"},
				ConflictsWith: []string{"access_key_id"},
				Description:   "Ephemeral S3 Access Key ID (write-only, not stored in state).",
			},
			"secret_access_key_wo": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				WriteOnly:     true,
				Computed:      true,
				RequiredWith:  []string{"backup_wo_version"},
				ConflictsWith: []string{"secret_access_key"},
				Description:   "Ephemeral S3 Secret Access Key (write-only, not stored in state).",
			},
			"password_wo": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				WriteOnly:     true,
				Computed:      true,
				RequiredWith:  []string{"backup_wo_version"},
				ConflictsWith: []string{"password"},
				Description:   "Ephemeral encryption password (write-only, not stored in state).",
			},
			"backup_wo_version": {
				Type:          schema.TypeInt,
				Optional:      true,
				ForceNew:      true,
				Description:   "Version flag to trigger recreation when using ephemeral credentials.",
				ConflictsWith: []string{"access_key_id", "secret_access_key", "password"},
			},
			"bucket_name": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"region": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"s3_compatible_host": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"cron_rule": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
		},
	}
}

func getBackupS3Credentials(d *schema.ResourceData) (string, string, string) {
	accessKey := d.Get("access_key_id").(string)
	secretKey := d.Get("secret_access_key").(string)
	password := d.Get("password").(string)

	if d.Get("backup_wo_version").(int) != 0 {
		if raw, diag := d.GetRawConfigAt(cty.GetAttrPath("access_key_id_wo")); diag == nil && raw.IsKnown() && !raw.IsNull() {
			accessKey = raw.AsString()
		}
		if raw, diag := d.GetRawConfigAt(cty.GetAttrPath("secret_access_key_wo")); diag == nil && raw.IsKnown() && !raw.IsNull() {
			secretKey = raw.AsString()
		}
		if raw, diag := d.GetRawConfigAt(cty.GetAttrPath("password_wo")); diag == nil && raw.IsKnown() && !raw.IsNull() {
			password = raw.AsString()
		}
	}

	return accessKey, secretKey, password
}

func resourceBackupS3Create(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	accessKey, secretKey, password := getBackupS3Credentials(d)
	body := map[string]interface{}{
		"accessKeyID":      accessKey,
		"secretAccessKey":  secretKey,
		"bucketName":       d.Get("bucket_name").(string),
		"region":           d.Get("region").(string),
		"s3CompatibleHost": d.Get("s3_compatible_host").(string),
		"password":         password,
	}

	if v, ok := d.GetOk("cron_rule"); ok {
		body["cronRule"] = v.(string)
	}

	resp, err := client.DoRequest("POST", "/backup/s3/execute", nil, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to execute S3 backup: %s", string(data))
	}

	d.SetId("portainer_backup_s3")
	return nil
}

func resourceBackupS3Read(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	resp, err := client.DoRequest("GET", "/backup/s3/settings", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to fetch S3 backup settings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read S3 backup settings: %s", string(data))
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
		return fmt.Errorf("failed to decode S3 settings: %w", err)
	}

	d.Set("access_key_id", result.AccessKeyID)
	d.Set("secret_access_key", result.SecretAccessKey)
	d.Set("bucket_name", result.BucketName)
	d.Set("region", result.Region)
	d.Set("s3_compatible_host", result.S3CompatibleHost)
	d.Set("password", result.Password)
	d.Set("cron_rule", result.CronRule)

	d.SetId("portainer_backup_s3")
	return nil
}

func resourceBackupS3Delete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
