package internal

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceGitopsRepoFile() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceGitopsRepoFileRead,

		Schema: map[string]*schema.Schema{
			"repository_url": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "URL of the Git repository",
			},
			"reference": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Git reference (e.g. refs/heads/master)",
			},
			"target_file": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Path to the file whose content will be read",
			},
			"username": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Username for repository authentication",
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Password for repository authentication",
			},
			"git_credential_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Git credential ID for authentication",
			},
			"tls_skip_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Skip TLS verification when cloning the Git repository",
			},
			// Computed output
			"file_content": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Content of the file from the Git repository",
			},
		},
	}
}

func dataSourceGitopsRepoFileRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	repoURL := d.Get("repository_url").(string)

	payload := map[string]interface{}{
		"repository": repoURL,
	}

	if v, ok := d.GetOk("reference"); ok {
		payload["reference"] = v.(string)
	}
	if v, ok := d.GetOk("target_file"); ok {
		payload["targetFile"] = v.(string)
	}
	if v, ok := d.GetOk("username"); ok {
		payload["username"] = v.(string)
	}
	if v, ok := d.GetOk("password"); ok {
		payload["password"] = v.(string)
	}
	if v, ok := d.GetOk("git_credential_id"); ok {
		payload["gitCredentialID"] = v.(int)
	}
	if v, ok := d.GetOk("tls_skip_verify"); ok {
		payload["TLSSkipVerify"] = v.(bool)
	}

	resp, err := client.DoRequest("POST", "/gitops/repo/file/preview", nil, payload)
	if err != nil {
		return fmt.Errorf("failed to preview Git repository file: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to preview Git repository file (status %d): %s", resp.StatusCode, string(data))
	}

	var result struct {
		FileContent string `json:"FileContent"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode file preview response: %w", err)
	}

	d.SetId(fmt.Sprintf("gitops-repo-file-%s-%s", repoURL, d.Get("target_file").(string)))
	d.Set("file_content", result.FileContent)

	return nil
}
