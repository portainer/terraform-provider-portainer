package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// portainer_oidc_client registers an application or add-on that signs users
// in through Portainer's OIDC provider (Business Edition 3.1+). Portainer
// generates the OAuth client ID and a first client secret; the secret is
// returned only by the create request, so it lives in the state written by
// that apply, like portainer_user_api_key's raw_api_key.

var (
	_ resource.Resource                = (*oidcClientResource)(nil)
	_ resource.ResourceWithConfigure   = (*oidcClientResource)(nil)
	_ resource.ResourceWithImportState = (*oidcClientResource)(nil)
	_ resource.ResourceWithIdentity    = (*oidcClientResource)(nil)
)

func newOIDCClientResource() resource.Resource { return &oidcClientResource{} }

type oidcClientResource struct {
	fwIDIdentity
	client *APIClient
}

type oidcClientModel struct {
	ID                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	Description            types.String `tfsdk:"description"`
	RedirectURIs           types.List   `tfsdk:"redirect_uris"`
	PostLogoutRedirectURIs types.List   `tfsdk:"post_logout_redirect_uris"`
	HomepageURL            types.String `tfsdk:"homepage_url"`
	Permission             types.String `tfsdk:"permission"`
	AllUsers               types.Bool   `tfsdk:"all_users"`
	GroupsClaim            types.Bool   `tfsdk:"groups_claim"`
	ShowInLauncher         types.Bool   `tfsdk:"show_in_launcher"`
	Enabled                types.Bool   `tfsdk:"enabled"`
	UserAccessPolicies     types.Map    `tfsdk:"user_access_policies"`
	TeamAccessPolicies     types.Map    `tfsdk:"team_access_policies"`
	SecretExpiresAt        types.Int64  `tfsdk:"secret_expires_at"`
	ClientID               types.String `tfsdk:"client_id"`
	ClientSecret           types.String `tfsdk:"client_secret"`
	ClientSecretID         types.String `tfsdk:"client_secret_id"`
}

// oidcClientCreateBody is oidcclients.clientCreatePayload.
type oidcClientCreateBody struct {
	Name                   string                      `json:"name"`
	Description            string                      `json:"description,omitempty"`
	RedirectURIs           []string                    `json:"redirectUris"`
	PostLogoutRedirectURIs []string                    `json:"postLogoutRedirectUris,omitempty"`
	HomepageURL            string                      `json:"homepageUrl,omitempty"`
	Permission             string                      `json:"permission,omitempty"`
	AllUsers               bool                        `json:"allUsers"`
	GroupsClaim            bool                        `json:"groupsClaim"`
	ShowInLauncher         bool                        `json:"showInLauncher"`
	SecretExpiresAt        int64                       `json:"secretExpiresAt,omitempty"`
	UserAccessPolicies     map[string]oidcAccessPolicy `json:"userAccessPolicies,omitempty"`
	TeamAccessPolicies     map[string]oidcAccessPolicy `json:"teamAccessPolicies,omitempty"`
}

// oidcClientUpdateBody is oidcclients.clientUpdatePayload. Portainer leaves
// absent fields unchanged, so every field is sent: an attribute removed from
// the configuration is cleared.
type oidcClientUpdateBody struct {
	Name                   string                      `json:"name"`
	Description            string                      `json:"description"`
	RedirectURIs           []string                    `json:"redirectUris"`
	PostLogoutRedirectURIs []string                    `json:"postLogoutRedirectUris"`
	HomepageURL            string                      `json:"homepageUrl"`
	Permission             string                      `json:"permission"`
	AllUsers               bool                        `json:"allUsers"`
	GroupsClaim            bool                        `json:"groupsClaim"`
	ShowInLauncher         bool                        `json:"showInLauncher"`
	Enabled                bool                        `json:"enabled"`
	UserAccessPolicies     map[string]oidcAccessPolicy `json:"userAccessPolicies"`
	TeamAccessPolicies     map[string]oidcAccessPolicy `json:"teamAccessPolicies"`
}

type oidcClientCreateResponse struct {
	Client oidcClientAPI `json:"client"`
	Secret string        `json:"secret"`
}

func (r *oidcClientResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_oidc_client"
}

