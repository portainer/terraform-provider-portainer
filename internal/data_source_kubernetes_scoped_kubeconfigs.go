package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// dataSourceKubernetesScopedKubeconfigs lists issued scoped kubeconfigs, for
// one environment or across all of them. Portainer never returns the
// credential after issuing it, so nothing listed here is secret.
func dataSourceKubernetesScopedKubeconfigs() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceKubernetesScopedKubeconfigsRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Identifier of the Kubernetes environment to list the scoped kubeconfigs of. Leave unset to list them across every environment.",
			},
			// Computed attributes
			"scoped_kubeconfigs": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Scoped kubeconfigs, newest first. A user sees the ones they issued; an administrator sees every one.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Portainer identifier of the scoped kubeconfig.",
						},
						"environment_id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Environment the kubeconfig gives access to.",
						},
						"user_id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Identifier of the user who issued the kubeconfig.",
						},
						"username": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name of the user who issued the kubeconfig.",
						},
						"description": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Description given when the kubeconfig was issued.",
						},
						"namespaces": {
							Type:        schema.TypeList,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Namespaces the kubeconfig covers.",
						},
						"include_secrets": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "Whether the kubeconfig can read Secrets.",
						},
						"service_account": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Kubernetes service account the kubeconfig authenticates as.",
						},
						"active": {
							Type:        schema.TypeBool,
							Computed:    true,
							Description: "False once the kubeconfig has expired, or was revoked but its Kubernetes objects could not be removed.",
						},
						"created_at": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Time the kubeconfig was issued, as a Unix timestamp.",
						},
						"expires_at": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Time the kubeconfig stops working, as a Unix timestamp.",
						},
					},
				},
			},
		},
	}
}

func dataSourceKubernetesScopedKubeconfigsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	url := client.Endpoint + "/kubernetes/scoped_kubeconfigs"
	id := "portainer-scoped-kubeconfigs"
	envID, perEnvironment := d.GetOk("environment_id")
	if perEnvironment {
		url = fmt.Sprintf("%s/kubernetes/%d/scoped_kubeconfig", client.Endpoint, envID.(int))
		id = strconv.Itoa(envID.(int)) + "/scoped_kubeconfigs"
	}

	var items []scopedKubeconfigStatus
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &items); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_kubernetes_scoped_kubeconfigs", requiresBE30)
		return diag.FromErr(fmt.Errorf("failed to list scoped kubeconfigs: %w", err))
	}

	list := make([]interface{}, 0, len(items))
	for _, item := range items {
		list = append(list, map[string]interface{}{
			"id":              item.ID,
			"environment_id":  item.EndpointID,
			"user_id":         item.UserID,
			"username":        item.Username,
			"description":     item.Description,
			"namespaces":      item.Namespaces,
			"include_secrets": item.IncludeSecrets,
			"service_account": item.ServiceAccount,
			"active":          item.Active,
			"created_at":      item.CreatedAt,
			"expires_at":      item.ExpiresAt,
		})
	}

	if err := d.Set("scoped_kubeconfigs", list); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(id)
	return nil
}
