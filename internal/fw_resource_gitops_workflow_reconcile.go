package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// portainer_gitops_workflow_reconcile asks a workflow on the reconcile engine
// to re-read its sources and every environment running it to reconcile now,
// instead of at its next interval. It is a one-shot action, so it has no read
// or update: the resource records that a reconcile was asked for, and
// `triggers` is how another one is.
//
// Portainer does not wait for the environments: they apply at their next
// check-in, so success means the request was accepted, not that it applied.

var (
	_ resource.Resource              = (*gitopsWorkflowReconcileResource)(nil)
	_ resource.ResourceWithConfigure = (*gitopsWorkflowReconcileResource)(nil)
)

func newGitopsWorkflowReconcileResource() resource.Resource {
	return &gitopsWorkflowReconcileResource{}
}

type gitopsWorkflowReconcileResource struct{ client *APIClient }

type gitopsWorkflowReconcileResModel struct {
	ID         types.String `tfsdk:"id"`
	WorkflowID types.Int64  `tfsdk:"workflow_id"`
	Triggers   types.Map    `tfsdk:"triggers"`
}

func (r *gitopsWorkflowReconcileResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gitops_workflow_reconcile"
}

func (r *gitopsWorkflowReconcileResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"workflow_id": schema.Int64Attribute{
				Required:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				Description:   "Identifier of the GitOps workflow to reconcile. The workflow has to be on the `reconcile` engine.",
			},
			"triggers": schema.MapAttribute{
				Optional:      true,
				ElementType:   types.StringType,
				PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()},
				Description:   "Arbitrary values that force another reconcile when they change. A reconcile is a one-shot action, so without a trigger the resource runs once and then stays put.",
			},
		},
	}
}

func (r *gitopsWorkflowReconcileResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *gitopsWorkflowReconcileResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan gitopsWorkflowReconcileResModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	gitopsWorkflowReconcileRun(ctx, r.client, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// gitopsWorkflowReconcileRun asks for the reconcile; it is the operation of
// both the resource and the portainer_gitops_workflow_reconcile action.
func gitopsWorkflowReconcileRun(ctx context.Context, client *APIClient, plan *gitopsWorkflowReconcileResModel, diags *diag.Diagnostics) {
	workflowID := plan.WorkflowID.ValueInt64()

	endpoint := fmt.Sprintf("%s/gitops/workflows/%d/reconcile", client.Endpoint, workflowID)
	if err := doJSON(ctx, client, http.MethodPost, endpoint, nil, nil); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_gitops_workflow_reconcile", requiresBE30)
		gitopsResErrorf(diags, "failed to reconcile GitOps workflow %d: %w", workflowID, err)
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("%d-reconcile-%d", workflowID, makeTimestamp()))
}

// Read keeps the state as it is: the reconcile was a one-shot request.
func (r *gitopsWorkflowReconcileResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

// Update is never called: every argument forces a new reconcile.
func (r *gitopsWorkflowReconcileResource) Update(_ context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.State.Raw = req.Plan.Raw
}

// Delete only forgets the reconcile: there is nothing to undo.
func (r *gitopsWorkflowReconcileResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
