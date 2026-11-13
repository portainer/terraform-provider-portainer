package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// dataSourceGitopsWorkflowDeploymentResources lists the Kubernetes resources a
// reconciled deployment declares, read from the manifests Portainer
// materialised for it - one copy of what every targeted environment applies,
// at the revision the deployment last assembled. A Helm chart lists nothing:
// what a release contains is only known once Helm has rendered it.
func dataSourceGitopsWorkflowDeploymentResources() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceGitopsWorkflowDeploymentResourcesRead,

		Schema: map[string]*schema.Schema{
			"workflow_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the GitOps workflow.",
			},
			"deployment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Identifier of the reconciled deployment: the `artifact_id` of an artifact of a `portainer_gitops_workflow` on the `reconcile` engine.",
			},
			// Computed attributes
			"resources": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Kubernetes resources the deployment declares.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"api_version": {Type: schema.TypeString, Computed: true, Description: "API version of the resource, for example `apps/v1`."},
						"kind":        {Type: schema.TypeString, Computed: true, Description: "Kind of the resource, for example `Deployment`."},
						"name":        {Type: schema.TypeString, Computed: true, Description: "Name of the resource."},
						"namespace":   {Type: schema.TypeString, Computed: true, Description: "Namespace of the resource, empty for a cluster-scoped one."},
					},
				},
			},
		},
	}
}

func dataSourceGitopsWorkflowDeploymentResourcesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	workflowID := d.Get("workflow_id").(int)
	deploymentID := d.Get("deployment_id").(int)

	var list []struct {
		APIVersion string `json:"apiVersion"`
		Kind       string `json:"kind"`
		Name       string `json:"name"`
		Namespace  string `json:"namespace"`
	}
	endpoint := fmt.Sprintf("%s/gitops/workflows/%d/deployments/%d/resources", client.Endpoint, workflowID, deploymentID)
	if err := doJSON(ctx, client, http.MethodGet, endpoint, nil, &list); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_gitops_workflow_deployment_resources", requiresBE30)
		return diag.FromErr(fmt.Errorf("failed to read the resources of deployment %d of GitOps workflow %d: %w", deploymentID, workflowID, err))
	}

	resources := make([]map[string]interface{}, len(list))
	for i, r := range list {
		resources[i] = map[string]interface{}{
			"api_version": r.APIVersion, "kind": r.Kind, "name": r.Name, "namespace": r.Namespace,
		}
	}
	if err := setFields(d, map[string]interface{}{"resources": resources}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(fmt.Sprintf("%d:%d", workflowID, deploymentID))
	return nil
}
