package internal

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// portainer_oidc_client_secret adds a client secret to an OIDC client. A
// client holds several, so one can be replaced without downtime. Portainer
// returns the value only when the secret is created or regenerated.
//
// Secrets cannot be edited. Changing regenerate_version regenerates the
// secret in place (POST .../regenerate): Portainer adds a secret with the
// same description and lifetime and retires the replaced one after
// old_secret_grace_period, so the application can be given the new value
// first.

var (
	_ resource.Resource                   = (*oidcClientSecretResource)(nil)
	_ resource.ResourceWithConfigure      = (*oidcClientSecretResource)(nil)
	_ resource.ResourceWithImportState    = (*oidcClientSecretResource)(nil)
	_ resource.ResourceWithModifyPlan     = (*oidcClientSecretResource)(nil)
	_ resource.ResourceWithValidateConfig = (*oidcClientSecretResource)(nil)
)

func newOIDCClientSecretResource() resource.Resource { return &oidcClientSecretResource{} }

type oidcClientSecretResource struct{ client *APIClient }

type oidcClientSecretModel struct {
	ID                   types.String `tfsdk:"id"`
	ClientID             types.Int64  `tfsdk:"client_id"`
	Description          types.String `tfsdk:"description"`
	ExpiresAt            types.Int64  `tfsdk:"expires_at"`
	RegenerateVersion    types.Int64  `tfsdk:"regenerate_version"`
	OldSecretGracePeriod types.String `tfsdk:"old_secret_grace_period"`
	SecretID             types.String `tfsdk:"secret_id"`
	Value                types.String `tfsdk:"value"`
	Hint                 types.String `tfsdk:"hint"`
	CreatedAt            types.Int64  `tfsdk:"created_at"`
	EffectiveExpiresAt   types.Int64  `tfsdk:"effective_expires_at"`
}

// oidcSecretCreateBody is oidcclients.secretCreatePayload.
type oidcSecretCreateBody struct {
	Description string `json:"description,omitempty"`
	ExpiresAt   int64  `json:"expiresAt"`
}

// oidcSecretRegenerateBody is oidcclients.secretRegeneratePayload.
type oidcSecretRegenerateBody struct {
	OldSecretExpiresAt int64 `json:"oldSecretExpiresAt"`
}

func (r *oidcClientSecretResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_oidc_client_secret"
}

func (r *oidcClientSecretResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	keepInt := []planmodifier.Int64{int64planmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Description: "Adds a client secret to a client of Portainer's OIDC provider, and regenerates it on demand. Business Edition 3.1 or later.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the resource, `<client id>:<secret id>`; also the import ID.",
				PlanModifiers: keep,
			},
			"client_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Numeric Portainer identifier of the OIDC client (the `id` of `portainer_oidc_client`, not its OAuth `client_id`). Changing it forces a new secret.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"description": schema.StringAttribute{
				Optional:      true,
				Description:   "Description of the secret, to tell secrets apart. Changing it forces a new secret.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"expires_at": schema.Int64Attribute{
				Optional:      true,
				Description:   "Unix time the secret expires at; unset or 0 for never. Changing it forces a new secret. A regenerated secret keeps the lifetime of the one it replaces, so after a regenerate `effective_expires_at` holds its actual expiry.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"regenerate_version": schema.Int64Attribute{
				Optional:    true,
				Description: "Changing it from one number to another regenerates the secret in place: Portainer issues a new value (same description and lifetime) and retires the old one after `old_secret_grace_period`. Setting it for the first time (for example after an import) does not regenerate.",
			},
			"old_secret_grace_period": schema.StringAttribute{
				Optional:    true,
				Description: "How long the replaced secret keeps working after a regenerate, as a Go duration such as `24h`, so the application can be given the new value first. Unset or `0s` removes it at once. It never extends the replaced secret's own expiry.",
				Validators:  []validator.String{durationValidator{}},
			},
			"secret_id": schema.StringAttribute{
				Computed:      true,
				Description:   "Portainer identifier of the secret. It changes when the secret is regenerated.",
				PlanModifiers: keep,
			},
			"value": schema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				Description:   "The client secret. Portainer returns it only when the secret is created or regenerated, so it is only ever available from the state written by that apply; it is null after an import.",
				PlanModifiers: keep,
			},
			"hint": schema.StringAttribute{
				Computed:      true,
				Description:   "Last characters of the secret, which Portainer shows to tell secrets apart.",
				PlanModifiers: keep,
			},
			"created_at": schema.Int64Attribute{
				Computed:      true,
				Description:   "Unix time the secret was created (or last regenerated).",
				PlanModifiers: keepInt,
			},
			"effective_expires_at": schema.Int64Attribute{
				Computed:      true,
				Description:   "Unix time the current secret expires at as Portainer reports it, 0 for never.",
				PlanModifiers: keepInt,
			},
		},
	}
}

