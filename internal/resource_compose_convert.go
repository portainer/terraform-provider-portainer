package internal

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceComposeConvertResource() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceComposeConvertCreate,
		ReadContext:   schema.NoopContext,
		UpdateContext: schema.NoopContext,
		DeleteContext: schema.NoopContext,

		Schema: map[string]*schema.Schema{
			"compose_content": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "The content of the docker-compose.yml file.",
			},
			"manifests": {
				Type:        schema.TypeMap,
				Computed:    true,
				Description: "Map of generated Kubernetes YAML manifests.",
				Elem:        &schema.Schema{Type: schema.TypeString},
			},
		},
	}
}

func resourceComposeConvertCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	var diags diag.Diagnostics

	composeContent := d.Get("compose_content").(string)

	tmpDir, err := os.MkdirTemp("", "compose_convert_*")
	if err != nil {
		return diag.Errorf("failed to create temp directory: %s", err)
	}
	defer os.RemoveAll(tmpDir)

	composePath := filepath.Join(tmpDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(composeContent), 0644); err != nil {
		return diag.Errorf("failed to write docker-compose.yml: %s", err)
	}

	cmd := exec.CommandContext(ctx, "kompose", "convert")
	cmd.Dir = tmpDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return diag.Errorf("kompose convert failed: %s\nOutput:\n%s", err, string(out))
	}

	entries, err := os.ReadDir(tmpDir)
	if err != nil {
		return diag.Errorf("failed to read output directory: %s", err)
	}

	manifests := make(map[string]string)
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".yaml") {
			content, err := os.ReadFile(filepath.Join(tmpDir, entry.Name()))
			if err != nil {
				return diag.Errorf("failed to read manifest file: %s", err)
			}
			manifests[entry.Name()] = string(content)
		}
	}

	d.SetId(fmt.Sprintf("convert-%s", filepath.Base(tmpDir)))
	if err := d.Set("manifests", manifests); err != nil {
		return diag.Errorf("failed to set manifests: %s", err)
	}

	return diags
}
