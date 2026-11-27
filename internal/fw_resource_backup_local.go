package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*backupLocalSettingsResource)(nil)
	_ resource.ResourceWithConfigure   = (*backupLocalSettingsResource)(nil)
	_ resource.ResourceWithImportState = (*backupLocalSettingsResource)(nil)
	_ resource.Resource                = (*backupLocalRunResource)(nil)
	_ resource.ResourceWithConfigure   = (*backupLocalRunResource)(nil)
)

const backupLocalSettingsID = "portainer-backup-local-settings"

// backupLocalStatus is GET /backup/local/status.
type backupLocalStatus struct {
	Failed       bool   `json:"failed"`
	TimestampUTC string `json:"timestampUTC"`
	Path         string `json:"path"`
}

func newBackupLocalSettingsResource() resource.Resource { return &backupLocalSettingsResource{} }

type backupLocalSettingsResource struct{ client *APIClient }

type backupLocalSettingsModel struct {
	ID                types.String `tfsdk:"id"`
	CronRule          types.String `tfsdk:"cron_rule"`
	RetentionDays     types.Int64  `tfsdk:"retention_days"`
	LastRunFailed     types.Bool   `tfsdk:"last_run_failed"`
	LastRunTimestamp  types.String `tfsdk:"last_run_timestamp"`
	LastRunPath       types.String `tfsdk:"last_run_path"`
	Password          types.String `tfsdk:"password"`
	PasswordWO        types.String `tfsdk:"password_wo"`
	PasswordWOVersion types.Int64  `tfsdk:"password_wo_version"`
}

func (r *backupLocalSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_backup_local_settings"
}

func (r *backupLocalSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"cron_rule": schema.StringAttribute{
				Optional:    true,
				Description: "Cron expression for the scheduled local backup (for example `0 2 * * *`). Leave unset to keep the schedule off and back up only on demand with `portainer_backup_local_run`.",
			},
			"retention_days": schema.Int64Attribute{
				Optional:    true,
				Validators:  []validator.Int64{int64validator.AtLeast(0)},
				Description: "How many days Portainer keeps local backup archives before pruning them. Zero keeps them indefinitely.",
			},
			"password": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Password the scheduled local backup archives are encrypted with. Leave unset for unencrypted archives. Required when Portainer runs in FIPS mode (Business Edition 3.1 or later), which refuses unencrypted backups. Stored in state as a sensitive value; use `password_wo` to keep it out of state.",
			},
			"password_wo":         gsbWriteOnlySecretAttribute("password", "password_wo_version", "password used to encrypt the local backup archives"),
			"password_wo_version": gsbWriteOnlyVersionAttribute("password_wo", false),
			"last_run_failed": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether the most recent scheduled local backup failed.",
			},
			"last_run_timestamp": schema.StringAttribute{
				Computed:    true,
				Description: "UTC timestamp of the most recent scheduled local backup, empty when none has run.",
			},
			"last_run_path": schema.StringAttribute{
				Computed:    true,
				Description: "Path on the Portainer host where the most recent local backup archive was written.",
			},
		},
	}
}

func (r *backupLocalSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *backupLocalSettingsResource) write(ctx context.Context, config tfsdk.Config, plan *backupLocalSettingsModel, diags *diag.Diagnostics) {
	payload := map[string]interface{}{
		"cronRule":      plan.CronRule.ValueString(),
		"retentionDays": int(plan.RetentionDays.ValueInt64()),
	}
	// The password is new in Business Edition 3.1 and only sent when
	// configured, so older servers never see the field.
	password, _ := gsbTwinSecret(ctx, config, plan.Password, path.Root("password_wo"), plan.PasswordWOVersion, diags)
	if diags.HasError() {
		return
	}
	if password != "" {
		payload["password"] = password
	}
	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/backup/local/settings", payload, nil); err != nil {
		diags.AddError("Failed to update the local backup settings", fmt.Sprintf("failed to update the local backup settings: %s", err))
		return
	}
	plan.ID = types.StringValue(backupLocalSettingsID)
	plan.PasswordWO = types.StringNull()

	// The configured values are what was just written; only the computed
	// status comes from the read. The password is never read back.
	read := *plan
	read.LastRunFailed, read.LastRunTimestamp, read.LastRunPath = types.BoolNull(), types.StringNull(), types.StringNull()
	r.read(ctx, &read, true, diags)
	plan.LastRunFailed, plan.LastRunTimestamp, plan.LastRunPath = read.LastRunFailed, read.LastRunTimestamp, read.LastRunPath
}

