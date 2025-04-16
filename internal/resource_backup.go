package internal

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceBackup() *schema.Resource {
	return &schema.Resource{
		Create: resourceBackupCreate,
		Read:   schema.Noop,
		Delete: schema.Noop,

		Schema: map[string]*schema.Schema{
			"password": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
				ForceNew:  true,
			},
			"output_path": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Path on local disk where the backup .tar.gz should be saved",
				ForceNew:    true,
			},
		},
	}
}

func resourceBackupCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	password := d.Get("password").(string)
	outputPath := d.Get("output_path").(string)

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

	// Create output file
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
