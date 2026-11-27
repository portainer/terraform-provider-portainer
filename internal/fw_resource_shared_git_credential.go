package internal

import (
	"context"
	"errors"
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
	_ resource.Resource                = (*sharedGitCredentialResource)(nil)
	_ resource.ResourceWithConfigure   = (*sharedGitCredentialResource)(nil)
	_ resource.ResourceWithImportState = (*sharedGitCredentialResource)(nil)
)

func newSharedGitCredentialResource() resource.Resource { return &sharedGitCredentialResource{} }

type sharedGitCredentialResource struct{ client *APIClient }

type sharedGitCredentialResourceModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Username          types.String `tfsdk:"username"`
	Password          types.String `tfsdk:"password"`
	AuthorizationType types.Int64  `tfsdk:"authorization_type"`
	UserID            types.Int64  `tfsdk:"user_id"`
	URLPattern        types.String `tfsdk:"url_pattern"`
	// importedPattern is the urlPattern the last read returned, for the
	// first Read after an import.
	importedPattern string `tfsdk:"-"`
}

// gitCredentialAPI is a git credential as Portainer returns it, shared or
// owned by a user.
type gitCredentialAPI struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	Username          string `json:"username"`
	AuthorizationType int    `json:"authorizationType"`
	UserID            int    `json:"userId"`
}

func (r *sharedGitCredentialResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_shared_git_credential"
}

func (r *sharedGitCredentialResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the shared git credential.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the shared git credential",
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
			"url_pattern": schema.StringAttribute{
				Optional: true,
				Description: "Repository URL pattern the credential is offered for (e.g. `github.com/portainer/*`). " +
					"Business Edition 2.45 or later, and required by Business Edition 3.1 and later. Left out, it is not sent and Portainer keeps its current value.",
				Validators: []validator.String{stringvalidator.LengthAtLeast(1)},
			},
			"user_id": schema.Int64Attribute{
				Computed:      true,
				Description:   "User ID of the credential owner",
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
		},
	}
}

func (r *sharedGitCredentialResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func sharedGitCredentialPayload(m sharedGitCredentialResourceModel) map[string]interface{} {
	payload := map[string]interface{}{
		"name":              m.Name.ValueString(),
		"username":          m.Username.ValueString(),
		"password":          m.Password.ValueString(),
		"authorizationType": int(m.AuthorizationType.ValueInt64()),
	}
	// urlPattern is only sent when configured, so the request of a
	// configuration without it stays as it was.
	if !m.URLPattern.IsNull() && !m.URLPattern.IsUnknown() {
		payload["urlPattern"] = m.URLPattern.ValueString()
	}
	return payload
}

// sharedGitCredentialCheckPattern reports a configured url_pattern that the
// server did not store: older releases ignore the field.
func sharedGitCredentialCheckPattern(planned types.String, m sharedGitCredentialResourceModel, diags *diag.Diagnostics) {
	if !planned.IsNull() && !m.URLPattern.Equal(planned) {
		diags.AddError("Failed to apply url_pattern", fmt.Sprintf("Portainer did not store url_pattern %q for shared git credential %s; it requires %s",
			planned.ValueString(), m.ID.ValueString(), requiresBE245))
	}
}

// sharedGitCredentialExplain explains the 400 of Business Edition 3.1 and
// later for a credential without url_pattern, which they require.
func sharedGitCredentialExplain(err error, m sharedGitCredentialResourceModel) error {
	var se *apiStatusError
	if m.URLPattern.IsNull() && errors.As(err, &se) && se.StatusCode == http.StatusBadRequest &&
		strings.Contains(strings.ToLower(se.Body), "url pattern") {
		return fmt.Errorf("%w\n\nPortainer Business Edition 3.1 and later require url_pattern on shared git credentials; "+
			"set url_pattern (e.g. \"github.com/my-org/*\") on this resource", err)
	}
	return err
}

func (r *sharedGitCredentialResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan sharedGitCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var result struct {
		GitCredential struct {
			ID int `json:"id"`
		} `json:"gitCredential"`
	}
	if err := doJSON(ctx, r.client, http.MethodPost, fmt.Sprintf("%s/cloud/gitcredentials", r.client.Endpoint), sharedGitCredentialPayload(plan), &result); err != nil {
		resp.Diagnostics.AddError("Failed to create shared git credential", fmt.Errorf("failed to create shared git credential: %w", sharedGitCredentialExplain(err, plan)).Error())
		return
	}

	plan.ID = types.StringValue(strconv.Itoa(result.GitCredential.ID))
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), plan.ID)...)
	planned := plan.URLPattern
	r.read(ctx, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	sharedGitCredentialCheckPattern(planned, plan, &resp.Diagnostics)
}

