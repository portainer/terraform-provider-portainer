package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/mapplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*backupAzureExecuteResource)(nil)
	_ resource.ResourceWithConfigure = (*backupAzureExecuteResource)(nil)
)

func newBackupAzureExecuteResource() resource.Resource { return &backupAzureExecuteResource{} }

type backupAzureExecuteResource struct{ client *APIClient }

type backupAzureExecuteModel struct {
	ID types.String `tfsdk:"id"`
	azureBlobModel
	azureBlobWriteOnlyModel
	Triggers types.Map `tfsdk:"triggers"`
}

// gsbTriggersAttribute is the "triggers" map of the one-shot backup runs.
func gsbTriggersAttribute() schema.MapAttribute {
	return schema.MapAttribute{
		Optional:      true,
		ElementType:   types.StringType,
		PlanModifiers: []planmodifier.Map{mapplanmodifier.RequiresReplace()},
		Description:   "Arbitrary values that force another backup run when they change. A backup is a one-shot action, so without a trigger the resource runs once and then stays put.",
	}
}

func (r *backupAzureExecuteResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_backup_azure_execute"
}

func (r *backupAzureExecuteResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	attrs := azureBlobAttributes(azureBlobDescriptions, azureBlobWriteOnlyWhat, true)
	attrs["id"] = schema.StringAttribute{
		Description:   "Identifier of the resource.",
		Computed:      true,
		PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
	}
	attrs["triggers"] = gsbTriggersAttribute()
	resp.Schema = schema.Schema{Attributes: attrs}
}

func (r *backupAzureExecuteResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *backupAzureExecuteResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan backupAzureExecuteModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	backupAzureExecuteRun(ctx, r.client, &plan, req.Config, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// backupAzureExecuteRun runs the Azure Blob backup; it is the operation of
// both the resource and the portainer_backup_azure_execute action.
func backupAzureExecuteRun(ctx context.Context, client *APIClient, plan *backupAzureExecuteModel, config tfsdk.Config, diags *diag.Diagnostics) {
	// The ad-hoc endpoint takes the same settings object as the scheduled one
	// but without a cron rule: it backs up once with the credentials given and
	// does not change the stored schedule.
	payload := azureBlobPayload(ctx, &plan.azureBlobModel, &plan.azureBlobWriteOnlyModel, &config, azureBlobSettingsRequiredKeys, azureBlobSettingsKeys, diags)
	if diags.HasError() {
		return
	}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/backup/azure/execute", payload, nil); err != nil {
		diags.AddError("Failed to execute the Azure Blob backup", fmt.Sprintf("failed to execute the Azure Blob backup: %s", err))
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("portainer-backup-azure-execute-%d", makeTimestamp()))
	plan.clearWriteOnly()
}

// Read keeps the state: a backup run is a one-shot action.
func (r *backupAzureExecuteResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}

// Update only stores a value equivalent to the one in state (an empty string
// for a null one): every real change requires replacement.
func (r *backupAzureExecuteResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var plan backupAzureExecuteModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.clearWriteOnly()
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *backupAzureExecuteResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
