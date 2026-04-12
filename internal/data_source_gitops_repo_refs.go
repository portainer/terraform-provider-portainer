package internal

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceGitopsRepoRefs() *schema.Resource {
	return &schema.Resource{
		Read: dataSourceGitopsRepoRefsRead,

		Schema: map[string]*schema.Schema{
			"repository_url": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "URL of the Git repository",
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
			"refs": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "List of Git references (branches and tags)",
			},
		},
	}
}

func dataSourceGitopsRepoRefsRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	repoURL := d.Get("repository_url").(string)

	payload := map[string]interface{}{
		"repository": repoURL,
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

	resp, err := client.DoRequest("POST", "/gitops/repo/refs", nil, payload)
	if err != nil {
		return fmt.Errorf("failed to list Git repository refs: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to list Git repository refs (status %d): %s", resp.StatusCode, string(data))
	}

	var refs []string
	if err := json.NewDecoder(resp.Body).Decode(&refs); err != nil {
		return fmt.Errorf("failed to decode Git refs response: %w", err)
	}

	d.SetId(fmt.Sprintf("gitops-repo-refs-%s", repoURL))
	d.Set("refs", refs)

	return nil
}
