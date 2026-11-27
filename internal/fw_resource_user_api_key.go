package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*userAPIKeyResource)(nil)
	_ resource.ResourceWithConfigure = (*userAPIKeyResource)(nil)
)

func newUserAPIKeyResource() resource.Resource { return &userAPIKeyResource{} }

type userAPIKeyResource struct{ client *APIClient }

type userAPIKeyResourceModel struct {
	ID          types.String `tfsdk:"id"`
	UserID      types.Int64  `tfsdk:"user_id"`
	Description types.String `tfsdk:"description"`
	Password    types.String `tfsdk:"password"`
	RawAPIKey   types.String `tfsdk:"raw_api_key"`
	Prefix      types.String `tfsdk:"prefix"`
	DateCreated types.Int64  `tfsdk:"date_created"`
	LastUsed    types.Int64  `tfsdk:"last_used"`
}

func (r *userAPIKeyResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_api_key"
}

func (r *userAPIKeyResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the resource.", Computed: true, PlanModifiers: keep},
			"user_id": schema.Int64Attribute{
				Required:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				Description:   "Identifier of the user the API key belongs to.",
			},
			"description": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Description of the key, shown in Portainer's user settings. Portainer requires it to be unique per user.",
			},
			"password": schema.StringAttribute{
				Required:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Password of the user the key is generated for. Portainer requires it to authorize the request; it is not stored on the key itself.",
			},
			"raw_api_key": schema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				PlanModifiers: keep,
				Description:   "The generated API key. Portainer returns it exactly once, at creation, so it is only ever available from the state written by that apply.",
			},
			"prefix": schema.StringAttribute{
				Computed:      true,
				PlanModifiers: keep,
				Description:   "Non-secret prefix of the key, which is what Portainer displays to identify it.",
			},
			"date_created": schema.Int64Attribute{
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				Description:   "Unix timestamp at which the key was created.",
			},
			"last_used": schema.Int64Attribute{
				Computed:    true,
				Description: "Unix timestamp of the key's last use, zero when it has never been used.",
			},
		},
	}
}

func (r *userAPIKeyResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *userAPIKeyResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userAPIKeyResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	client := r.client
	userID := plan.UserID.ValueInt64()

	payload := map[string]interface{}{
		"description": plan.Description.ValueString(),
		"password":    plan.Password.ValueString(),
	}

	var response struct {
		RawAPIKey string          `json:"rawAPIKey"`
		APIKey    portainerAPIKey `json:"apiKey"`
	}
	// Portainer accepts this call only from a session, never from an API key,
	// and only for the calling user's own account. Saying so here turns an
	// opaque 401 "Auth not supported" into something actionable.
	if client.APIKey != "" {
		resp.Diagnostics.AddError("Failed to create API key", fmt.Sprintf(
			"cannot create an API key for user %d: Portainer only accepts this call from a session, so the provider has to be configured with api_user and api_password rather than api_key - and the key can only be created for that same user", userID))
		return
	}

	url := fmt.Sprintf("%s/users/%d/tokens", client.Endpoint, userID)
	if err := doJSON(ctx, client, http.MethodPost, url, payload, &response); err != nil {
		resp.Diagnostics.AddError("Failed to create API key", fmt.Sprintf("failed to create an API key for user %d: %s", userID, err))
		return
	}
	if response.APIKey.ID == 0 {
		resp.Diagnostics.AddError("Failed to create API key", fmt.Sprintf("failed to create an API key for user %d: Portainer returned no key ID", userID))
		return
	}

	// rawAPIKey is returned only here and can never be read back, so it is
	// stored now or lost.
	plan.ID = types.StringValue(strconv.Itoa(response.APIKey.ID))
	plan.RawAPIKey = types.StringValue(response.RawAPIKey)
	plan.Prefix = types.StringValue(response.APIKey.Prefix)
	plan.DateCreated = types.Int64Value(int64(response.APIKey.DateCreated))
	plan.LastUsed = types.Int64Value(int64(response.APIKey.LastUsed))
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userAPIKeyResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userAPIKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	userID := state.UserID.ValueInt64()

	// Portainer has no endpoint for a single key, so the user's keys are listed
	// and matched by ID.
	var keys []portainerAPIKey
	url := fmt.Sprintf("%s/users/%d/tokens", r.client.Endpoint, userID)
	if err := doJSON(ctx, r.client, http.MethodGet, url, nil, &keys); err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read API key", fmt.Sprintf("failed to list the API keys of user %d: %s", userID, err))
		return
	}

	for _, key := range keys {
		if strconv.Itoa(key.ID) != state.ID.ValueString() {
			continue
		}
		state.Description = types.StringValue(key.Description)
		state.Prefix = types.StringValue(key.Prefix)
		state.DateCreated = types.Int64Value(int64(key.DateCreated))
		state.LastUsed = types.Int64Value(int64(key.LastUsed))
		resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
		return
	}

	// Revoked outside Terraform.
	resp.State.RemoveResource(ctx)
}

// Update is never called: every argument forces a new key.
func (r *userAPIKeyResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("API keys cannot be updated", "Every argument of portainer_user_api_key forces a new key.")
}

func (r *userAPIKeyResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userAPIKeyResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	userID := state.UserID.ValueInt64()
	url := fmt.Sprintf("%s/users/%d/tokens/%s", r.client.Endpoint, userID, state.ID.ValueString())
	if err := doJSON(ctx, r.client, http.MethodDelete, url, nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to revoke API key", fmt.Sprintf("failed to revoke API key %s of user %d: %s", state.ID.ValueString(), userID, err))
	}
}