func (r *oidcClientResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keep := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Description: "Registers an application or add-on that signs users in through Portainer's built-in OIDC provider. Business Edition 3.1 or later.",
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Numeric Portainer identifier of the OIDC client (not the OAuth `client_id`).",
				PlanModifiers: keep,
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the client, shown in Portainer and in its launcher.",
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"description": schema.StringAttribute{
				Optional:    true,
				Description: "Description of the client, shown in Portainer's launcher.",
			},
			"redirect_uris": schema.ListAttribute{
				Required:    true,
				ElementType: types.StringType,
				Description: "Redirect URIs the client may send users back to after they sign in, for example `https://grafana.example.com/login/generic_oauth`.",
				Validators:  []validator.List{listvalidator.SizeAtLeast(1)},
			},
			"post_logout_redirect_uris": schema.ListAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "URIs the client may send users to after they sign out.",
			},
			"homepage_url": schema.StringAttribute{
				Optional:    true,
				Description: "URL Portainer's launcher opens for the client.",
			},
			"permission": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("none"),
				Description: "What the client's tokens may do in Portainer's API: `none` (sign-in only), `read-only` or `act-as-user`. Defaults to `none`.",
				Validators:  []validator.String{stringvalidator.OneOf("none", "read-only", "act-as-user")},
			},
			"all_users": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether every Portainer user may sign in to the client. Otherwise only the users and teams in `user_access_policies` and `team_access_policies` may. Defaults to `false`.",
			},
			"groups_claim": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether the client's tokens carry the user's team names in a `groups` claim. Defaults to `false`.",
			},
			"show_in_launcher": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether the client is listed in Portainer's launcher for the users who may sign in to it. Defaults to `false`.",
			},
			"enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether the client is switched on. While it is off no one can sign in to it and its tokens are refused. Portainer creates clients switched on; `false` is applied by an update right after the create. Defaults to `true`.",
			},
			"user_access_policies": schema.MapAttribute{
				Optional:    true,
				ElementType: types.Int64Type,
				Description: "Users who may sign in to the client: map of user IDs to role IDs (e.g. userID -> roleID). Portainer stores a role with each policy.",
			},
			"team_access_policies": schema.MapAttribute{
				Optional:    true,
				ElementType: types.Int64Type,
				Description: "Teams whose members may sign in to the client: map of team IDs to role IDs (e.g. teamID -> roleID). Portainer stores a role with each policy.",
			},
			"secret_expires_at": schema.Int64Attribute{
				Optional:      true,
				Description:   "Unix time the first client secret expires at; unset or 0 for never. It only applies when the client is created, so changing it replaces the client. Further secrets are managed with `portainer_oidc_client_secret`.",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"client_id": schema.StringAttribute{
				Computed:      true,
				Description:   "OAuth client ID Portainer generated, which the application is configured with.",
				PlanModifiers: keep,
			},
			"client_secret": schema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				Description:   "First client secret. Portainer returns it exactly once, at creation, so it is only ever available from the state written by that apply; it is null after an import.",
				PlanModifiers: keep,
			},
			"client_secret_id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the first client secret, to import it into `portainer_oidc_client_secret` (for example to regenerate it). Null after an import.",
				PlanModifiers: keep,
			},
		},
	}
}

func (r *oidcClientResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *oidcClientResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan oidcClientModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	body := oidcClientCreateBody{
		Name:                   plan.Name.ValueString(),
		Description:            plan.Description.ValueString(),
		RedirectURIs:           oidcStrings(ctx, plan.RedirectURIs, &resp.Diagnostics),
		PostLogoutRedirectURIs: oidcStrings(ctx, plan.PostLogoutRedirectURIs, &resp.Diagnostics),
		HomepageURL:            plan.HomepageURL.ValueString(),
		Permission:             plan.Permission.ValueString(),
		AllUsers:               plan.AllUsers.ValueBool(),
		GroupsClaim:            plan.GroupsClaim.ValueBool(),
		ShowInLauncher:         plan.ShowInLauncher.ValueBool(),
		SecretExpiresAt:        plan.SecretExpiresAt.ValueInt64(),
		UserAccessPolicies:     oidcPoliciesFromMap(ctx, plan.UserAccessPolicies, &resp.Diagnostics),
		TeamAccessPolicies:     oidcPoliciesFromMap(ctx, plan.TeamAccessPolicies, &resp.Diagnostics),
	}
	if resp.Diagnostics.HasError() {
		return
	}

	var created oidcClientCreateResponse
	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/oidc_clients", body, &created); err != nil {
		err = explainUnsupported(ctx, r.client, err, "portainer_oidc_client", requiresBE31OIDC)
		resp.Diagnostics.AddError("Failed to create OIDC client", fmt.Sprintf("failed to create OIDC client: %s", err))
		return
	}
	c := created.Client
	plan.ID = types.StringValue(fwID(c.ID))
	plan.ClientID = types.StringValue(c.ClientID)
	plan.ClientSecret = types.StringValue(created.Secret)
	plan.ClientSecretID = types.StringNull()
	if len(c.Secrets) > 0 {
		plan.ClientSecretID = types.StringValue(c.Secrets[0].ID)
	}
	// Save what exists before the next request, so a failing update leaves
	// the client (and its only copy of the secret) in state.
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// The create payload has no enabled field: a client created switched off
	// is switched off by an update.
	if !plan.Enabled.ValueBool() {
		updated, ok := r.update(ctx, &plan, &resp.Diagnostics)
		if !ok {
			return
		}
		c = updated
	}
	r.fromAPI(c, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// update sends PUT /oidc_clients/{id} with the whole plan and returns the
// client Portainer answers with.
func (r *oidcClientResource) update(ctx context.Context, m *oidcClientModel, diags *diag.Diagnostics) (oidcClientAPI, bool) {
	id, err := oidcParseID(m.ID.ValueString())
	if err != nil {
		diags.AddError("Failed to update OIDC client", err.Error())
		return oidcClientAPI{}, false
	}
	body := oidcClientUpdateBody{
		Name:                   m.Name.ValueString(),
		Description:            m.Description.ValueString(),
		RedirectURIs:           oidcStrings(ctx, m.RedirectURIs, diags),
		PostLogoutRedirectURIs: oidcStrings(ctx, m.PostLogoutRedirectURIs, diags),
		HomepageURL:            m.HomepageURL.ValueString(),
		Permission:             m.Permission.ValueString(),
		AllUsers:               m.AllUsers.ValueBool(),
		GroupsClaim:            m.GroupsClaim.ValueBool(),
		ShowInLauncher:         m.ShowInLauncher.ValueBool(),
		Enabled:                m.Enabled.ValueBool(),
		UserAccessPolicies:     oidcPoliciesFromMap(ctx, m.UserAccessPolicies, diags),
		TeamAccessPolicies:     oidcPoliciesFromMap(ctx, m.TeamAccessPolicies, diags),
	}
	if diags.HasError() {
		return oidcClientAPI{}, false
	}
	var updated oidcClientAPI
	if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/oidc_clients/%d", r.client.Endpoint, id), body, &updated); err != nil {
		diags.AddError("Failed to update OIDC client", fmt.Sprintf("failed to update OIDC client %d: %s", id, err))
		return oidcClientAPI{}, false
	}
	if updated.ID == 0 {
		// An empty answer: read the client back instead.
		if updated, err = oidcGetClient(ctx, r.client, id); err != nil {
			diags.AddError("Failed to read OIDC client", fmt.Sprintf("failed to read OIDC client %d: %s", id, err))
			return oidcClientAPI{}, false
		}
	}
	return updated, true
}

