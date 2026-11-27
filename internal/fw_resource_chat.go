package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*chatResource)(nil)
	_ resource.ResourceWithConfigure = (*chatResource)(nil)
)

type ChatRequest struct {
	Context       string `json:"context"`
	EnvironmentID int    `json:"environmentID"`
	Message       string `json:"message"`
	Model         string `json:"model"`
}

type ChatResponse struct {
	Message string `json:"message"`
	YAML    string `json:"yaml"`
}

func newChatResource() resource.Resource { return &chatResource{} }

type chatResource struct{ client *APIClient }

type chatResourceModel struct {
	ID              types.String `tfsdk:"id"`
	Context         types.String `tfsdk:"context"`
	EnvironmentID   types.Int64  `tfsdk:"environment_id"`
	Message         types.String `tfsdk:"message"`
	Model           types.String `tfsdk:"model"`
	ResponseMessage types.String `tfsdk:"response_message"`
	ResponseYAML    types.String `tfsdk:"response_yaml"`
}

func (r *chatResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_chat"
}

func (r *chatResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		DeprecationMessage: "portainer_chat is deprecated: current Portainer releases have no OpenAI chat API (POST /api/chat answers 404), " +
			"and the resource will be removed in a future major version.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the chat request, derived from the environment identifier.",
				PlanModifiers: keep,
			},
			"context": schema.StringAttribute{
				Required:    true,
				Description: "Conversation context identifier passed to the Portainer chat assistant.",
			},
			"environment_id": schema.Int64Attribute{
				Required:    true,
				Description: "Identifier of the Portainer environment the chat request is scoped to.",
			},
			"message": schema.StringAttribute{
				Required:    true,
				Description: "User prompt sent to the Portainer chat assistant.",
			},
			"model": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("gpt-3.5-turbo"),
				Description: "AI model name used to generate the chat response (defaults to `gpt-3.5-turbo`).",
			},
			"response_message": schema.StringAttribute{
				Computed:      true,
				Description:   "Textual response returned by the Portainer chat assistant.",
				PlanModifiers: keep,
			},
			"response_yaml": schema.StringAttribute{
				Computed:      true,
				Description:   "YAML manifest extracted from the chat response, when applicable.",
				PlanModifiers: keep,
			},
		},
	}
}

func (r *chatResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *chatResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan chatResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	reqBody := ChatRequest{
		Context:       plan.Context.ValueString(),
		EnvironmentID: int(plan.EnvironmentID.ValueInt64()),
		Message:       plan.Message.ValueString(),
		Model:         plan.Model.ValueString(),
	}

	var chatResp ChatResponse
	if err := doJSON(ctx, r.client, http.MethodPost, fmt.Sprintf("%s/chat", r.client.Endpoint), reqBody, &chatResp); err != nil {
		err = explainRemovedAPI(ctx, r.client, err, "portainer_chat", "the OpenAI chat API was removed from Portainer together with its OpenAI integration")
		resp.Diagnostics.AddError("Failed to send chat", fmt.Errorf("failed to send chat: %w", err).Error())
		return
	}

	plan.ResponseMessage = types.StringValue(chatResp.Message)
	plan.ResponseYAML = types.StringValue(chatResp.YAML)
	plan.ID = types.StringValue(fmt.Sprintf("chat-%d", reqBody.EnvironmentID))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read is a no-op: a chat request is a one-shot action.
func (r *chatResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {}

// Update stores the new arguments without sending another request, as the
// SDKv2 version did; the previous responses are kept.
func (r *chatResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state chatResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	plan.ResponseMessage = state.ResponseMessage
	plan.ResponseYAML = state.ResponseYAML
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Delete only removes the chat from the state.
func (r *chatResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
