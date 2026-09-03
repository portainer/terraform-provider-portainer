package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceKubernetesDeploymentRollback() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceKubernetesDeploymentRollbackCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: schema.NoopContext,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the Portainer Kubernetes environment the deployment belongs to.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Namespace the deployment lives in.",
			},
			"deployment_name": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the deployment to roll back.",
			},
			"revision": {
				Type:         schema.TypeInt,
				Optional:     true,
				ForceNew:     true,
				Default:      0,
				ValidateFunc: validation.IntAtLeast(0),
				Description:  "Rollout revision to roll back to. `0` (the default) rolls back to the previous revision, matching `kubectl rollout undo`. Use the `portainer_kubernetes_replicasets` data source to discover the available revisions.",
			},
			// Computed attributes
			"rolled_back_to_replicas": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Desired replica count of the deployment after the rollback.",
			},
		},
	}
}

func resourceKubernetesDeploymentRollbackCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	envID := d.Get("environment_id").(int)
	namespace := d.Get("namespace").(string)
	name := d.Get("deployment_name").(string)
	revision := d.Get("revision").(int)

	var deployment k8sDeployment
	url := fmt.Sprintf("%s/kubernetes/%d/namespaces/%s/deployments/%s/rollback", client.Endpoint, envID, namespace, name)
	if err := doJSON(ctx, client, http.MethodPost, url, map[string]interface{}{"revision": revision}, &deployment); err != nil {
		return diag.FromErr(fmt.Errorf("failed to roll deployment %q in namespace %q back to revision %d: %w", name, namespace, revision, err))
	}

	replicas := 1
	if deployment.Spec.Replicas != nil {
		replicas = *deployment.Spec.Replicas
	}
	if err := d.Set("rolled_back_to_replicas", replicas); err != nil {
		return diag.FromErr(err)
	}

	// A rollback is a one-shot action, so the ID carries a timestamp: re-running
	// it means replacing the resource, the same pattern portainer_helm_rollback
	// uses.
	d.SetId(fmt.Sprintf("%d/%s/%s/rollback/%d", envID, namespace, name, makeTimestamp()))
	return nil
}