// fromAPI copies what Portainer reports into m. Optional attributes the
// configuration leaves out stay null while Portainer has nothing for them;
// the first secret and its expiry are never returned again and keep their
// prior values.
func (r *oidcClientResource) fromAPI(c oidcClientAPI, m *oidcClientModel) {
	m.ID = types.StringValue(fwID(c.ID))
	m.Name = types.StringValue(c.Name)
	m.Description = egOptString(c.Description, m.Description)
	m.RedirectURIs = oidcStringList(c.RedirectURIs)
	m.PostLogoutRedirectURIs = oidcOptStringList(c.PostLogoutRedirectURIs, m.PostLogoutRedirectURIs)
	m.HomepageURL = egOptString(c.HomepageURL, m.HomepageURL)
	permission := c.Permission
	if permission == "" {
		permission = "none"
	}
	m.Permission = types.StringValue(permission)
	m.AllUsers = types.BoolValue(c.AllUsers)
	m.GroupsClaim = types.BoolValue(c.GroupsClaim)
	m.ShowInLauncher = types.BoolValue(c.ShowInLauncher)
	m.Enabled = types.BoolValue(c.Enabled)
	m.UserAccessPolicies = oidcPoliciesValue(c.UserAccessPolicies, m.UserAccessPolicies)
	m.TeamAccessPolicies = oidcPoliciesValue(c.TeamAccessPolicies, m.TeamAccessPolicies)
	m.ClientID = types.StringValue(c.ClientID)
}

func (r *oidcClientResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	fwSetIDIdentity(ctx, req.State, resp.Identity, &resp.Diagnostics)
	var state oidcClientModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := oidcParseID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to read OIDC client", err.Error())
		return
	}
	c, err := oidcGetClient(ctx, r.client, id)
	if err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read OIDC client", fmt.Sprintf("failed to read OIDC client %d: %s", id, err))
		return
	}
	r.fromAPI(c, &state)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *oidcClientResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan oidcClientModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	c, ok := r.update(ctx, &plan, &resp.Diagnostics)
	if !ok {
		return
	}
	r.fromAPI(c, &plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *oidcClientResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state oidcClientModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, err := oidcParseID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to delete OIDC client", err.Error())
		return
	}
	if err := doJSON(ctx, r.client, http.MethodDelete, fmt.Sprintf("%s/oidc_clients/%d", r.client.Endpoint, id), nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete OIDC client", fmt.Sprintf("failed to delete OIDC client %d: %s", id, err))
	}
}

func (r *oidcClientResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	fwImportIDOrIdentity(ctx, req, resp)
}

// newOIDCClientListResource lists the OIDC clients for `terraform query`.
var newOIDCClientListResource = fwNewIDList(fwIDListResource{
	typeSuffix:  "_oidc_client",
	description: "Lists the clients of Portainer's OIDC provider. Business Edition 3.1 or later.",
	newResource: newOIDCClientResource,
	filters:     fwListNameFilter("OIDC clients"),
	items: func(ctx context.Context, client *APIClient, config tfsdk.Config, diags *diag.Diagnostics) []fwIDListItem {
		name := fwListFilter(ctx, config, "name", diags)
		clients, err := oidcListClients(ctx, client, "portainer_oidc_client list")
		if err != nil {
			diags.AddError("Failed to list OIDC clients", err.Error())
			return nil
		}
		var out []fwIDListItem
		for _, c := range clients {
			if name == "" || c.Name == name {
				out = append(out, fwIDListItem{id: fwID(c.ID), name: c.Name})
			}
		}
		return out
	},
})
