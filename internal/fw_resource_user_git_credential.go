package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*userGitCredentialResource)(nil)
	_ resource.ResourceWithConfigure   = (*userGitCredentialResource)(nil)
	_ resource.ResourceWithImportState = (*userGitCredentialResource)(nil)
)

func newUserGitCredentialResource() resource.Resource { return &userGitCredentialResource{} }

type userGitCredentialResource struct{ client *APIClient }

type userGitCredentialResourceModel struct {
	ID                types.String `tfsdk:"id"`
	UserID            types.Int64  `tfsdk:"user_id"`
	CredentialID      types.Int64  `tfsdk:"credential_id"`
	Name              types.String `tfsdk:"name"`
	Username          types.String `tfsdk:"username"`
	Password          types.String `tfsdk:"password"`
	AuthorizationType types.Int64  `tfsdk:"authorization_type"`
}

func (r *userGitCredentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_user_git_credential"
}

func (r *userGitCredentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Description: "Manages a git credential of one user (Business Edition 2.42 and earlier). " +
			"Deprecated: Portainer Business Edition 2.43 removed the per-user git credential API; use portainer_shared_git_credential.",
		DeprecationMessage: userGitCredentialDeprecation,
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the git credential, in the form `<user_id>:<credential_id>`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"user_id": schema.Int64Attribute{
				Required:      true,
				Description:   "ID of the user who owns this git credential",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
			},
			"credential_id": schema.Int64Attribute{
				Computed:      true,
				Description:   "ID of the git credential",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the git credential",
				Validators:  []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"username": schema.StringAttribute{
				Required:    true,
				Description: "Username for git authentication",
			},
			"password": schema.StringAttribute{
				Required:    true,
				Sensitive:   true,
				Description: "Password or token for git authentication",
			},
			"authorization_type": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(0),
				Description: "Authorization type: 0 = Basic, 1 = Token",
				Validators:  []validator.Int64{int64validator.Between(0, 1)},
			},
		},
	}
}

func (r *userGitCredentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// userGitCredentialDeprecation: the per-user git credential API
// (/users/{id}/gitcredentials) is in the Business Edition specifications up
// to 2.42.0 and in none from 2.43.0 on (Community Edition never had it); its
// replacement is the shared git credential API (/cloud/gitcredentials,
// portainer_shared_git_credential), which Business Edition has since 2.39.
const userGitCredentialDeprecation = "portainer_user_git_credential is deprecated: Portainer Business Edition 2.43 and later " +
	"no longer have per-user git credentials (/users/{id}/gitcredentials). Use portainer_shared_git_credential instead."

// userGitCredentialRemovedIn is the first release without the API.
const userGitCredentialRemovedIn = "2.43.0"

// explainUserGitCredentialRemoved turns the 404 (or 405) of a server without
// the per-user git credential API into an error that says it was removed and
// names the replacement. Other errors, and the 404 of a server that still has
// the API, are returned unchanged.
func explainUserGitCredentialRemoved(ctx context.Context, client *APIClient, err error) error {
	if !isAPINotFound(err) && !isAPIStatus(err, http.StatusMethodNotAllowed) {
		return err
	}
	var version struct {
		ServerVersion string `json:"ServerVersion"`
		ServerEdition string `json:"ServerEdition"`
	}
	if verr := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/system/version", nil, &version); verr != nil || version.ServerVersion == "" {
		return err
	}
	edition := version.ServerEdition
	if edition == "" {
		edition = "CE"
	}
	if edition != "EE" && edition != "BE" {
		return fmt.Errorf("portainer_user_git_credential requires Portainer Business Edition 2.42 or earlier; this server is Portainer %s %s: %w",
			edition, version.ServerVersion, err)
	}
	if !versionLess(version.ServerVersion, userGitCredentialRemovedIn) {
		return fmt.Errorf("portainer_user_git_credential is not available on Portainer %s or later: the per-user git credential API "+
			"was removed; this server is Portainer %s %s. Use portainer_shared_git_credential instead: %w",
			strings.TrimSuffix(userGitCredentialRemovedIn, ".0"), edition, version.ServerVersion, err)
	}
	return err
}

func userGitCredentialPayload(m userGitCredentialResourceModel) map[string]interface{} {
	return map[string]interface{}{
		"name":              m.Name.ValueString(),
		"username":          m.Username.ValueString(),
		"password":          m.Password.ValueString(),
		"authorizationType": int(m.AuthorizationType.ValueInt64()),
	}
}