func (r *oidcClientSecretResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *oidcClientSecretResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var expires types.Int64
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("expires_at"), &expires)...)
	if !expires.IsNull() && !expires.IsUnknown() && expires.ValueInt64() < 0 {
		resp.Diagnostics.AddAttributeError(path.Root("expires_at"), "Invalid expires_at", "expires_at must be a Unix time, or 0 for never.")
	}
}

// oidcSecretRegenerating reports whether the plan regenerates the secret:
// regenerate_version changed from one number to another.
func oidcSecretRegenerating(plan, state oidcClientSecretModel) bool {
	return !state.RegenerateVersion.IsNull() && !plan.RegenerateVersion.IsNull() && !plan.RegenerateVersion.IsUnknown() &&
		plan.RegenerateVersion.ValueInt64() != state.RegenerateVersion.ValueInt64()
}

// ModifyPlan marks what a regenerate changes as unknown.
func (r *oidcClientSecretResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var plan, state oidcClientSecretModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || !oidcSecretRegenerating(plan, state) {
		return
	}
	plan.ID = types.StringUnknown()
	plan.SecretID = types.StringUnknown()
	plan.Value = types.StringUnknown()
	plan.Hint = types.StringUnknown()
	plan.CreatedAt = types.Int64Unknown()
	plan.EffectiveExpiresAt = types.Int64Unknown()
	resp.Diagnostics.Append(resp.Plan.Set(ctx, &plan)...)
}

// applySecret stores a secret Portainer has just issued.
func (m *oidcClientSecretModel) applySecret(created oidcSecretCreateResponse) {
	s := created.Secret
	m.ID = types.StringValue(fmt.Sprintf("%d:%s", m.ClientID.ValueInt64(), s.ID))
	m.SecretID = types.StringValue(s.ID)
	m.Value = types.StringValue(created.Value)
	m.Hint = types.StringValue(s.Hint)
	m.CreatedAt = types.Int64Value(s.CreatedAt)
	m.EffectiveExpiresAt = types.Int64Value(s.ExpiresAt)
}

