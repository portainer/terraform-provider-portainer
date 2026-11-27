package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*registryResource)(nil)
	_ resource.ResourceWithConfigure   = (*registryResource)(nil)
	_ resource.ResourceWithImportState = (*registryResource)(nil)
	_ resource.ResourceWithIdentity    = (*registryResource)(nil)
)

func newRegistryResource() resource.Resource { return &registryResource{} }

type registryResource struct {
	fwIDIdentity
	client *APIClient
}

type registryResourceModel struct {
	ID                     types.String `tfsdk:"id"`
	Name                   types.String `tfsdk:"name"`
	URL                    types.String `tfsdk:"url"`
	BaseURL                types.String `tfsdk:"base_url"`
	Type                   types.Int64  `tfsdk:"type"`
	Authentication         types.Bool   `tfsdk:"authentication"`
	Username               types.String `tfsdk:"username"`
	Password               types.String `tfsdk:"password"`
	PasswordWO             types.String `tfsdk:"password_wo"`
	PasswordWOVersion      types.Int64  `tfsdk:"password_wo_version"`
	InstanceURL            types.String `tfsdk:"instance_url"`
	AWSRegion              types.String `tfsdk:"aws_region"`
	GithubUseOrganisation  types.Bool   `tfsdk:"github_use_organisation"`
	GithubOrganisationName types.String `tfsdk:"github_organisation_name"`
	QuayUseOrganisation    types.Bool   `tfsdk:"quay_use_organisation"`
	QuayOrganisationName   types.String `tfsdk:"quay_organisation_name"`
}

func (r *registryResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_registry"
}

// The optional strings Read fills in from Portainer were stored as "" by
// v3.0.0 when unset; registryReadString keeps them null for new states and
// egKeepZeroString keeps the "" of an old one, so neither plans a change.
func registryReadString(desc string) schema.StringAttribute {
	return schema.StringAttribute{
		Optional:      true,
		Computed:      true,
		Description:   desc,
		PlanModifiers: []planmodifier.String{egKeepZeroString{}},
	}
}

func (r *registryResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the registry.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name":     schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, Description: "Name of the registry as displayed in Portainer."},
			"url":      schema.StringAttribute{Required: true, Validators: []validator.String{stringvalidator.LengthAtLeast(1)}, Description: "URL of the registry (e.g. registry hostname). Used by Portainer to push and pull images."},
			"base_url": registryReadString("Optional base URL of the registry used by Portainer for web-based actions and links."),
			"type": schema.Int64Attribute{
				Required:      true,
				Validators:    []validator.Int64{int64validator.Between(1, 8)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				Description:   "Registry type: 1 = Quay, 2 = Azure, 3 = Custom, 4 = GitLab, 5 = ProGet, 6 = DockerHub, 7 = ECR, 8 = GitHub. Changing this value forces resource recreation.",
			},
			"authentication": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Whether the registry requires authentication. When true, username and password are sent to Portainer. Defaults to true when `username` is set, false otherwise.",
				PlanModifiers: []planmodifier.Bool{registryAuthenticationPlan{}},
			},
			"username": registryReadString("Username used to authenticate against the registry when authentication is enabled."),
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Validators:  []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("password_wo"))},
				Description: "Password used to authenticate against the registry when authentication is enabled. Stored in state as a sensitive value; use `password_wo` to keep it out of state.",
			},
			"password_wo": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("password")),
					stringvalidator.AlsoRequires(path.MatchRoot("password_wo_version")),
				},
				Description: "Write-only registry password (supports ephemeral values; never stored in Terraform state). Sent on create and whenever `password_wo_version` changes. Requires Terraform 1.11 or later.",
			},
			"password_wo_version": schema.Int64Attribute{
				Optional:    true,
				Description: "Version of `password_wo`. Increment it to send a new password; the registry is updated in place.",
			},
			"instance_url":             registryReadString("Instance URL used by GitLab (type 4) registries to identify the GitLab instance."),
			"aws_region":               registryReadString("AWS region used by ECR (type 7) registries."),
			"github_use_organisation":  schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether the GitHub (type 8) registry should be scoped to an organisation rather than a user account."},
			"github_organisation_name": registryReadString("Name of the GitHub organisation used when github_use_organisation is true (type 8)."),
			"quay_use_organisation":    schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether the Quay (type 1) registry should be scoped to an organisation rather than a user account."},
			"quay_organisation_name":   registryReadString("Name of the Quay organisation used when quay_use_organisation is true (type 1)."),
		},
	}
}

