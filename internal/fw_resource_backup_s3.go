package internal

import (
	"context"
	"fmt"
	"net/http"
	"strings"

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
	_ resource.Resource                     = (*backupS3Resource)(nil)
	_ resource.ResourceWithConfigure        = (*backupS3Resource)(nil)
	_ resource.ResourceWithConfigValidators = (*backupS3Resource)(nil)
)

const backupS3ID = "portainer_backup_s3"

func newBackupS3Resource() resource.Resource { return &backupS3Resource{} }

type backupS3Resource struct{ client *APIClient }

type backupS3ResourceModel struct {
	ID                       types.String `tfsdk:"id"`
	AccessKeyID              types.String `tfsdk:"access_key_id"`
	SecretAccessKey          types.String `tfsdk:"secret_access_key"`
	SecretAccessKeyWO        types.String `tfsdk:"secret_access_key_wo"`
	SecretAccessKeyWOVersion types.Int64  `tfsdk:"secret_access_key_wo_version"`
	BucketName               types.String `tfsdk:"bucket_name"`
	Region                   types.String `tfsdk:"region"`
	S3CompatibleHost         types.String `tfsdk:"s3_compatible_host"`
	Password                 types.String `tfsdk:"password"`
	PasswordWO               types.String `tfsdk:"password_wo"`
	PasswordWOVersion        types.Int64  `tfsdk:"password_wo_version"`
	CronRule                 types.String `tfsdk:"cron_rule"`
	LastRunFailed            types.Bool   `tfsdk:"last_run_failed"`
	LastRunTimestamp         types.String `tfsdk:"last_run_timestamp"`
}

func (r *backupS3Resource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_backup_s3"
}

func (r *backupS3Resource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"access_key_id": schema.StringAttribute{
				Required:      true,
				Sensitive:     true,
				PlanModifiers: replace,
				Description:   "Sensitive S3 access key identifier used to upload the Portainer backup archive.",
			},
			"secret_access_key": schema.StringAttribute{
				Optional:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{gsbRequiresReplaceString()},
				Description:   "Sensitive S3 secret access key paired with `access_key_id`. Stored in state as a sensitive value; use `secret_access_key_wo` to keep it out of state.",
			},
			"secret_access_key_wo":         gsbWriteOnlySecretAttribute("secret_access_key", "secret_access_key_wo_version", "S3 secret access key"),
			"secret_access_key_wo_version": gsbWriteOnlyVersionAttribute("secret_access_key_wo", true),
			"bucket_name": schema.StringAttribute{
				Required:      true,
				PlanModifiers: replace,
				Description:   "Name of the S3 bucket where the Portainer backup archive will be stored.",
			},
			"region": schema.StringAttribute{
				Required:      true,
				PlanModifiers: replace,
				Description:   "AWS or S3-compatible region of the destination bucket.",
			},
			"s3_compatible_host": schema.StringAttribute{
				Required:      true,
				PlanModifiers: replace,
				Description:   "Endpoint URL of the S3 or S3-compatible service used for the backup upload.",
			},
			"password": schema.StringAttribute{
				Optional:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{gsbRequiresReplaceString()},
				Description:   "Sensitive password used to encrypt the Portainer backup archive before upload. Stored in state as a sensitive value; use `password_wo` to keep it out of state.",
			},
			"password_wo":         gsbWriteOnlySecretAttribute("password", "password_wo_version", "password used to encrypt the backup archive"),
			"password_wo_version": gsbWriteOnlyVersionAttribute("password_wo", true),
			"cron_rule": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{gsbRequiresReplaceString()},
				Description:   "Optional cron expression that schedules recurring S3 backups in Portainer.",
			},
			"last_run_failed": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the most recent scheduled S3 backup failed.",
			},
			"last_run_timestamp": schema.StringAttribute{
				Computed:    true,
				Description: "UTC timestamp of the most recent scheduled S3 backup, empty when none has run.",
			},
		},
	}
}

func (r *backupS3Resource) ConfigValidators(_ context.Context) []resource.ConfigValidator {
	return []resource.ConfigValidator{
		resourcevalidator.ExactlyOneOf(path.MatchRoot("secret_access_key"), path.MatchRoot("secret_access_key_wo")),
		resourcevalidator.ExactlyOneOf(path.MatchRoot("password"), path.MatchRoot("password_wo")),
	}
}