func (r *oidcClientSecretResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan oidcClientSecretModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	clientID := plan.ClientID.ValueInt64()
	body := oidcSecretCreateBody{Description: plan.Description.ValueString(), ExpiresAt: plan.ExpiresAt.ValueInt64()}
	var created oidcSecretCreateResponse
	if err := doJSON(ctx, r.client, http.MethodPost, fmt.Sprintf("%s/oidc_clients/%d/secrets", r.client.Endpoint, clientID), body, &created); err != nil {
		err = explainUnsupported(ctx, r.client, err, "portainer_oidc_client_secret", requiresBE31OIDC)
		resp.Diagnostics.AddError("Failed to create OIDC client secret", fmt.Sprintf("failed to add a secret to OIDC client %d: %s", clientID, err))
		return
	}
	if created.Secret.ID == "" {
		resp.Diagnostics.AddError("Failed to create OIDC client secret", fmt.Sprintf("Portainer returned no secret identifier for OIDC client %d", clientID))
		return
	}
	plan.applySecret(created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *oidcClientSecretResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state oidcClientSecretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	clientID := state.ClientID.ValueInt64()
	c, err := oidcGetClient(ctx, r.client, clientID)
	if err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read OIDC client secret", fmt.Sprintf("failed to read OIDC client %d: %s", clientID, err))
		return
	}
	var found *oidcSecretAPI
	for i := range c.Secrets {
		if c.Secrets[i].ID == state.SecretID.ValueString() {
			found = &c.Secrets[i]
			break
		}
	}
	if found == nil {
		// Deleted, or expired and removed by Portainer.
		resp.State.RemoveResource(ctx)
		return
	}
	state.Description = egOptString(found.Description, state.Description)
	state.Hint = types.StringValue(found.Hint)
	state.CreatedAt = types.Int64Value(found.CreatedAt)
	state.EffectiveExpiresAt = types.Int64Value(found.ExpiresAt)
	// expires_at keeps the configured value (a regenerated secret gets a new
	// expiry from Portainer); an import, which has none, takes Portainer's.
	if state.ExpiresAt.IsNull() && found.ExpiresAt != 0 {
		state.ExpiresAt = types.Int64Value(found.ExpiresAt)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *oidcClientSecretResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state oidcClientSecretModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !oidcSecretRegenerating(plan, state) {
		// Only regenerate_version or old_secret_grace_period changed.
		resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
		return
	}

	var oldExpiresAt int64
	if grace := plan.OldSecretGracePeriod.ValueString(); grace != "" {
		d, err := time.ParseDuration(grace)
		if err != nil {
			resp.Diagnostics.AddAttributeError(path.Root("old_secret_grace_period"), "Invalid duration", err.Error())
			return
		}
		if d > 0 {
			oldExpiresAt = time.Now().Add(d).Unix()
		}
	}
	clientID := state.ClientID.ValueInt64()
	secretID := state.SecretID.ValueString()
	var created oidcSecretCreateResponse
	endpoint := fmt.Sprintf("%s/oidc_clients/%d/secrets/%s/regenerate", r.client.Endpoint, clientID, secretID)
	if err := doJSON(ctx, r.client, http.MethodPost, endpoint, oidcSecretRegenerateBody{OldSecretExpiresAt: oldExpiresAt}, &created); err != nil {
		resp.Diagnostics.AddError("Failed to regenerate OIDC client secret", fmt.Sprintf("failed to regenerate secret %s of OIDC client %d: %s", secretID, clientID, err))
		return
	}
	if created.Secret.ID == "" {
		resp.Diagnostics.AddError("Failed to regenerate OIDC client secret", fmt.Sprintf("Portainer returned no secret identifier for OIDC client %d", clientID))
		return
	}
	plan.applySecret(created)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *oidcClientSecretResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state oidcClientSecretModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	clientID := state.ClientID.ValueInt64()
	secretID := state.SecretID.ValueString()
	endpoint := fmt.Sprintf("%s/oidc_clients/%d/secrets/%s", r.client.Endpoint, clientID, secretID)
	if err := doJSON(ctx, r.client, http.MethodDelete, endpoint, nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete OIDC client secret", fmt.Sprintf("failed to delete secret %s of OIDC client %d: %s", secretID, clientID, err))
	}
}

// ImportState takes "<client id>:<secret id>". The value cannot be
// imported: Portainer never returns it again.
func (r *oidcClientSecretResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	clientStr, secretID, ok := strings.Cut(req.ID, ":")
	clientID, err := oidcParseID(clientStr)
	if !ok || err != nil || secretID == "" {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("expected \"<client id>:<secret id>\" (the numeric Portainer ID of the OIDC client and the secret's ID), got %q", req.ID))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), req.ID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("client_id"), clientID)...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("secret_id"), secretID)...)
}