// registryAuthenticationPlan plans the authentication registryAuth will send
// when it is not configured: whether a username is set. The documented Azure,
// GitLab, ProGet and Docker Hub examples set credentials without
// authentication = true, which used to create those registries without them.
type registryAuthenticationPlan struct{}

func (registryAuthenticationPlan) Description(context.Context) string {
	return "Defaults to whether a username is set."
}

func (m registryAuthenticationPlan) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (registryAuthenticationPlan) PlanModifyBool(ctx context.Context, req planmodifier.BoolRequest, resp *planmodifier.BoolResponse) {
	if !req.ConfigValue.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var username types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("username"), &username)...)
	if username.IsUnknown() {
		if !req.StateValue.IsNull() {
			resp.PlanValue = req.StateValue
		}
		return
	}
	resp.PlanValue = types.BoolValue(username.ValueString() != "")
}

func (r *registryResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *registryResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	fwImportIDOrIdentity(ctx, req, resp)
}

// registryAuth is the configured authentication, or whether a username is set.
func registryAuth(ctx context.Context, config tfsdk.Config, m *registryResourceModel, diags *diag.Diagnostics) bool {
	var v types.Bool
	diags.Append(config.GetAttribute(ctx, path.Root("authentication"), &v)...)
	if !v.IsNull() && !v.IsUnknown() {
		return v.ValueBool()
	}
	return m.Username.ValueString() != ""
}

// registryPasswordValue returns the password to send: password_wo when sendWO
// is set (on create, or when password_wo_version changed), otherwise password.
func registryPasswordValue(m *registryResourceModel, wo string, sendWO bool) string {
	if sendWO && m.PasswordWOVersion.ValueInt64() != 0 && wo != "" {
		return wo
	}
	return m.Password.ValueString()
}

func (r *registryResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan registryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	wo := writeOnlyConfigString(ctx, req.Config, path.Root("password_wo"), &resp.Diagnostics)
	auth := registryAuth(ctx, req.Config, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	name := plan.Name.ValueString()

	existingID, err := findRegistryByName(ctx, r.client, name)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create registry", err.Error())
		return
	}
	m := plan
	if existingID != 0 {
		m.ID = types.StringValue(strconv.Itoa(existingID))
		// On create every set value counts as changed, the version too.
		if !r.update(ctx, &m, auth, registryPasswordValue(&m, wo, m.PasswordWOVersion.ValueInt64() != 0), &resp.Diagnostics) {
			return
		}
		registryFinish(ctx, &m, &plan, &resp.State, &resp.Diagnostics)
		return
	}

	registryType := plan.Type.ValueInt64()
	body := registryPayload{
		Name:           name,
		Type:           &registryType,
		URL:            plan.URL.ValueString(),
		BaseURL:        plan.BaseURL.ValueString(),
		Authentication: auth,
	}
	if auth {
		body.Username = plan.Username.ValueString()
		body.Password = registryPasswordValue(&plan, wo, true)
	}
	switch registryType {
	case 1: // Quay.io
		body.Quay = &registryOrganisation{UseOrganisation: plan.QuayUseOrganisation.ValueBool(), OrganisationName: plan.QuayOrganisationName.ValueString()}
	case 4: // GitLab
		body.Gitlab = &registryGitlab{InstanceURL: plan.InstanceURL.ValueString()}
	case 7: // AWS ECR
		body.Ecr = &registryEcr{Region: plan.AWSRegion.ValueString()}
	case 8: // GitHub
		body.Github = &registryOrganisation{UseOrganisation: plan.GithubUseOrganisation.ValueBool(), OrganisationName: plan.GithubOrganisationName.ValueString()}
	}

	var created portainerRegistry
	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/registries", body, &created); err != nil {
		resp.Diagnostics.AddError("Failed to create registry", fmt.Errorf("failed to create registry: %w", err).Error())
		return
	}
	m.ID = types.StringValue(strconv.FormatInt(created.ID, 10))
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), m.ID)...)
	found, err := registryRead(ctx, r.client, &m)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read registry", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to read registry", fmt.Sprintf("registry %s was not found right after it was created", m.ID.ValueString()))
		return
	}
	registryFinish(ctx, &m, &plan, &resp.State, &resp.Diagnostics)
}

