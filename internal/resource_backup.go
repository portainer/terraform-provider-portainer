package internal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceBackup() *schema.Resource {
	return &schema.Resource{
		Create: resourceBackupCreate,
		Read:   schema.Noop,
		Delete: schema.Noop,

		Schema: map[string]*schema.Schema{
			"password": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				ForceNew:      true,
				ConflictsWith: []string{"password_wo", "backup_wo_version"},
				Description:   "Password used to encrypt the backup file (stored in Terraform state).",
			},
			"password_wo": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				WriteOnly:     true,
				RequiredWith:  []string{"backup_wo_version"},
				ConflictsWith: []string{"password"},
				Description:   "Ephemeral write-only password used for backup encryption (not stored in state).",
			},
			"backup_wo_version": {
				Type:          schema.TypeInt,
				Optional:      true,
				ForceNew:      true,
				Description:   "Version flag to trigger recreation when using ephemeral password.",
				ConflictsWith: []string{"password"},
			},
			"output_path": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Path on local disk where the backup .tar.gz file will be saved.",
				ForceNew:    true,
			},
		},
	}
}

func getBackupPassword(d *schema.ResourceData) string {
	password := d.Get("password").(string)

	if d.Get("backup_wo_version").(int) != 0 {
		if raw, diag := d.GetRawConfigAt(cty.GetAttrPath("password_wo")); diag == nil && raw.IsKnown() && !raw.IsNull() {
			password = raw.AsString()
		}
	}

	return password
}

func resourceBackupCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	password := getBackupPassword(d)
	outputPath := d.Get("output_path").(string)

	if password == "" {
		return fmt.Errorf("backup password cannot be empty")
	}

	body := map[string]interface{}{
		"password": password,
	}

	resp, err := client.DoRequest("POST", "/backup", nil, body)
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create backup: %s", string(data))
	}

	// Create output file safely
	f, err := os.Create(filepath.Clean(outputPath))
	if err != nil {
		return fmt.Errorf("failed to create file at output_path: %w", err)
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return fmt.Errorf("failed to write backup file: %w", err)
	}

	d.SetId(strconv.FormatInt(makeTimestamp(), 10))
	return nil
}

func makeTimestamp() int64 {
	return int64(os.Getpid()) + int64(os.Getppid())
}