func (r *userGitCredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan userGitCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	userID := int(plan.UserID.ValueInt64())

	var result struct {
		GitCredential struct {
			ID int `json:"id"`
		} `json:"gitCredential"`
	}
	if err := doJSON(ctx, r.client, http.MethodPost, fmt.Sprintf("%s/users/%d/gitcredentials", r.client.Endpoint, userID), userGitCredentialPayload(plan), &result); err != nil {
		resp.Diagnostics.AddError("Failed to create user git credential", fmt.Errorf("failed to create user git credential: %w", explainUserGitCredentialRemoved(ctx, r.client, err)).Error())
		return
	}

	credentialID := result.GitCredential.ID
	plan.ID = types.StringValue(fmt.Sprintf("%d:%d", userID, credentialID))
	plan.CredentialID = types.Int64Value(int64(credentialID))
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)...)

	r.read(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// read refreshes m from Portainer; false when the credential is gone or the
// ID cannot be parsed (the latter with an error).
func (r *userGitCredentialResource) read(ctx context.Context, m *userGitCredentialResourceModel, diags *diag.Diagnostics) bool {
	userID, credentialID, err := parseUserGitCredentialID(m.ID.ValueString())
	if err != nil {
		diags.AddError("Invalid user git credential ID", err.Error())
		return false
	}

	var result gitCredentialAPI
	if err := doJSON(ctx, r.client, http.MethodGet, fmt.Sprintf("%s/users/%d/gitcredentials/%d", r.client.Endpoint, userID, credentialID), nil, &result); err != nil {
		if isAPINotFound(err) {
			return false
		}
		diags.AddError("Failed to read user git credential", fmt.Errorf("failed to read user git credential: %w", err).Error())
		return false
	}

	m.UserID = types.Int64Value(int64(userID))
	m.CredentialID = types.Int64Value(int64(credentialID))
	m.Name = types.StringValue(result.Name)
	m.Username = types.StringValue(result.Username)
	m.AuthorizationType = types.Int64Value(int64(result.AuthorizationType))
	return true
}

func (r *userGitCredentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state userGitCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.read(ctx, &state, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *userGitCredentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state userGitCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	userID, credentialID, err := parseUserGitCredentialID(plan.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid user git credential ID", err.Error())
		return
	}
	if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/users/%d/gitcredentials/%d", r.client.Endpoint, userID, credentialID), userGitCredentialPayload(plan), nil); err != nil {
		resp.Diagnostics.AddError("Failed to update user git credential", fmt.Errorf("failed to update user git credential: %w", explainUserGitCredentialRemoved(ctx, r.client, err)).Error())
		return
	}

	if !r.read(ctx, &plan, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *userGitCredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state userGitCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	userID, credentialID, err := parseUserGitCredentialID(state.ID.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Invalid user git credential ID", err.Error())
		return
	}
	if err := doJSON(ctx, r.client, http.MethodDelete, fmt.Sprintf("%s/users/%d/gitcredentials/%d", r.client.Endpoint, userID, credentialID), nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete user git credential", fmt.Errorf("failed to delete user git credential: %w", err).Error())
	}
}

// ImportState takes "<user_id>:<credential_id>", as the SDKv2 importer did.
func (r *userGitCredentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	parts := strings.SplitN(req.ID, ":", 2)
	if len(parts) != 2 {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("unexpected format of ID (%q), expected <user_id>:<credential_id>", req.ID))
		return
	}
	userID, err := strconv.Atoi(parts[0])
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("invalid user ID: %s", err))
		return
	}
	credentialID, err := strconv.Atoi(parts[1])
	if err != nil {
		resp.Diagnostics.AddError("Invalid import ID", fmt.Sprintf("invalid credential ID: %s", err))
		return
	}
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("user_id"), int64(userID))...)
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), fmt.Sprintf("%d:%d", userID, credentialID))...)
}

func parseUserGitCredentialID(id string) (int, int, error) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected format of ID (%q), expected <user_id>:<credential_id>", id)
	}
	userID, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid user ID in composite ID: %w", err)
	}
	credentialID, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid credential ID in composite ID: %w", err)
	}
	return userID, credentialID, nil
}