func (r *registryResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	fwSetIDIdentity(ctx, req.State, resp.Identity, &resp.Diagnostics)
	var state registryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := registryRead(ctx, r.client, &state)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read registry", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *registryResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan, state registryResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	wo := writeOnlyConfigString(ctx, req.Config, path.Root("password_wo"), &resp.Diagnostics)
	auth := registryAuth(ctx, req.Config, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	m := plan
	m.ID = state.ID
	sendWO := plan.PasswordWOVersion.ValueInt64() != state.PasswordWOVersion.ValueInt64()
	if !r.update(ctx, &m, auth, registryPasswordValue(&m, wo, sendWO), &resp.Diagnostics) {
		return
	}
	registryFinish(ctx, &m, &plan, &resp.State, &resp.Diagnostics)
}

// update sends PUT /registries/{id} and reads the registry back into m.
func (r *registryResource) update(ctx context.Context, m *registryResourceModel, auth bool, password string, diags *diag.Diagnostics) bool {
	id, _ := strconv.ParseInt(m.ID.ValueString(), 10, 64)
	body := registryPayload{
		Name:           m.Name.ValueString(),
		URL:            m.URL.ValueString(),
		BaseURL:        m.BaseURL.ValueString(),
		Authentication: auth,
		Username:       m.Username.ValueString(),
		// An empty password is left out, so Portainer keeps the stored one.
		Password: password,
	}
	switch m.Type.ValueInt64() {
	case 1: // Quay.io
		body.Quay = &registryOrganisation{UseOrganisation: m.QuayUseOrganisation.ValueBool(), OrganisationName: m.QuayOrganisationName.ValueString()}
	// The update payload has no GitLab settings and no type.
	case 7: // AWS ECR
		body.Ecr = &registryEcr{Region: m.AWSRegion.ValueString()}
	case 8: // GitHub
		body.Github = &registryOrganisation{UseOrganisation: m.GithubUseOrganisation.ValueBool(), OrganisationName: m.GithubOrganisationName.ValueString()}
	}
	if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/registries/%d", r.client.Endpoint, id), body, nil); err != nil {
		diags.AddError("Failed to update registry", fmt.Errorf("failed to update registry: %w", err).Error())
		return false
	}
	found, err := registryRead(ctx, r.client, m)
	if err != nil {
		diags.AddError("Failed to read registry", err.Error())
		return false
	}
	if !found {
		diags.AddError("Failed to read registry", fmt.Sprintf("registry %d was not found after the update", id))
		return false
	}
	return true
}

// registryFinish stores the result of Create or Update: the planned values,
// which Terraform requires back unchanged, and what Portainer reported for
// the rest.
func registryFinish(ctx context.Context, m, plan *registryResourceModel, state *tfsdk.State, diags *diag.Diagnostics) {
	out := *plan
	out.ID = m.ID
	for _, p := range []struct{ dst, src *types.String }{
		{&out.BaseURL, &m.BaseURL},
		{&out.Username, &m.Username},
		{&out.InstanceURL, &m.InstanceURL},
		{&out.AWSRegion, &m.AWSRegion},
		{&out.GithubOrganisationName, &m.GithubOrganisationName},
		{&out.QuayOrganisationName, &m.QuayOrganisationName},
	} {
		if p.dst.IsUnknown() {
			*p.dst = *p.src
		}
	}
	if out.Authentication.IsUnknown() {
		out.Authentication = m.Authentication
	}
	out.PasswordWO = types.StringNull()
	diags.Append(state.Set(ctx, &out)...)
}

func (r *registryResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state registryResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, _ := strconv.ParseInt(state.ID.ValueString(), 10, 64)
	if err := doJSON(ctx, r.client, http.MethodDelete, fmt.Sprintf("%s/registries/%d", r.client.Endpoint, id), nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete registry", fmt.Errorf("failed to delete registry: %w", err).Error())
	}
}

// registryOptString returns the state value of an optional string Read
// fills in: null for an empty API value while the prior value is null.
func registryOptString(api string, prior types.String) types.String {
	if api == "" && (prior.IsNull() || prior.IsUnknown()) {
		return types.StringNull()
	}
	return types.StringValue(api)
}

