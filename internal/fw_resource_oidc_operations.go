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

// The one-shot operations of Portainer's OIDC provider, as action resources
// (for OpenTofu and Terraform < 1.14) and actions:
//   - portainer_oidc_keys_rotate: sign tokens with a new key
//     (POST /oidc_keys/rotate);
//   - portainer_oidc_client_sessions_end: end the sessions of a client, or a
//     user's sessions in it (DELETE /oidc_clients/{id}/sessions);
//   - portainer_oidc_user_sessions_end: sign a user out of every client
//     (DELETE /oidc_sessions).
// Each runs once when created; `triggers` runs it again. Read keeps the
// state, Delete only forgets it.

// oidcOperationResource is a one-shot OIDC operation resource. run performs
// the operation and returns the ID to record.
type oidcOperationResource[M any] struct {
	typeSuffix  string
	description string
	attributes  map[string]schema.Attribute
	run         func(ctx context.Context, client *APIClient, m *M, diags *diag.Diagnostics)
	client      *APIClient
}

func (r *oidcOperationResource[M]) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + r.typeSuffix
}

func (r *oidcOperationResource[M]) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := map[string]schema.Attribute{
		"id": schema.StringAttribute{
			Computed:      true,
			Description:   "Identifier of the resource.",
			PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
		},
		"triggers": schema.MapAttribute{
			Optional:      true,
			ElementType:   types.StringType,
			PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()},
			Description:   "Arbitrary values that run the operation again when they change. It is a one-shot operation, so without a trigger the resource runs once and then stays put.",
		},
	}
	for k, v := range r.attributes {
		attrs[k] = v
	}
	resp.Schema = schema.Schema{Description: r.description, Attributes: attrs}
}

func (r *oidcOperationResource[M]) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *oidcOperationResource[M]) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan M
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.run(ctx, r.client, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read keeps the state as it is: the operation was a one-shot request.
func (r *oidcOperationResource[M]) Read(context.Context, resource.ReadRequest, *resource.ReadResponse) {
}

// Update is never called: every argument forces the operation again.
func (r *oidcOperationResource[M]) Update(_ context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.State.Raw = req.Plan.Raw
}

// Delete only forgets the operation: there is nothing to undo.
func (r *oidcOperationResource[M]) Delete(context.Context, resource.DeleteRequest, *resource.DeleteResponse) {
}

func oidcRequiredInt(desc string) schema.Int64Attribute {
	return schema.Int64Attribute{Required: true, Description: desc, PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()}}
}

// portainer_oidc_keys_rotate

type oidcKeysRotateModel struct {
	ID       types.String `tfsdk:"id"`
	Triggers types.Map    `tfsdk:"triggers"`
}

func newOIDCKeysRotateResource() resource.Resource {
	return &oidcOperationResource[oidcKeysRotateModel]{
		typeSuffix:  "_oidc_keys_rotate",
		description: "Rotates the signing key of Portainer's OIDC provider. Tokens signed with the replaced key verify until they expire; rotating again before then refuses them. Business Edition 3.1 or later.",
		run:         oidcKeysRotateRun,
	}
}

// oidcKeysRotateRun is the operation of both the resource and the action.
func oidcKeysRotateRun(ctx context.Context, client *APIClient, m *oidcKeysRotateModel, diags *diag.Diagnostics) {
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/oidc_keys/rotate", nil, nil); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_oidc_keys_rotate", requiresBE31OIDC)
		diags.AddError("Failed to rotate the OIDC signing key", fmt.Sprintf("failed to rotate the OIDC provider's signing key: %s", err))
		return
	}
	m.ID = types.StringValue(fmt.Sprintf("oidc-keys-rotate-%d", makeTimestamp()))
}

// portainer_oidc_client_sessions_end

type oidcClientSessionsEndModel struct {
	ID       types.String `tfsdk:"id"`
	ClientID types.Int64  `tfsdk:"client_id"`
	UserID   types.Int64  `tfsdk:"user_id"`
	Triggers types.Map    `tfsdk:"triggers"`
}

