package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*edgeStackWebhookResource)(nil)
	_ resource.ResourceWithConfigure = (*edgeStackWebhookResource)(nil)
)

func newEdgeStackWebhookResource() resource.Resource { return &edgeStackWebhookResource{} }

// edgeStackWebhookResource triggers an edge stack webhook once, on create.
type edgeStackWebhookResource struct{ client *APIClient }

type edgeStackWebhookModel struct {
	ID        types.String `tfsdk:"id"`
	WebhookID types.String `tfsdk:"webhook_id"`
}

func (r *edgeStackWebhookResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_stack_webhook"
}

func (r *edgeStackWebhookResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "The webhook UUID that was triggered.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"webhook_id": schema.StringAttribute{
				Required:      true,
				Description:   "UUID of the webhook to trigger",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
		},
	}
}

func (r *edgeStackWebhookResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *edgeStackWebhookResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan edgeStackWebhookModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	edgeStackWebhookRun(ctx, r.client, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// edgeStackWebhookRun triggers the edge stack webhook; it is the operation of
// both the resource and the portainer_edge_stack_webhook action.
func edgeStackWebhookRun(ctx context.Context, client *APIClient, plan *edgeStackWebhookModel, diags *diag.Diagnostics) {
	webhookID := plan.WebhookID.ValueString()
	url := fmt.Sprintf("%s/edge_stacks/webhooks/%s", client.Endpoint, webhookID)
	if err := doJSON(ctx, client, http.MethodPost, url, nil, nil); err != nil {
		diags.AddError("Failed to trigger webhook", fmt.Sprintf("failed to trigger webhook: %s", err))
		return
	}
	plan.ID = types.StringValue(webhookID)
}

// Read keeps the state: a webhook trigger leaves nothing to read back.
func (r *edgeStackWebhookResource) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

// Update is never called: every attribute requires replacement.
func (r *edgeStackWebhookResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Edge stack webhooks cannot be updated", "Every attribute of portainer_edge_stack_webhook forces a new resource.")
}

// Delete only drops the resource from state.
func (r *edgeStackWebhookResource) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}