func (r *backupS3Resource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *backupS3Resource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan backupS3ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	backupS3Run(ctx, r.client, &plan, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	gsbMarkFrameworkState(ctx, resp.Private, &resp.Diagnostics)
}

// backupS3Run uploads a backup to S3; it is the operation of both the
// resource and the portainer_backup_s3 action.
func backupS3Run(ctx context.Context, client *APIClient, plan *backupS3ResourceModel, config tfsdk.Config, diags *diag.Diagnostics) {
	secretAccessKey, _ := gsbTwinSecret(ctx, config, plan.SecretAccessKey, path.Root("secret_access_key_wo"), plan.SecretAccessKeyWOVersion, diags)
	password, _ := gsbTwinSecret(ctx, config, plan.Password, path.Root("password_wo"), plan.PasswordWOVersion, diags)
	if diags.HasError() {
		return
	}

	body := map[string]interface{}{
		"accessKeyID":      plan.AccessKeyID.ValueString(),
		"secretAccessKey":  secretAccessKey,
		"bucketName":       plan.BucketName.ValueString(),
		"region":           plan.Region.ValueString(),
		"s3CompatibleHost": plan.S3CompatibleHost.ValueString(),
		"password":         password,
	}
	if v := plan.CronRule.ValueString(); v != "" {
		body["cronRule"] = v
	}

	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/backup/s3/execute", body, nil); err != nil {
		diags.AddError("Failed to execute S3 backup", fmt.Sprintf("failed to execute S3 backup: %s", err))
		return
	}

	plan.ID = types.StringValue(backupS3ID)
	plan.LastRunFailed = types.BoolNull()
	plan.LastRunTimestamp = types.StringNull()
}

// isRedacted reports a secret the API no longer returns: empty, or masked
// with asterisks.
func isRedacted(s string) bool {
	return strings.Trim(s, "*") == ""
}

func (r *backupS3Resource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state backupS3ResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	fw := gsbFrameworkState(ctx, req.Private)

	var result struct {
		AccessKeyID      string `json:"accessKeyID"`
		SecretAccessKey  string `json:"secretAccessKey"`
		BucketName       string `json:"bucketName"`
		Region           string `json:"region"`
		S3CompatibleHost string `json:"s3CompatibleHost"`
		Password         string `json:"password"`
		CronRule         string `json:"cronRule"`
	}
	if err := doJSON(ctx, r.client, http.MethodGet, r.client.Endpoint+"/backup/s3/settings", nil, &result); err != nil {
		resp.Diagnostics.AddError("Failed to read S3 backup settings", fmt.Sprintf("failed to read S3 backup settings: %s", err))
		return
	}

	state.AccessKeyID = types.StringValue(result.AccessKeyID)
	// A secret given write-only is not read back: storing it is what the
	// write-only twin avoids, and the configuration has no value to compare.
	// Portainer 3.0 redacts both secrets in this response; a redacted value
	// keeps the one in state rather than showing as a change on every plan.
	if state.SecretAccessKeyWOVersion.ValueInt64() == 0 && !isRedacted(result.SecretAccessKey) {
		state.SecretAccessKey = types.StringValue(result.SecretAccessKey)
	}
	state.BucketName = types.StringValue(result.BucketName)
	state.Region = types.StringValue(result.Region)
	state.S3CompatibleHost = types.StringValue(result.S3CompatibleHost)
	if state.PasswordWOVersion.ValueInt64() == 0 && !isRedacted(result.Password) {
		state.Password = types.StringValue(result.Password)
	}
	state.CronRule = gsbString(result.CronRule, state.CronRule, fw)

	// Informational only: an instance that has never run a scheduled backup
	// should not fail the read.
	var status struct {
		Failed       bool   `json:"Failed"`
		TimestampUTC string `json:"TimestampUTC"`
	}
	if err := doJSON(ctx, r.client, http.MethodGet, r.client.Endpoint+"/backup/s3/status", nil, &status); err == nil {
		state.LastRunFailed = types.BoolValue(status.Failed)
		state.LastRunTimestamp = types.StringValue(status.TimestampUTC)
	}

	state.ID = types.StringValue(backupS3ID)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	gsbMarkFrameworkState(ctx, resp.Private, &resp.Diagnostics)
}

// Update only stores an equivalent value (an empty string for a null one):
// every real change requires replacement.
func (r *backupS3Resource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan, state backupS3ResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.LastRunFailed = state.LastRunFailed
	plan.LastRunTimestamp = state.LastRunTimestamp
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	gsbMarkFrameworkState(ctx, resp.Private, &resp.Diagnostics)
}

// Delete only forgets the resource: the operation cannot be undone via the API.
func (r *backupS3Resource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