func newOIDCClientSessionsEndResource() resource.Resource {
	return &oidcOperationResource[oidcClientSessionsEndModel]{
		typeSuffix:  "_oidc_client_sessions_end",
		description: "Ends every session of a client of Portainer's OIDC provider, or only a user's. Their tokens are revoked at once. Business Edition 3.1 or later.",
		attributes: map[string]schema.Attribute{
			"client_id": oidcRequiredInt("Numeric Portainer identifier of the OIDC client (the `id` of `portainer_oidc_client`, not its OAuth `client_id`)."),
			"user_id": schema.Int64Attribute{
				Optional:      true,
				Description:   "Only end this user's sessions. Unset ends every session of the client.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
		},
		run: oidcClientSessionsEndRun,
	}
}

func oidcClientSessionsEndRun(ctx context.Context, client *APIClient, m *oidcClientSessionsEndModel, diags *diag.Diagnostics) {
	clientID := m.ClientID.ValueInt64()
	endpoint := fmt.Sprintf("%s/oidc_clients/%d/sessions", client.Endpoint, clientID)
	if !m.UserID.IsNull() {
		endpoint += fmt.Sprintf("?userId=%d", m.UserID.ValueInt64())
	}
	if err := doJSON(ctx, client, http.MethodDelete, endpoint, nil, nil); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_oidc_client_sessions_end", requiresBE31OIDC)
		diags.AddError("Failed to end OIDC client sessions", fmt.Sprintf("failed to end the sessions of OIDC client %d: %s", clientID, err))
		return
	}
	m.ID = types.StringValue(fmt.Sprintf("%d-sessions-end-%d", clientID, makeTimestamp()))
}

// portainer_oidc_user_sessions_end

type oidcUserSessionsEndModel struct {
	ID       types.String `tfsdk:"id"`
	UserID   types.Int64  `tfsdk:"user_id"`
	Triggers types.Map    `tfsdk:"triggers"`
}

func newOIDCUserSessionsEndResource() resource.Resource {
	return &oidcOperationResource[oidcUserSessionsEndModel]{
		typeSuffix:  "_oidc_user_sessions_end",
		description: "Signs a user out of every client and add-on that signed them in through Portainer's OIDC provider. Their tokens are revoked at once. Business Edition 3.1 or later.",
		attributes: map[string]schema.Attribute{
			"user_id": oidcRequiredInt("Identifier of the Portainer user to sign out."),
		},
		run: oidcUserSessionsEndRun,
	}
}

func oidcUserSessionsEndRun(ctx context.Context, client *APIClient, m *oidcUserSessionsEndModel, diags *diag.Diagnostics) {
	userID := m.UserID.ValueInt64()
	endpoint := fmt.Sprintf("%s/oidc_sessions?userId=%d", client.Endpoint, userID)
	if err := doJSON(ctx, client, http.MethodDelete, endpoint, nil, nil); err != nil {
		err = explainUnsupported(ctx, client, err, "portainer_oidc_user_sessions_end", requiresBE31OIDC)
		diags.AddError("Failed to end OIDC user sessions", fmt.Sprintf("failed to end the OIDC sessions of user %d: %s", userID, err))
		return
	}
	m.ID = types.StringValue(fmt.Sprintf("%d-oidc-sessions-end-%d", userID, makeTimestamp()))
}

var (
	newOIDCKeysRotateAction = newResourceAction(&resourceActionDef{
		resource:    newOIDCKeysRotateResource,
		description: "Rotates the signing key of Portainer's OIDC provider. Business Edition 3.1 or later.",
		invoke:      actionDiagsOnly(oidcKeysRotateRun),
	})

	newOIDCClientSessionsEndAction = newResourceAction(&resourceActionDef{
		resource:    newOIDCClientSessionsEndResource,
		description: "Ends every session of a client of Portainer's OIDC provider, or only a user's. Business Edition 3.1 or later.",
		invoke:      actionDiagsOnly(oidcClientSessionsEndRun),
	})

	newOIDCUserSessionsEndAction = newResourceAction(&resourceActionDef{
		resource:    newOIDCUserSessionsEndResource,
		description: "Signs a user out of every client of Portainer's OIDC provider. Business Edition 3.1 or later.",
		invoke:      actionDiagsOnly(oidcUserSessionsEndRun),
	})
)