// read refreshes m from Portainer; false when the credential is gone.
func (r *sharedGitCredentialResource) read(ctx context.Context, m *sharedGitCredentialResourceModel, diags *diag.Diagnostics) bool {
	var result struct {
		gitCredentialAPI
		URLPattern string `json:"urlPattern"`
	}
	if err := doJSON(ctx, r.client, http.MethodGet, fmt.Sprintf("%s/cloud/gitcredentials/%s", r.client.Endpoint, m.ID.ValueString()), nil, &result); err != nil {
		if isAPINotFound(err) {
			return false
		}
		diags.AddError("Failed to read shared git credential", fmt.Errorf("failed to read shared git credential: %w", err).Error())
		return false
	}
	m.Name = types.StringValue(result.Name)
	m.Username = types.StringValue(result.Username)
	m.AuthorizationType = types.Int64Value(int64(result.AuthorizationType))
	m.UserID = types.Int64Value(int64(result.UserID))
	// Read back only while managed, so a credential without it plans no change.
	if !m.URLPattern.IsNull() {
		m.URLPattern = types.StringValue(result.URLPattern)
	}
	m.importedPattern = result.URLPattern
	return true
}

// sharedGitCredentialImportedKey marks the first Read after an import, which
// adopts the url_pattern the credential has (Business Edition 3.1 requires
// one, so an imported credential always has it there).
const sharedGitCredentialImportedKey = "imported"

func (r *sharedGitCredentialResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state sharedGitCredentialResourceModel
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
	if v, d := req.Private.GetKey(ctx, sharedGitCredentialImportedKey); !d.HasError() && len(v) > 0 {
		if state.URLPattern.IsNull() && state.importedPattern != "" {
			state.URLPattern = types.StringValue(state.importedPattern)
		}
		resp.Diagnostics.Append(resp.Private.SetKey(ctx, sharedGitCredentialImportedKey, nil)...)
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *sharedGitCredentialResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state sharedGitCredentialResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.ID = state.ID

	if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/cloud/gitcredentials/%s", r.client.Endpoint, plan.ID.ValueString()), sharedGitCredentialPayload(plan), nil); err != nil {
		resp.Diagnostics.AddError("Failed to update shared git credential", fmt.Errorf("failed to update shared git credential: %w", sharedGitCredentialExplain(err, plan)).Error())
		return
	}

	planned := plan.URLPattern
	if !r.read(ctx, &plan, &resp.Diagnostics) {
		if !resp.Diagnostics.HasError() {
			resp.State.RemoveResource(ctx)
		}
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	sharedGitCredentialCheckPattern(planned, plan, &resp.Diagnostics)
}

func (r *sharedGitCredentialResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state sharedGitCredentialResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := doJSON(ctx, r.client, http.MethodDelete, fmt.Sprintf("%s/cloud/gitcredentials/%s", r.client.Endpoint, state.ID.ValueString()), nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete shared git credential", fmt.Errorf("failed to delete shared git credential: %w", err).Error())
	}
}

func (r *sharedGitCredentialResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
	resp.Diagnostics.Append(resp.Private.SetKey(ctx, sharedGitCredentialImportedKey, []byte(`true`))...)
}
