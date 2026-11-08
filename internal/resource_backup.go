package internal

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceBackup() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceBackupCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: schema.NoopContext,

		Schema: map[string]*schema.Schema{
			"password": {
				Type:         schema.TypeString,
				Optional:     true,
				ExactlyOneOf: []string{"password", "password_wo"},
				Sensitive:    true,
				ForceNew:     true,
				Description:  "Sensitive password used to encrypt the Portainer backup archive. Stored in state as a sensitive value; use `password_wo` to keep it out of state.",
			},
			"password_wo":         writeOnlySecretSchema("password", "password_wo_version", "password used to encrypt the backup archive"),
			"password_wo_version": writeOnlyVersionSchema("password_wo", true),
			"output_path": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Path on local disk where the backup .tar.gz should be saved",
				ForceNew:    true,
			},
		},
	}
}

func resourceBackupCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	password, err := twinSecret(d, "password")
	if err != nil {
		return diag.FromErr(err)
	}
	outputPath := d.Get("output_path").(string)

	body := map[string]interface{}{
		"password": password,
	}

	resp, err := client.DoRequest(ctx, "POST", "/backup", nil, body)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create backup: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to create backup: %s", string(data)))
	}

	// Create output file
	f, err := os.Create(filepath.Clean(outputPath))
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create file at output_path: %w", err))
	}
	defer f.Close()

	_, err = io.Copy(f, resp.Body)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to write backup file: %w", err))
	}

	d.SetId(strconv.FormatInt(makeTimestamp(), 10))
	return nil
}

func makeTimestamp() int64 {
	return int64(os.Getpid()) + int64(os.Getppid())
}
