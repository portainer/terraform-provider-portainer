package internal

import (
	"cmp"
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/listplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// gitopsSourceDetail is the sources.SourceDetail payload returned by
// GET /gitops/sources/{id}.
type gitopsSourceDetail struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Interval   string `json:"interval"`
	LastSync   int    `json:"lastSync"`
	Error      string `json:"error"`
	Connection struct {
		TLSSkipVerify  bool `json:"tlsSkipVerify"`
		Authentication struct {
			Username string `json:"username"`
		} `json:"authentication"`
	} `json:"connection"`
	Access struct {
		Public bool  `json:"public"`
		Teams  []int `json:"teams"`
		Users  []int `json:"users"`
	} `json:"access"`
}

var (
	_ resource.Resource                = (*gitopsSourceResource)(nil)
	_ resource.ResourceWithConfigure   = (*gitopsSourceResource)(nil)
	_ resource.ResourceWithImportState = (*gitopsSourceResource)(nil)
)

func newGitopsSourceResource() resource.Resource { return &gitopsSourceResource{} }

type gitopsSourceResource struct {
	fwIDIdentity
	client *APIClient
}

type gitopsSourceResModel struct {
	ID                 types.String `tfsdk:"id"`
	URL                types.String `tfsdk:"url"`
	Name               types.String `tfsdk:"name"`
	Interval           types.String `tfsdk:"interval"`
	Username           types.String `tfsdk:"username"`
	Password           types.String `tfsdk:"password"`
	PasswordWO         types.String `tfsdk:"password_wo"`
	PasswordWOVersion  types.Int64  `tfsdk:"password_wo_version"`
	TLSSkipVerify      types.Bool   `tfsdk:"tls_skip_verify"`
	AdministratorsOnly types.Bool   `tfsdk:"administrators_only"`
	Public             types.Bool   `tfsdk:"public"`
	UserAccesses       types.List   `tfsdk:"user_accesses"`
	TeamAccesses       types.List   `tfsdk:"team_accesses"`
	Type               types.String `tfsdk:"type"`
	Status             types.String `tfsdk:"status"`
	StatusError        types.String `tfsdk:"status_error"`
	LastSync           types.Int64  `tfsdk:"last_sync"`
}

func (r *gitopsSourceResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gitops_source"
}

func (r *gitopsSourceResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"url": schema.StringAttribute{
				Required: true,
				// Portainer stores the URL normalized (lowercase scheme and
				// host, no credentials, no trailing slash or .git); reads keep
				// the configured spelling when it normalizes to the same URL.
				Description: "URL of the Git repository the source syncs from. Portainer stores it normalized (without a trailing `/` or `.git`); such differences are not shown as changes.",
			},
			"name": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "Name of the GitOps source as displayed in Portainer. Portainer derives one from the repository URL when this is not set.",
			},
			"interval": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "Polling interval for the repository as a Go duration string (for example `5m`). Leave unset to keep Portainer's default.",
			},
			"username": schema.StringAttribute{
				Optional:    true,
				Description: "Username used to authenticate against the Git repository. Leave unset for a public repository.",
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Validators:  []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("password_wo"))},
				Description: "Password or personal access token paired with `username`. Portainer never returns it, so it is kept from configuration and cannot detect drift. Use `password_wo` to keep it out of state.",
			},
			"password_wo": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("password")),
					stringvalidator.AlsoRequires(path.MatchRoot("password_wo_version")),
				},
				Description: "Write-only password or personal access token paired with `username` (supports ephemeral values; never stored in Terraform state). Conflicts with `password`. Requires Terraform 1.11 or later.",
			},
			"password_wo_version": schema.Int64Attribute{
				Optional:    true,
				Description: "Version of `password_wo`. Increment it to send a new value.",
			},
			"tls_skip_verify": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether to skip TLS certificate verification when contacting the Git repository.",
			},
			"administrators_only": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(false),
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.RequiresReplace()},
				Description:   "Whether the source is restricted to administrators. Only honoured at creation time; Portainer's update API does not carry this field.",
			},
			"public": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				Description:   "Whether every user can use this source. Changing it is applied through the source's access-control endpoint.",
			},
			"user_accesses": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.Int64Type,
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				Description:   "IDs of users granted access to the source. Applied through the source's access-control endpoint.",
			},
			"team_accesses": schema.ListAttribute{
				Optional:      true,
				Computed:      true,
				ElementType:   types.Int64Type,
				PlanModifiers: []planmodifier.List{listplanmodifier.UseStateForUnknown()},
				Description:   "IDs of teams granted access to the source. Applied through the source's access-control endpoint.",
			},
			"type": schema.StringAttribute{
				Computed:    true,
				Description: "Source type reported by Portainer: `git`, `helm` or `oci`. This resource always creates a `git` source.",
			},
			"status": schema.StringAttribute{
				Computed:    true,
				Description: "Sync status of the source: `healthy`, `syncing`, `error`, `paused` or `unknown`.",
			},
			"status_error": schema.StringAttribute{
				Computed:    true,
				Description: "Error reported by the last synchronisation attempt, empty while the source is healthy.",
			},
			"last_sync": schema.Int64Attribute{
				Computed:    true,
				Description: "Unix timestamp of the last successful synchronisation, zero when the source has never synced.",
			},
		},
	}
}

