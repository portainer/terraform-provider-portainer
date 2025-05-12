package internal

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePortainerUploadTLS() *schema.Resource {
	return &schema.Resource{
		Create: resourcePortainerUploadTLSCreate,
		Read:   schema.Noop,
		Update: schema.Noop,
		Delete: schema.RemoveFromState,
		Schema: map[string]*schema.Schema{
			"certificate": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Type of TLS certificate: one of 'ca', 'cert', 'key'",
			},
			"folder": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Folder name where the TLS file will be stored",
			},
			"file_path": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Path to the local TLS file to upload",
			},
		},
	}
}

func resourcePortainerUploadTLSCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	certType := d.Get("certificate").(string)
	folder := d.Get("folder").(string)
	filePath := d.Get("file_path").(string)

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("failed to open TLS file: %w", err)
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	_ = writer.WriteField("folder", folder)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return fmt.Errorf("failed to create form file: %w", err)
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return fmt.Errorf("failed to write file content: %w", err)
	}

	writer.Close()

	uploadURL := fmt.Sprintf("%s/upload/tls/%s", client.Endpoint, certType)
	req, err := http.NewRequest("POST", uploadURL, body)
	if err != nil {
		return err
	}

	req.Header.Set("X-API-Key", client.APIKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("upload failed: %s", string(respBody))
	}

	d.SetId(fmt.Sprintf("upload-%s-%s", certType, filepath.Base(filePath)))
	return nil
}
