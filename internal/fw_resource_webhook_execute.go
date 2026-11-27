package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*webhookExecuteResource)(nil)
	_ resource.ResourceWithConfigure = (*webhookExecuteResource)(nil)
)

func newWebhookExecuteResource() resource.Resource { return &webhookExecuteResource{} }

type webhookExecuteResource struct{ client *APIClient }

type webhookExecuteModel struct {
	ID          types.String `tfsdk:"id"`
	Token       types.String `tfsdk:"token"`
	StackID     types.String `tfsdk:"stack_id"`
	EdgeStackID types.String `tfsdk:"edge_stack_id"`
}

func (r *webhookExecuteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_webhook_execute"
}

func (r *webhookExecuteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{rcRequiresReplaceString()}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "The token, stack ID or edge stack ID the webhook was executed for.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"token": schema.StringAttribute{
				Optional:      true,
				Sensitive:     true,
				Description:   "Webhook token for service restart webhook",
				PlanModifiers: replace,
				Validators:    []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("stack_id"), path.MatchRoot("edge_stack_id"))},
			},
			"stack_id": schema.StringAttribute{
				Optional:      true,
				Description:   "Stack ID for triggering stack GitOps update",
				PlanModifiers: replace,
				Validators:    []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("token"), path.MatchRoot("edge_stack_id"))},
			},
			"edge_stack_id": schema.StringAttribute{
				Optional:      true,
				Description:   "Edge Stack ID for triggering edge stack GitOps update",
				PlanModifiers: replace,
				Validators:    []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("token"), path.MatchRoot("stack_id"))},
			},
		},
	}
}

func (r *webhookExecuteResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *webhookExecuteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan webhookExecuteModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	webhookExecuteRun(ctx, r.client, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// webhookExecuteRun calls the webhook; it is the operation of both the
// resource and the portainer_webhook_execute action.
func webhookExecuteRun(ctx context.Context, client *APIClient, plan *webhookExecuteModel, diags *diag.Diagnostics) {
	var url string
	switch {
	case plan.Token.ValueString() != "":
		url = fmt.Sprintf("%s/webhooks/%s", client.Endpoint, plan.Token.ValueString())
		plan.ID = plan.Token
	case plan.StackID.ValueString() != "":
		url = fmt.Sprintf("%s/stacks/webhooks/%s", client.Endpoint, plan.StackID.ValueString())
		plan.ID = plan.StackID
	case plan.EdgeStackID.ValueString() != "":
		url = fmt.Sprintf("%s/edge_stacks/webhooks/%s", client.Endpoint, plan.EdgeStackID.ValueString())
		plan.ID = plan.EdgeStackID
	default:
		diags.AddError("Failed to execute webhook", "one of 'token', 'stack_id' or 'edge_stack_id' must be set")
		return
	}

	// Webhooks are public endpoints: the request carries no credentials.
	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, url, nil)
	if err != nil {
		diags.AddError("Failed to execute webhook", err.Error())
		return
	}
	httpResp, err := client.HTTPClient.Do(httpReq)
	if err != nil {
		diags.AddError("Failed to execute webhook", err.Error())
		return
	}
	defer httpResp.Body.Close()
	if httpResp.StatusCode >= 400 {
		diags.AddError("Failed to execute webhook", fmt.Sprintf("failed to execute webhook: HTTP %d", httpResp.StatusCode))
		return
	}
}

// Read has nothing to refresh: the execution is a one-off.
func (r *webhookExecuteResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

// Update is never called: every attribute requires replacement.
func (r *webhookExecuteResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Webhook executions cannot be updated", "Every attribute of portainer_webhook_execute forces a new resource.")
}

// Delete only forgets the execution.
func (r *webhookExecuteResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