// registryRead refreshes m from GET /registries/{id}; false when it is gone.
func registryRead(ctx context.Context, client *APIClient, m *registryResourceModel) (bool, error) {
	id, _ := strconv.ParseInt(m.ID.ValueString(), 10, 64)
	registry, err := getRegistry(ctx, client, id)
	if err != nil {
		if isAPINotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read registry: %w", err)
	}
	m.Name = types.StringValue(registry.Name)
	m.URL = types.StringValue(registry.URL)
	m.BaseURL = registryOptString(registry.BaseURL, m.BaseURL)
	m.Type = types.Int64Value(registry.Type)
	m.Authentication = types.BoolValue(registry.Authentication)
	m.Username = registryOptString(registry.Username, m.Username)
	if registry.Github != nil {
		m.GithubUseOrganisation = types.BoolValue(registry.Github.UseOrganisation)
		m.GithubOrganisationName = registryOptString(registry.Github.OrganisationName, m.GithubOrganisationName)
	}
	if registry.Quay != nil {
		m.QuayUseOrganisation = types.BoolValue(registry.Quay.UseOrganisation)
		m.QuayOrganisationName = registryOptString(registry.Quay.OrganisationName, m.QuayOrganisationName)
	}
	if registry.Gitlab != nil {
		m.InstanceURL = registryOptString(registry.Gitlab.InstanceURL, m.InstanceURL)
	}
	if registry.Ecr != nil {
		m.AWSRegion = registryOptString(registry.Ecr.Region, m.AWSRegion)
	}
	return true, nil
}

func findRegistryByName(ctx context.Context, client *APIClient, name string) (int, error) {
	registryList, err := listRegistries(ctx, client)
	if err != nil {
		return 0, err
	}
	for _, r := range registryList {
		if r.Name == name {
			return int(r.ID), nil
		}
	}
	return 0, nil
}

// registryPayload is the POST /registries and PUT /registries/{id} body. Type
// and GitLab settings are only part of the create payload.
type registryPayload struct {
	Name           string                `json:"name"`
	Type           *int64                `json:"type,omitempty"`
	URL            string                `json:"url"`
	BaseURL        string                `json:"baseURL,omitempty"`
	Authentication bool                  `json:"authentication"`
	Username       string                `json:"username,omitempty"`
	Password       string                `json:"password,omitempty"`
	Quay           *registryOrganisation `json:"quay,omitempty"`
	Github         *registryOrganisation `json:"github,omitempty"`
	Gitlab         *registryGitlab       `json:"gitlab,omitempty"`
	Ecr            *registryEcr          `json:"ecr,omitempty"`
}

type registryOrganisation struct {
	UseOrganisation  bool   `json:"UseOrganisation,omitempty"`
	OrganisationName string `json:"OrganisationName,omitempty"`
}

type registryGitlab struct {
	InstanceURL string `json:"InstanceURL,omitempty"`
}

type registryEcr struct {
	Region string `json:"Region,omitempty"`
}

type portainerRegistry struct {
	ID               int64                             `json:"Id"`
	Name             string                            `json:"Name"`
	Type             int64                             `json:"Type"`
	URL              string                            `json:"URL"`
	BaseURL          string                            `json:"BaseURL"`
	Authentication   bool                              `json:"Authentication"`
	Username         string                            `json:"Username"`
	Quay             *registryOrganisation             `json:"Quay"`
	Github           *registryOrganisation             `json:"Github"`
	Gitlab           *registryGitlab                   `json:"Gitlab"`
	Ecr              *registryEcr                      `json:"Ecr"`
	RegistryAccesses map[string]registryAccessPolicies `json:"RegistryAccesses"`
}

// getRegistry returns the registry (GET /registries/{id}).
func getRegistry(ctx context.Context, client *APIClient, id int64) (*portainerRegistry, error) {
	var registry portainerRegistry
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/registries/%d", client.Endpoint, id), nil, &registry); err != nil {
		return nil, err
	}
	return &registry, nil
}

// listRegistries returns every registry (GET /registries).
func listRegistries(ctx context.Context, client *APIClient) ([]portainerRegistry, error) {
	var registryList []portainerRegistry
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/registries", nil, &registryList); err != nil {
		return nil, fmt.Errorf("failed to list registries: %w", err)
	}
	return registryList, nil
}
