package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// resourceGitopsWorkflowReconcile asks a workflow on the reconcile engine to
// re-read its sources and every environment running it to reconcile now,
// instead of at its next interval. It is a one-shot action, so it has no read
// or update: the resource records that a reconcile was asked for, and
// `triggers` is how another one is.
//
// Portainer does not wait for the environments: they apply at their next
// check-in, so success means the request was accepted, not that it applied.
func resourceGitopsWorkflowReconcile() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceGitopsWorkflowReconcileCreate,
		ReadContext:   schema.NoopContext,
		DeleteContext: schema.NoopContext,

		Schema: map[string]*schema.Schema{
			"workflow_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the GitOps workflow to reconcile. The workflow has to be on the `reconcile` engine.",
			},
			"triggers": {
				Type:        schema.TypeMap,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Arbitrary values that force another reconcile when they change. A reconcile is a one-shot action, so without a trigger the resource runs once and then stays put.",
			},
		},
	}
}

func resourceGitopsWorkflowReconcileCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	workflowID := d.Get("workflow_id").(int)

	endpoint := fmt.Sprintf("%s/gitops/workflows/%d/reconcile", client.Endpoint, workflowID)
	if err := doJSON(ctx, client, http.MethodPost, endpoint, nil, nil); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_gitops_workflow_reconcile", requiresBE30)
		return diag.FromErr(fmt.Errorf("failed to reconcile GitOps workflow %d: %w", workflowID, err))
	}

	d.SetId(fmt.Sprintf("%d-reconcile-%d", workflowID, makeTimestamp()))
	return nil
}
