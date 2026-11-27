package internal

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/resourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                     = (*backupResource)(nil)
	_ resource.ResourceWithConfigure        = (*backupResource)(nil)
	_ resource.ResourceWithConfigValidators = (*backupResource)(nil)
)

func newBackupResource() resource.Resource { return &backupResource{} }

type backupResource struct{ client *APIClient }

type backupResourceModel struct {
	ID                types.String `tfsdk:"id"`
	Password          types.String `tfsdk:"password"`
	PasswordWO        types.String `tfsdk:"password_wo"`
	PasswordWOVersion types.Int64  `tfsdk:"password_wo_version"`
	OutputPath        types.String `tfsdk:"output_path"`
}

func (r *backupResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_backup"
}

func (r *backupResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"password": schema.StringAttribute{
				Optional:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Sensitive password used to encrypt the Portainer backup archive. Stored in state as a sensitive value; use `password_wo` to keep it out of state.",
			},
			"password_wo":         gsbWriteOnlySecretAttribute("password", "password_wo_version", "password used to encrypt the backup archive"),
			"password_wo_version": gsbWriteOnlyVersionAttribute("password_wo", true),
			"output_path": schema.StringAttribute{
				Required:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Path on local disk where the backup .tar.gz should be saved",
			},
		},
	}
}

func (r *backupResource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(path.MatchRoot("password"), path.MatchRoot("password_wo")),
	}
}

func (r *backupResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *backupResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan backupResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	backupRun(ctx, r.client, &plan, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// backupRun downloads the backup archive to output_path; it is the operation
// of both the resource and the portainer_backup action.
func backupRun(ctx context.Context, client *APIClient, plan *backupResourceModel, config tfsdk.Config, diags *diag.Diagnostics) {
	password, _ := gsbTwinSecret(ctx, config, plan.Password, path.Root("password_wo"), plan.PasswordWOVersion, diags)
	if diags.HasError() {
		return
	}

	body := map[string]interface{}{
		"password": password,
	}

	httpResp, err := client.DoRequest(ctx, "POST", "/backup", nil, body)
	if err != nil {
		diags.AddError("Failed to create backup", fmt.Sprintf("failed to create backup: %s", err))
		return
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		data, _ := io.ReadAll(httpResp.Body)
		diags.AddError("Failed to create backup", fmt.Sprintf("failed to create backup: %s", string(data)))
		return
	}

	f, err := os.Create(filepath.Clean(plan.OutputPath.ValueString()))
	if err != nil {
		diags.AddError("Failed to create backup", fmt.Sprintf("failed to create file at output_path: %s", err))
		return
	}
	defer f.Close()

	if _, err := io.Copy(f, httpResp.Body); err != nil {
		diags.AddError("Failed to create backup", fmt.Sprintf("failed to write backup file: %s", err))
		return
	}

	plan.ID = types.StringValue(strconv.FormatInt(makeTimestamp(), 10))
	plan.PasswordWO = types.StringNull()
}

// Read keeps the state: the archive on disk is not tracked.
func (r *backupResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {}

// Update is never called: every attribute requires replacement.
func (r *backupResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Backups cannot be updated", "Every attribute of portainer_backup forces a new resource.")
}

func (r *backupResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// makeTimestamp is the unique part of the IDs of one-shot resources.
func makeTimestamp() int64 {
	return int64(os.Getpid()) + int64(os.Getppid())
}
