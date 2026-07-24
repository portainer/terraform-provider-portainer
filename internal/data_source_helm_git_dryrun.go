package internal

import (
	"context"
	"fmt"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceHelmGitDryRun() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceHelmGitDryRunRead,

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (Endpoint) identifier",
			},
			"repository_url": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "URL of the Git repository containing the Helm chart",
			},
			"reference_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Git reference name (e.g. refs/heads/main)",
			},
			"chart_path": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Path to the Helm chart in the repository",
			},
			"values_files": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "List of Helm values files to use",
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Kubernetes namespace for the release",
			},
			"release_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Name of the Helm release",
			},
			"repository_authentication": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether the repository requires authentication",
			},
			"repository_username": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Username for repository authentication",
			},
			"repository_password": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Password for repository authentication",
			},
			"repository_git_credential_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Git credential ID for repository authentication",
			},
			"tls_skip_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Skip TLS verification when cloning the repository",
			},
			// Computed outputs
			"manifest": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Rendered manifest from the dry run",
			},
			"release_version": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Version (revision) of the release",
			},
		},
	}
}

func dataSourceHelmGitDryRunRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	payload := map[string]interface{}{
		"repositoryURL": d.Get("repository_url").(string),
	}

	if v, ok := d.GetOk("reference_name"); ok {
		payload["repositoryReferenceName"] = v.(string)
	}
	if v, ok := d.GetOk("chart_path"); ok {
		payload["helmChartPath"] = v.(string)
	}
	if v, ok := d.GetOk("values_files"); ok {
		files := make([]string, 0)
		for _, f := range v.([]interface{}) {
			files = append(files, f.(string))
		}
		payload["helmValuesFiles"] = files
	}
	if v, ok := d.GetOk("namespace"); ok {
		payload["namespace"] = v.(string)
	}
	if v, ok := d.GetOk("release_name"); ok {
		payload["name"] = v.(string)
	}
	if v, ok := d.GetOk("repository_authentication"); ok {
		payload["repositoryAuthentication"] = v.(bool)
	}
	if v, ok := d.GetOk("repository_username"); ok {
		payload["repositoryUsername"] = v.(string)
	}
	if v, ok := d.GetOk("repository_password"); ok {
		payload["repositoryPassword"] = v.(string)
	}
	if v, ok := d.GetOk("repository_git_credential_id"); ok {
		payload["repositoryGitCredentialID"] = v.(int)
	}
	if v, ok := d.GetOk("tls_skip_verify"); ok {
		payload["tlsSkipVerify"] = v.(bool)
	}

	var result struct {
		Manifest  string `json:"manifest"`
		Name      string `json:"name"`
		Namespace string `json:"namespace"`
		Version   int    `json:"version"`
	}
	if err := doJSON(ctx, client, "POST", fmt.Sprintf("%s/endpoints/%d/kubernetes/helm/git/dryrun", client.Endpoint, endpointID), payload, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to perform Helm Git dry run: %w", err))
	}

	d.SetId(fmt.Sprintf("helm-git-dryrun-%d-%s", endpointID, d.Get("repository_url").(string)))
	if err := d.Set("manifest", result.Manifest); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("release_version", result.Version); err != nil {
		return diag.FromErr(err)
	}

	return nil
}
