package internal

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                     = (*restoreResource)(nil)
	_ resource.ResourceWithConfigure        = (*restoreResource)(nil)
	_ resource.ResourceWithConfigValidators = (*restoreResource)(nil)
)

func newRestoreResource() resource.Resource { return &restoreResource{} }

type restoreResource struct{ client *APIClient }

type restoreResourceModel struct {
	ID                         types.String `tfsdk:"id"`
	FileContentBase64          types.String `tfsdk:"file_content_base64"`
	FileContentBase64WO        types.String `tfsdk:"file_content_base64_wo"`
	FileContentBase64WOVersion types.Int64  `tfsdk:"file_content_base64_wo_version"`
	FileName                   types.String `tfsdk:"file_name"`
	Password                   types.String `tfsdk:"password"`
	PasswordWO                 types.String `tfsdk:"password_wo"`
	PasswordWOVersion          types.Int64  `tfsdk:"password_wo_version"`
	SetupTokenWO               types.String `tfsdk:"setup_token_wo"`
	SetupTokenWOVersion        types.Int64  `tfsdk:"setup_token_wo_version"`
}

func (r *restoreResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_restore"
}

func (r *restoreResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"file_content_base64": schema.StringAttribute{
				Optional:      true,
				Sensitive:     true,
				PlanModifiers: replace,
				Description:   "Base64-encoded content of the backup archive, typically `filebase64(\"portainer-backup.tar.gz\")`. A backup contains every credential Portainer holds, so it is stored in state as a sensitive value; use `file_content_base64_wo` to keep it out of state.",
			},
			"file_content_base64_wo":         gsbWriteOnlySecretAttribute("file_content_base64", "file_content_base64_wo_version", "base64-encoded backup archive"),
			"file_content_base64_wo_version": gsbWriteOnlyVersionAttribute("file_content_base64_wo", true),
			"file_name": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Default:       stringdefault.StaticString("portainer-backup.tar.gz"),
				PlanModifiers: replace,
				Description:   "Name reported to Portainer for the uploaded archive.",
			},
			"password": schema.StringAttribute{
				Optional:      true,
				Sensitive:     true,
				PlanModifiers: replace,
				Description:   "Password the backup was encrypted with. Leave unset for an unencrypted backup. Stored in state as a sensitive value; use `password_wo` to keep it out of state.",
			},
			"password_wo":         gsbWriteOnlySecretAttribute("password", "password_wo_version", "password the backup archive was encrypted with"),
			"password_wo_version": gsbWriteOnlyVersionAttribute("password_wo", true),
			// Write-only only: the token is a one-time bootstrap secret, so there
			// is no plaintext twin that would put it in state.
			"setup_token_wo": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				WriteOnly:   true,
				Validators:  []validator.String{stringvalidator.AlsoRequires(path.MatchRoot("setup_token_wo_version"))},
				Description: "Write-only value sent as the `X-Setup-Token` header (supports ephemeral values; never stored in Terraform state). Portainer requires it when the instance has not been initialised yet, which is the usual state for a restore. Requires Terraform 1.11 or later.",
			},
			"setup_token_wo_version": gsbWriteOnlyVersionAttribute("setup_token_wo", true),
		},
	}
}

func (r *restoreResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(path.MatchRoot("file_content_base64"), path.MatchRoot("file_content_base64_wo")),
	}
}

func (r *restoreResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *restoreResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan restoreResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	encoded, _ := gsbTwinSecret(ctx, req.Config, plan.FileContentBase64, path.Root("file_content_base64_wo"), plan.FileContentBase64WOVersion, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	content, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		resp.Diagnostics.AddError("Invalid backup archive", fmt.Sprintf("file_content_base64 is not valid base64: %s", err))
		return
	}
	if len(content) == 0 {
		resp.Diagnostics.AddError("Invalid backup archive", "file_content_base64 decodes to an empty archive")
		return
	}

	// FileContent is []byte on the Portainer side, so encoding/json re-encodes it
	// as base64 on the wire; the decode above is what validates the input.
	payload := map[string]interface{}{
		"FileContent": content,
		"FileName":    plan.FileName.ValueString(),
	}
	password, _ := gsbTwinSecret(ctx, req.Config, plan.Password, path.Root("password_wo"), plan.PasswordWOVersion, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if password != "" {
		payload["Password"] = password
	}

	// The setup token goes in a header rather than the body, so this call does
	// not go through doJSON.
	headers := map[string]string{}
	if plan.SetupTokenWOVersion.ValueInt64() != 0 {
		if setupToken := writeOnlyConfigString(ctx, req.Config, path.Root("setup_token_wo"), &resp.Diagnostics); setupToken != "" {
			headers["X-Setup-Token"] = setupToken
		}
	}
	if err := doJSONWithHeaders(ctx, r.client, http.MethodPost, r.client.Endpoint+"/restore", payload, nil, headers); err != nil {
		resp.Diagnostics.AddError("Failed to restore the Portainer backup", fmt.Sprintf("failed to restore the Portainer backup: %s", err))
		return
	}

	plan.ID = types.StringValue("portainer-restore-" + fmt.Sprint(makeTimestamp()))
	plan.FileContentBase64WO = types.StringNull()
	plan.PasswordWO = types.StringNull()
	plan.SetupTokenWO = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// Read keeps the state: a restore is a one-shot action.
func (r *restoreResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {}

// Update is never called: every attribute requires replacement.
func (r *restoreResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Restores cannot be updated", "Every attribute of portainer_restore forces a new resource.")
}

// Delete only forgets the restore, which cannot be undone.
func (r *restoreResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
