package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// The KubeSolo install flow itself (POST /kubesolo/tests, /kubesolo/install
// and the retry/cancel actions) is not modelled: it is an asynchronous SSH job
// over many machines that Portainer holds one of at a time, with host key
// confirmation in between, which no Terraform resource lifecycle fits. The
// data sources below read what that flow and a running KubeSolo report.

func dataSourceKubeSoloVersions() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubeSoloVersionsRead,

		Schema: map[string]*schema.Schema{
			"bundled": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "KubeSolo release Portainer was built with. It is installed by default and on machines that cannot reach the internet.",
			},
			"minimum": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Oldest KubeSolo release Portainer installs.",
			},
			"versions": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "KubeSolo releases Portainer can install, newest first. Only the bundled one when GitHub cannot be reached.",
			},
			"github_error": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Why Portainer could not list the releases on GitHub, empty when it could.",
			},
		},
	}
}

func dataSourceKubeSoloVersionsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var versions struct {
		Bundled     string   `json:"bundled"`
		Minimum     string   `json:"minimum"`
		Versions    []string `json:"versions"`
		GithubError string   `json:"githubError"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/kubesolo/versions", nil, &versions); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_kubesolo_versions", requiresBE30)
		return diag.FromErr(fmt.Errorf("failed to list KubeSolo versions: %w", err))
	}

	if err := setFields(d, map[string]interface{}{
		"bundled":      versions.Bundled,
		"minimum":      versions.Minimum,
		"versions":     versions.Versions,
		"github_error": versions.GithubError,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("portainer-kubesolo-versions")
	return nil
}

// kubeSoloOfflineBundle is one uploaded offline tarball, as listed by GET
// /kubesolo/offline-bundles and returned by the upload.
type kubeSoloOfflineBundle struct {
	Name string `json:"name"`
	Arch string `json:"arch"`
	Libc string `json:"libc"`
	Size int64  `json:"size"`
}

// listKubeSoloOfflineBundles returns every uploaded KubeSolo offline bundle.
func listKubeSoloOfflineBundles(ctx context.Context, client *APIClient) ([]kubeSoloOfflineBundle, error) {
	var bundles []kubeSoloOfflineBundle
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/kubesolo/offline-bundles", nil, &bundles); err != nil {
		return nil, err
	}
	return bundles, nil
}

func dataSourceKubeSoloOfflineBundles() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubeSoloOfflineBundlesRead,

		Schema: map[string]*schema.Schema{
			"bundles": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Offline tarballs of the bundled KubeSolo release uploaded for machines that cannot reach the internet.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "File name of the tarball, for example `kubesolo-v1.2.3-linux-amd64-offline.tar.gz`.",
						},
						"arch": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "CPU architecture the bundle installs on, for example `amd64`.",
						},
						"libc": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "C library of the machines the bundle installs on: `glibc` or `musl`.",
						},
						"size": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Size of the tarball in bytes.",
						},
					},
				},
			},
		},
	}
}

func dataSourceKubeSoloOfflineBundlesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	bundles, err := listKubeSoloOfflineBundles(ctx, client)
	if err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_kubesolo_offline_bundles", requiresBE30)
		return diag.FromErr(fmt.Errorf("failed to list KubeSolo offline bundles: %w", err))
	}

	items := make([]map[string]interface{}, 0, len(bundles))
	for _, b := range bundles {
		items = append(items, map[string]interface{}{
			"name": b.Name,
			"arch": b.Arch,
			"libc": b.Libc,
			"size": int(b.Size),
		})
	}
	if err := d.Set("bundles", items); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("portainer-kubesolo-offline-bundles")
	return nil
}

func dataSourceKubeSoloConfig() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubeSoloConfigRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the KubeSolo environment (a Kubernetes Edge Agent environment).",
			},
			// Computed attributes
			"api": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Whether the agent could reach the KubeSolo API: `available`, `notConfigured` or `unreachable`.",
			},
			"version": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "KubeSolo version running on the machine.",
			},
			"config_json": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "KubeSolo configuration as a JSON object. Sensitive: it carries KubeSolo's secrets, such as the Edge key. Decode it with `jsondecode`.",
			},
			"error": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Why the configuration could not be read, empty when it could.",
			},
			"read_at": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "When the snapshot that carried the configuration was taken, as a Unix timestamp.",
			},
		},
	}
}

func dataSourceKubeSoloConfigRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID := d.Get("environment_id").(int)

	// The configuration comes from the environment's latest snapshot, so a
	// 404 can also mean the agent has not sent one with KubeSolo's state yet.
	var resp struct {
		API     string          `json:"api"`
		Version string          `json:"version"`
		Config  json.RawMessage `json:"config"`
		Error   string          `json:"error"`
		ReadAt  int64           `json:"readAt"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/kubesolo/environments/%d/config", client.Endpoint, envID), nil, &resp); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_kubesolo_config", requiresBE30)
		return diag.FromErr(fmt.Errorf("failed to read the KubeSolo configuration of environment %d: %w", envID, err))
	}

	config := ""
	if len(resp.Config) > 0 && string(resp.Config) != "null" {
		config = string(resp.Config)
	}

	if err := setFields(d, map[string]interface{}{
		"api":         resp.API,
		"version":     resp.Version,
		"config_json": config,
		"error":       resp.Error,
		"read_at":     int(resp.ReadAt),
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(envID))
	return nil
}
