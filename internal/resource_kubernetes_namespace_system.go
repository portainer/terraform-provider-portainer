package internal

import (
	"context"
	"fmt"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceKubernetesNamespaceSystem() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesNamespaceSystemToggle,
		ReadContext:   resourceKubernetesNamespaceSystemRead,
		UpdateContext: resourceKubernetesNamespaceSystemToggle,
		DeleteContext: resourceKubernetesNamespaceSystemUnset,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the Portainer Kubernetes environment owning the namespace.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Kubernetes namespace whose system flag is being toggled.",
			},
			"system": {
				Type:        schema.TypeBool,
				Required:    true,
				Description: "Whether the namespace should be marked as a Portainer system namespace.",
			},
		},
	}
}

func resourceKubernetesNamespaceSystemToggle(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)
	system := d.Get("system").(bool)

	body := map[string]interface{}{
		"system": system,
	}

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/system", client.Endpoint, id, namespace)

	if err := doJSON(ctx, client, http.MethodPut, url, body, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to toggle namespace system state: %w", err))
	}

	d.SetId(fmt.Sprintf("%d:%s", id, namespace))
	return nil
}

func resourceKubernetesNamespaceSystemRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	parts := strings.SplitN(d.Id(), ":", 2)
	if len(parts) != 2 {
		return diag.FromErr(fmt.Errorf("invalid ID format, expected 'envID:namespace': %s", d.Id()))
	}
	var envID int
	fmt.Sscanf(parts[0], "%d", &envID)
	namespace := parts[1]
	if envID == 0 || namespace == "" {
		return diag.FromErr(fmt.Errorf("invalid ID format, expected 'envID:namespace': %s", d.Id()))
	}

	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s", client.Endpoint, envID, namespace)

	var ns struct {
		IsSystem bool `json:"IsSystem"`
	}
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &ns); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read namespace %q: %w", namespace, err))
	}

	if err := d.Set("environment_id", envID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("namespace", namespace); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("system", ns.IsSystem); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceKubernetesNamespaceSystemUnset(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	if err := d.Set("system", false); err != nil {
		return diag.FromErr(err)
	}
	return resourceKubernetesNamespaceSystemToggle(ctx, d, meta)
}
