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

func resourceKubernetesRoleBindings() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesRoleBindingsCreate,
		ReadContext:   resourceKubernetesRoleBindingsRead,
		UpdateContext: resourceKubernetesRoleBindingsUpdate,
		DeleteContext: resourceKubernetesRoleBindingsDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer Kubernetes environment (endpoint) where the RoleBinding is managed. Changing this value forces resource recreation.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Kubernetes namespace in which the RoleBinding is created.",
			},
			"manifest": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Raw YAML or JSON manifest defining the Kubernetes RoleBinding.",
			},
		},
	}
}

func resourceKubernetesRoleBindingsCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	endpointID := d.Get("endpoint_id").(int)
	namespace := d.Get("namespace").(string)
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

	url := fmt.Sprintf("%s/endpoints/%d/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/%s/rolebindings", client.Endpoint, endpointID, namespace)

	if err := doJSON(ctx, client, http.MethodPost, url, parsed, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create Kubernetes Job: %w", err))
	}

	d.SetId(fmt.Sprintf("%d:%s:%s", endpointID, namespace, name))
	return nil
}

func resourceKubernetesRoleBindingsDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	endpointID, namespace, name := parseRoleBindingsID(d.Id())

	url := fmt.Sprintf("%s/endpoints/%d/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/%s/rolebindings/%s", client.Endpoint, endpointID, namespace, name)

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

func resourceKubernetesRoleBindingsUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if diags := resourceKubernetesRoleBindingsDelete(ctx, d, meta); diags.HasError() {
		return diags
	}
	return resourceKubernetesRoleBindingsCreate(ctx, d, meta)
}

func resourceKubernetesRoleBindingsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	endpointID, namespace, name := parseRoleBindingsID(d.Id())
	if endpointID == 0 || namespace == "" || name == "" {
		return diag.FromErr(fmt.Errorf("invalid ID format, expected 'endpointID:namespace:name': %s", d.Id()))
	}

	url := fmt.Sprintf("%s/endpoints/%d/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/%s/rolebindings/%s", client.Endpoint, endpointID, namespace, name)
	if diags := k8sConfirmExistsByGET(ctx, d, client, url, "rolebinding "+name); diags.HasError() {
		return diags
	}
	if d.Id() == "" {
		return nil
	}

	if err := d.Set("endpoint_id", endpointID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("namespace", namespace); err != nil {
		return diag.FromErr(err)
	}
	// "manifest" intentionally not refreshed — see k8sConfirmExistsByGET.
	return nil
}

func parseRoleBindingsID(id string) (endpointID int, namespace string, name string) {
	parts := strings.SplitN(id, ":", 3)
	if len(parts) != 3 {
		return 0, "", ""
	}
	fmt.Sscanf(parts[0], "%d", &endpointID)
	namespace = parts[1]
	name = parts[2]
	return
}
