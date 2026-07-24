package internal

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceKubernetesClusterRoles() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesClusterRolesCreate,
		ReadContext:   resourceKubernetesClusterRolesRead,
		UpdateContext: resourceKubernetesClusterRolesUpdate,
		DeleteContext: resourceKubernetesClusterRolesDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer Kubernetes environment (endpoint) where the cluster-scoped ClusterRole is managed. Changing this value forces resource recreation.",
			},
			"manifest": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Raw YAML or JSON manifest defining the Kubernetes ClusterRole.",
			},
		},
	}
}

func resourceKubernetesClusterRolesCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	endpointID := d.Get("endpoint_id").(int)
	manifest := d.Get("manifest").(string)

	parsed, err := parseManifest(manifest)
	if err != nil {
		return diag.FromErr(fmt.Errorf("manifest must be valid JSON or YAML: %w", err))
	}

	metadata, ok := parsed["metadata"].(map[string]interface{})
	if !ok {
		return diag.FromErr(fmt.Errorf("missing metadata in manifest"))
	}
	name, ok := metadata["name"].(string)
	if !ok || name == "" {
		return diag.FromErr(fmt.Errorf("missing metadata.name in manifest"))
	}

	url := fmt.Sprintf("%s/endpoints/%d/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterroles", client.Endpoint, endpointID)

	if err := doJSON(ctx, client, http.MethodPost, url, parsed, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create Kubernetes Job: %w", err))
	}

	d.SetId(fmt.Sprintf("%d:%s", endpointID, name))
	return nil
}

func resourceKubernetesClusterRolesDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	endpointID, name := parseClusterRolesID(d.Id())

	url := fmt.Sprintf("%s/endpoints/%d/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterroles/%s", client.Endpoint, endpointID, name)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return diag.FromErr(err)
	}
	if err := setAuthHeader(req, client); err != nil {
		return diag.FromErr(err)
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to delete Job: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to delete Job: %s", string(body)))
	}

	d.SetId("")
	return nil
}

func resourceKubernetesClusterRolesUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if diags := resourceKubernetesClusterRolesDelete(ctx, d, meta); diags.HasError() {
		return diags
	}
	return resourceKubernetesClusterRolesCreate(ctx, d, meta)
}

func resourceKubernetesClusterRolesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	endpointID, name := parseClusterRolesID(d.Id())
	if endpointID == 0 || name == "" {
		return diag.FromErr(fmt.Errorf("invalid ID format, expected 'endpointID:name': %s", d.Id()))
	}

	url := fmt.Sprintf("%s/endpoints/%d/kubernetes/apis/rbac.authorization.k8s.io/v1/clusterroles/%s", client.Endpoint, endpointID, name)
	if diags := k8sConfirmExistsByGET(ctx, d, client, url, "clusterrole "+name); diags.HasError() {
		return diags
	}
	if d.Id() == "" {
		return nil
	}

	if err := d.Set("endpoint_id", endpointID); err != nil {
		return diag.FromErr(err)
	}
	// "manifest" intentionally not refreshed — see k8sConfirmExistsByGET.
	return nil
}

func parseClusterRolesID(id string) (endpointID int, name string) {
	parts := strings.SplitN(id, ":", 3)
	if len(parts) < 2 {
		return 0, ""
	}
	fmt.Sscanf(parts[0], "%d", &endpointID)
	name = parts[1]
	return
}
