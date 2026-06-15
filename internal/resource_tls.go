package internal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePortainerUploadTLS() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePortainerUploadTLSCreate,
		ReadContext:   schema.NoopContext,
		UpdateContext: schema.NoopContext,
		DeleteContext: removeFromStateContext,
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

func resourcePortainerUploadTLSCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	certType := d.Get("certificate").(string)
	folder := d.Get("folder").(string)
	filePath := d.Get("file_path").(string)

	file, err := os.Open(filePath)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to open TLS file: %w", err))
	}
	defer file.Close()

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)

	_ = writer.WriteField("folder", folder)
	part, err := writer.CreateFormFile("file", filepath.Base(filePath))
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create form file: %w", err))
	}

	_, err = io.Copy(part, file)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to write file content: %w", err))
	}

	writer.Close()

	uploadURL := fmt.Sprintf("%s/upload/tls/%s", client.Endpoint, certType)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, uploadURL, body)
	if err != nil {
		return diag.FromErr(err)
	}

	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return diag.FromErr(fmt.Errorf("no valid authentication method provided (api_key or jwt token)"))
	}

	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("upload failed: %s", string(respBody)))
	}

	d.SetId(fmt.Sprintf("upload-%s-%s", certType, filepath.Base(filePath)))
	return nil
}
