package internal

import (
	"fmt"
	"io"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceBackupS3() *schema.Resource {
	return &schema.Resource{
		Create: resourceBackupS3Create,
		Read:   resourceBackupS3Read,
		Delete: resourceBackupS3Delete,
		Schema: map[string]*schema.Schema{
			"access_key_id": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
				ForceNew:  true,
			},
			"secret_access_key": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
				ForceNew:  true,
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
			"password": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
				ForceNew:  true,
			},
			"cron_rule": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
		},
	}
}

func resourceBackupS3Create(d *schema.ResourceData, meta interface{}) error {
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
	// Backup is one-time action; nothing to read. You can optionally clear the ID to mark as destroyed.
	return nil
}

func resourceBackupS3Delete(d *schema.ResourceData, meta interface{}) error {
	// This operation cannot be undone via API; just remove from state.
	d.SetId("")
	return nil
}