func (r *gitopsSourceResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// gitopsAuth is the authentication object of a create/update payload, nil
// when no credentials are configured.
func gitopsAuth(username, password string) map[string]interface{} {
	if username == "" && password == "" {
		return nil
	}
	return map[string]interface{}{"username": username, "password": password}
}

func (r *gitopsSourceResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan gitopsSourceResModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := map[string]interface{}{
		"url":                plan.URL.ValueString(),
		"tlsSkipVerify":      plan.TLSSkipVerify.ValueBool(),
		"administratorsOnly": plan.AdministratorsOnly.ValueBool(),
		"public":             plan.Public.ValueBool(),
	}
	if v := gitopsResString(plan.Name); v != "" {
		payload["name"] = v
	}
	if v := gitopsResString(plan.Interval); v != "" {
		payload["interval"] = v
	}
	if auth := gitopsAuth(gitopsResString(plan.Username), gitopsSourcePassword(ctx, req.Config, plan, &resp.Diagnostics)); auth != nil {
		payload["authentication"] = auth
	}
	if v := int64ListToInts(ctx, plan.UserAccesses, &resp.Diagnostics); len(v) > 0 {
		payload["userAccesses"] = v
	}
	if v := int64ListToInts(ctx, plan.TeamAccesses, &resp.Diagnostics); len(v) > 0 {
		payload["teamAccesses"] = v
	}
	if resp.Diagnostics.HasError() {
		return
	}

	var created struct {
		ID int `json:"id"`
	}
	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/gitops/sources/git", payload, &created); err != nil {
		gitopsResErrorf(&resp.Diagnostics, "failed to create GitOps source: %w", err)
		return
	}
	if created.ID == 0 {
		gitopsResErrorf(&resp.Diagnostics, "failed to create GitOps source: Portainer returned no source ID")
		return
	}

	plan.ID = types.StringValue(strconv.Itoa(created.ID))
	plan.PasswordWO = types.StringNull()
	if !r.read(ctx, &plan, true, &resp.Diagnostics) {
		return
	}
	// Recorded even when the read failed, as SDKv2 kept the ID it had set.
	gitopsSourceSettleUnknowns(&plan)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// gitopsSourceSettleUnknowns replaces the computed values a failed read left
// unknown with zero values.
func gitopsSourceSettleUnknowns(m *gitopsSourceResModel) {
	for _, s := range []*types.String{&m.Name, &m.Interval, &m.Type, &m.Status, &m.StatusError} {
		if s.IsUnknown() {
			*s = types.StringValue("")
		}
	}
	if m.Public.IsUnknown() {
		m.Public = types.BoolValue(false)
	}
	for _, l := range []*types.List{&m.UserAccesses, &m.TeamAccesses} {
		if l.IsUnknown() {
			*l = int64ListValue(nil)
		}
	}
	if m.LastSync.IsUnknown() {
		m.LastSync = types.Int64Value(0)
	}
}

// gitopsSourcePassword is the password to send: the write-only password_wo
// when its version is set and it is configured, the password otherwise.
func gitopsSourcePassword(ctx context.Context, config tfsdk.Config, plan gitopsSourceResModel, diags *diag.Diagnostics) string {
	if plan.PasswordWOVersion.ValueInt64() != 0 {
		var wo types.String
		diags.Append(config.GetAttribute(ctx, path.Root("password_wo"), &wo)...)
		if s := gitopsResString(wo); s != "" {
			return s
		}
	}
	return gitopsResString(plan.Password)
}

// read refreshes m from Portainer; false when the source is gone (an API
// error is reported in diags and counts as found). After an
// apply (applied) the arguments Terraform planned keep their planned value,
// so the apply is consistent with its plan; a refresh takes Portainer's.
func (r *gitopsSourceResource) read(ctx context.Context, m *gitopsSourceResModel, applied bool, diags *diag.Diagnostics) bool {
	var source gitopsSourceDetail
	sourceURL := fmt.Sprintf("%s/gitops/sources/%s", r.client.Endpoint, m.ID.ValueString())
	if err := doJSON(ctx, r.client, http.MethodGet, sourceURL, nil, &source); err != nil {
		if isAPINotFound(err) {
			return false
		}
		gitopsResErrorf(diags, "failed to read GitOps source %s: %w", m.ID.ValueString(), err)
		return true
	}

	teams := source.Access.Teams
	if teams == nil {
		teams = []int{}
	}
	users := source.Access.Users
	if users == nil {
		users = []int{}
	}

	keep := func(v interface{ IsUnknown() bool }) bool { return applied && !v.IsUnknown() }

	// Portainer stores the URL normalized, so the configured spelling is
	// kept as long as it names the same repository.
	if !keep(m.URL) && (m.URL.IsNull() || normalizeGitURL(m.URL.ValueString()) != normalizeGitURL(source.URL)) {
		m.URL = types.StringValue(source.URL)
	}
	if !keep(m.Name) {
		m.Name = types.StringValue(source.Name)
	}
	if !keep(m.Interval) {
		m.Interval = types.StringValue(source.Interval)
	}
	if !keep(m.TLSSkipVerify) {
		m.TLSSkipVerify = types.BoolValue(source.Connection.TLSSkipVerify)
	}
	// The password is never returned, so it stays as configured; the
	// username reflects what Portainer stores. No username is null, as an
	// unset argument is: SDKv2 recorded it as "" and treated both the same.
	if !applied {
		m.Username = types.StringNull()
		if u := source.Connection.Authentication.Username; u != "" {
			m.Username = types.StringValue(u)
		}
	}
	if !keep(m.Public) {
		m.Public = types.BoolValue(source.Access.Public)
	}
	if !keep(m.TeamAccesses) {
		m.TeamAccesses = int64ListValue(teams)
	}
	if !keep(m.UserAccesses) {
		m.UserAccesses = int64ListValue(users)
	}
	m.Type = types.StringValue(source.Type)
	m.Status = types.StringValue(source.Status)
	m.StatusError = types.StringValue(source.Error)
	m.LastSync = types.Int64Value(int64(source.LastSync))
	return true
}

func (r *gitopsSourceResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	fwSetIDIdentity(ctx, req.State, resp.Identity, &resp.Diagnostics)
	var state gitopsSourceResModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.read(ctx, &state, false, &resp.Diagnostics) {
		resp.State.RemoveResource(ctx)
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *gitopsSourceResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan, state gitopsSourceResModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID
	id := plan.ID.ValueString()

	// Portainer splits the update in two: the git connection is updated on the
	// source itself, while access control has its own endpoint and is not part
	// of sources.GitSourceUpdatePayload. A URL only spelled differently is no
	// change, as Portainer stores both the same way.
	connectionChanged := normalizeGitURL(plan.URL.ValueString()) != normalizeGitURL(state.URL.ValueString()) ||
		!plan.Name.Equal(state.Name) || !plan.Interval.Equal(state.Interval) ||
		!plan.TLSSkipVerify.Equal(state.TLSSkipVerify) || gitopsResString(plan.Username) != gitopsResString(state.Username) ||
		gitopsResString(plan.Password) != gitopsResString(state.Password) ||
		plan.PasswordWOVersion.ValueInt64() != state.PasswordWOVersion.ValueInt64()
	if connectionChanged {
		payload := map[string]interface{}{
			"url":           plan.URL.ValueString(),
			"name":          gitopsResString(plan.Name),
			"interval":      gitopsResString(plan.Interval),
			"tlsSkipVerify": plan.TLSSkipVerify.ValueBool(),
		}
		if auth := gitopsAuth(gitopsResString(plan.Username), gitopsSourcePassword(ctx, req.Config, plan, &resp.Diagnostics)); auth != nil {
			payload["authentication"] = auth
		}
		if resp.Diagnostics.HasError() {
			return
		}
		if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/gitops/sources/%s", r.client.Endpoint, id), payload, nil); err != nil {
			gitopsResErrorf(&resp.Diagnostics, "failed to update GitOps source %s: %w", id, err)
			return
		}
	}

	if !plan.Public.Equal(state.Public) || !plan.UserAccesses.Equal(state.UserAccesses) || !plan.TeamAccesses.Equal(state.TeamAccesses) {
		payload := map[string]interface{}{
			"public": plan.Public.ValueBool(),
			"users":  gitopsSourceAccessIDs(ctx, plan.UserAccesses, &resp.Diagnostics),
			"teams":  gitopsSourceAccessIDs(ctx, plan.TeamAccesses, &resp.Diagnostics),
		}
		if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/gitops/sources/%s/access", r.client.Endpoint, id), payload, nil); err != nil {
			gitopsResErrorf(&resp.Diagnostics, "failed to update access control of GitOps source %s: %w", id, err)
			return
		}
	}

	plan.PasswordWO = types.StringNull()
	if !r.read(ctx, &plan, true, &resp.Diagnostics) {
		resp.State.RemoveResource(ctx)
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// gitopsSourceAccessIDs is an access list as SDKv2 sent it: empty, never null.
func gitopsSourceAccessIDs(ctx context.Context, l types.List, diags *diag.Diagnostics) []int {
	ids := int64ListToInts(ctx, l, diags)
	if ids == nil {
		ids = []int{}
	}
	return ids
}

func (r *gitopsSourceResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state gitopsSourceResModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	sourceURL := fmt.Sprintf("%s/gitops/sources/%s", r.client.Endpoint, state.ID.ValueString())
	if err := doJSON(ctx, r.client, http.MethodDelete, sourceURL, nil, nil); err != nil && !isAPINotFound(err) {
		gitopsResErrorf(&resp.Diagnostics, "failed to delete GitOps source %s: %w", state.ID.ValueString(), err)
	}
}

// ImportState takes the source ID (or an identity with it). administrators_only is not readable, so it
// starts at its default: SDKv2 read the missing value as false and planned no
// replacement for it.
func (r *gitopsSourceResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	fwImportIDOrIdentity(ctx, req, resp)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("administrators_only"), false)...)
}

// normalizeGitURL mirrors Portainer's gittypes.NormalizeURL: lowercase scheme
// (https when missing) and host, no credentials, no trailing slash or .git.
// An unparsable URL is returned unchanged.
func normalizeGitURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	u.Scheme = strings.ToLower(cmp.Or(u.Scheme, "https"))
	u.Host = strings.ToLower(u.Host)
	u.User = nil
	u.Path = strings.TrimSuffix(strings.TrimRight(u.Path, "/"), ".git")
	return u.String()
}
