package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// newKubernetesScopedKubeconfigsDataSource lists issued scoped kubeconfigs,
// for one environment or across all of them. Portainer never returns the
// credential after issuing it, so nothing listed here is secret.
func newKubernetesScopedKubeconfigsDataSource() datasource.DataSource {
	return newK8sDataSource("kubernetes_scoped_kubeconfigs", schema.Schema{
		Attributes: map[string]schema.Attribute{
			"environment_id": schema.Int64Attribute{
				Optional:    true,
				Description: "Identifier of the Kubernetes environment to list the scoped kubeconfigs of. Leave unset to list them across every environment.",
			},
			// Computed attributes
			"scoped_kubeconfigs": k8sDSObjectList("Scoped kubeconfigs, newest first. A user sees the ones they issued; an administrator sees every one.",
				k8sDSField{"id", types.Int64Type, "Portainer identifier of the scoped kubeconfig."},
				k8sDSField{"environment_id", types.Int64Type, "Environment the kubeconfig gives access to."},
				k8sDSField{"user_id", types.Int64Type, "Identifier of the user who issued the kubeconfig."},
				k8sDSField{"username", types.StringType, "Name of the user who issued the kubeconfig."},
				k8sDSField{"description", types.StringType, "Description given when the kubeconfig was issued."},
				k8sDSField{"namespaces", k8sDSStringList, "Namespaces the kubeconfig covers."},
				k8sDSField{"include_secrets", types.BoolType, "Whether the kubeconfig can read Secrets."},
				k8sDSField{"service_account", types.StringType, "Kubernetes service account the kubeconfig authenticates as."},
				k8sDSField{"active", types.BoolType, "False once the kubeconfig has expired, or was revoked but its Kubernetes objects could not be removed."},
				k8sDSField{"created_at", types.Int64Type, "Time the kubeconfig was issued, as a Unix timestamp."},
				k8sDSField{"expires_at", types.Int64Type, "Time the kubeconfig stops working, as a Unix timestamp."},
			),
		},
	}, nil, kubernetesScopedKubeconfigsRead)
}

func kubernetesScopedKubeconfigsRead(ctx context.Context, client *APIClient, d *k8sDSData) error {
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
		return fmt.Errorf("failed to list scoped kubeconfigs: %w", err)
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
		return err
	}

	d.SetID(id)
	return nil
}