func (r *backupLocalSettingsResource) read(ctx context.Context, m *backupLocalSettingsModel, fw bool, diags *diag.Diagnostics) {
	var settings struct {
		CronRule      string `json:"cronRule"`
		RetentionDays int    `json:"retentionDays"`
	}
	if err := doJSON(ctx, r.client, http.MethodGet, r.client.Endpoint+"/backup/local/settings", nil, &settings); err != nil {
		diags.AddError("Failed to read the local backup settings", fmt.Sprintf("failed to read the local backup settings: %s", err))
		return
	}
	m.CronRule = gsbString(settings.CronRule, m.CronRule, fw)
	m.RetentionDays = gsbInt64(int64(settings.RetentionDays), m.RetentionDays, fw)

	// Informational only: an instance that has never run a scheduled backup
	// should not fail the read.
	var status backupLocalStatus
	if err := doJSON(ctx, r.client, http.MethodGet, r.client.Endpoint+"/backup/local/status", nil, &status); err == nil {
		m.LastRunFailed = types.BoolValue(status.Failed)
		m.LastRunTimestamp = types.StringValue(status.TimestampUTC)
		m.LastRunPath = types.StringValue(status.Path)
	}
}

func (r *backupLocalSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan backupLocalSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, req.Config, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	gsbMarkFrameworkState(ctx, resp.Private, &resp.Diagnostics)
}

func (r *backupLocalSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var state backupLocalSettingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.read(ctx, &state, gsbFrameworkState(ctx, req.Private), &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	gsbMarkFrameworkState(ctx, resp.Private, &resp.Diagnostics)
}

func (r *backupLocalSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan backupLocalSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.write(ctx, req.Config, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	gsbMarkFrameworkState(ctx, resp.Private, &resp.Diagnostics)
}

// Delete only forgets the settings: Portainer has no endpoint to clear them,
// so removing the resource leaves the schedule as configured.
func (r *backupLocalSettingsResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

// ImportState accepts any ID: there is one set of settings per instance.
func (r *backupLocalSettingsResource) ImportState(ctx context.Context, _ resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), backupLocalSettingsID)...)
}

func newBackupLocalRunResource() resource.Resource { return &backupLocalRunResource{} }

type backupLocalRunResource struct{ client *APIClient }

type backupLocalRunModel struct {
	ID        types.String `tfsdk:"id"`
	Triggers  types.Map    `tfsdk:"triggers"`
	Path      types.String `tfsdk:"path"`
	Failed    types.Bool   `tfsdk:"failed"`
	Timestamp types.String `tfsdk:"timestamp"`
}

func (r *backupLocalRunResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_backup_local_run"
}

func (r *backupLocalRunResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"triggers": gsbTriggersAttribute(),
			"path": schema.StringAttribute{
				Computed:    true,
				Description: "Path on the Portainer host where the archive was written, as reported after the run.",
			},
			"failed": schema.BoolAttribute{
				Computed:    true,
				Description: "Whether Portainer reports the run as failed.",
			},
			"timestamp": schema.StringAttribute{
				Computed:    true,
				Description: "UTC timestamp Portainer reports for the run.",
			},
		},
	}
}

func (r *backupLocalRunResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *backupLocalRunResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan backupLocalRunModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	backupLocalRun(ctx, r.client, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// backupLocalRun runs the local backup and records its status; it is the
// operation of both the resource and the portainer_backup_local_run action.
func backupLocalRun(ctx context.Context, client *APIClient, plan *backupLocalRunModel, diags *diag.Diagnostics) {
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/backup/local/run", nil, nil); err != nil {
		diags.AddError("Failed to trigger a local backup", fmt.Sprintf("failed to trigger a local backup: %s", err))
		return
	}

	// The run endpoint answers without a body, so the outcome comes from the
	// status endpoint. It is informational: a backup that ran is not undone by
	// a status read that fails.
	plan.Path, plan.Failed, plan.Timestamp = types.StringNull(), types.BoolNull(), types.StringNull()
	var status backupLocalStatus
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/backup/local/status", nil, &status); err == nil {
		plan.Path = types.StringValue(status.Path)
		plan.Failed = types.BoolValue(status.Failed)
		plan.Timestamp = types.StringValue(status.TimestampUTC)
	}

	plan.ID = types.StringValue(fmt.Sprintf("portainer-backup-local-run-%d", makeTimestamp()))
}

// Read keeps the state: a backup run is a one-shot action.
func (r *backupLocalRunResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}

// Update is never called: every attribute requires replacement.
func (r *backupLocalRunResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Backup runs cannot be updated", "Every attribute of portainer_backup_local_run forces a new resource.")
}

func (r *backupLocalRunResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
