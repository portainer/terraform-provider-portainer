package internal

import (
	"bytes"
	"context"
	"encoding/json"
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

	jsonBody, err := json.Marshal(parsed)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to encode manifest body: %w", err))
	}

	url := fmt.Sprintf("%s/endpoints/%d/kubernetes/apis/rbac.authorization.k8s.io/v1/namespaces/%s/rolebindings", client.Endpoint, endpointID, namespace)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err != nil {
		return diag.FromErr(err)
	}
	req.Header.Set("Content-Type", "application/json")
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return diag.FromErr(fmt.Errorf("no valid authentication method provided (api_key or jwt token)"))
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create Kubernetes Job: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to create Job (%d): %s", resp.StatusCode, string(body)))
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
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return diag.FromErr(fmt.Errorf("no valid authentication method provided (api_key or jwt token)"))
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
