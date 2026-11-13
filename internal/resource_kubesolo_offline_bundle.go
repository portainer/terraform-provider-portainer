package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// kubeSoloOfflineBundleName is the file name KubeSolo publishes offline
// tarballs under, which Portainer requires the upload to carry:
// kubesolo-<version>-linux-<arch>[-musl]-offline.tar.gz.
var kubeSoloOfflineBundleName = regexp.MustCompile(`^kubesolo-[^/]+-linux-[^/]+-offline\.tar\.gz$`)

func resourceKubeSoloOfflineBundle() *schema.Resource {
	return &schema.Resource{
		// PUT /kubesolo/offline-bundles/{name} stores the tarball or replaces
		// the one stored under that name, so create and update run the same
		// upload.
		CreateContext: resourceKubeSoloOfflineBundleUpload,
		ReadContext:   resourceKubeSoloOfflineBundleRead,
		UpdateContext: resourceKubeSoloOfflineBundleUpload,
		DeleteContext: resourceKubeSoloOfflineBundleDelete,
		CustomizeDiff: customizeDiffKubeSoloOfflineBundleHash,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				ValidateFunc: validation.StringMatch(kubeSoloOfflineBundleName, "must be the published tarball name, kubesolo-<version>-linux-<arch>[-musl]-offline.tar.gz"),
				Description:  "File name of the tarball as published with the KubeSolo release, for example `kubesolo-v1.2.3-linux-amd64-offline.tar.gz`. It must be a tarball of the release bundled with Portainer (see `portainer_kubesolo_versions`).",
			},
			"file_path": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Path on the local filesystem to the tarball to upload. Portainer keeps it only if it matches the release's SHA256SUMS.",
			},
			// Computed attributes
			"file_sha256": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "SHA256 hash of the uploaded tarball, used to upload it again when the file at `file_path` changes.",
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
				Description: "Size of the stored tarball in bytes.",
			},
		},
	}
}

// customizeDiffKubeSoloOfflineBundleHash plans an upload when the tarball at
// file_path was replaced without the path changing, the same way edge
// configurations do: Portainer exposes no digest of what it stores.
func customizeDiffKubeSoloOfflineBundleHash(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	fp, _ := d.Get("file_path").(string)
	if fp == "" {
		return nil
	}
	hash, err := sha256File(fp)
	if err != nil {
		return fmt.Errorf("failed to hash file_path %q: %w", fp, err)
	}
	if d.Get("file_sha256").(string) == hash {
		return nil
	}
	return d.SetNew("file_sha256", hash)
}

func kubeSoloOfflineBundleURL(client *APIClient, name string) string {
	return fmt.Sprintf("%s/kubesolo/offline-bundles/%s", client.Endpoint, url.PathEscape(name))
}

// uploadKubeSoloOfflineBundle streams the tarball as the raw request body
// (application/octet-stream). Bundles run to hundreds of megabytes, so the
// file is never read into memory; GetBody reopens it so the retry transport
// can replay the PUT.
func uploadKubeSoloOfflineBundle(ctx context.Context, client *APIClient, name, path string) (*kubeSoloOfflineBundle, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}
	open := func() (io.ReadCloser, error) { return os.Open(path) }
	body, err := open()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut, kubeSoloOfflineBundleURL(client, name), body)
	if err != nil {
		_ = body.Close()
		return nil, err
	}
	req.ContentLength = info.Size()
	req.GetBody = open
	req.Header.Set("Content-Type", "application/octet-stream")
	if err := setAuthHeader(req, client); err != nil {
		_ = body.Close()
		return nil, err
	}

	var bundle kubeSoloOfflineBundle
	if err := sendDecode(client, req, &bundle); err != nil {
		return nil, err
	}
	return &bundle, nil
}

func resourceKubeSoloOfflineBundleUpload(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	name := d.Get("name").(string)
	path := d.Get("file_path").(string)

	hash, err := sha256File(path)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to hash file_path %q: %w", path, err))
	}

	if _, err := uploadKubeSoloOfflineBundle(ctx, client, name, path); err != nil {
		// Only a first upload can 404 for want of the endpoint; on a replace
		// the bundle's own name is in the path.
		if d.Id() == "" {
			err = explainUnsupported(ctx, client, err, "portainer_kubesolo_offline_bundle", requiresBE30)
		}
		return diag.FromErr(fmt.Errorf("failed to upload KubeSolo offline bundle %q: %w", name, err))
	}

	d.SetId(name)
	if err := d.Set("file_sha256", hash); err != nil {
		return diag.FromErr(err)
	}
	return resourceKubeSoloOfflineBundleRead(ctx, d, meta)
}

func resourceKubeSoloOfflineBundleRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// There is no endpoint for one bundle, so it is looked up in the list.
	bundles, err := listKubeSoloOfflineBundles(ctx, client)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to list KubeSolo offline bundles: %w", err))
	}
	for _, b := range bundles {
		if b.Name != d.Id() {
			continue
		}
		if err := setFields(d, map[string]interface{}{
			"name": b.Name,
			"arch": b.Arch,
			"libc": b.Libc,
			"size": int(b.Size),
		}); err != nil {
			return diag.FromErr(err)
		}
		return nil
	}

	d.SetId("")
	return nil
}

func resourceKubeSoloOfflineBundleDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	if err := doJSON(ctx, client, http.MethodDelete, kubeSoloOfflineBundleURL(client, d.Id()), nil, nil); err != nil {
		if isAPINotFound(err) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete KubeSolo offline bundle: %w", err))
	}

	return nil
}
