package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// addonRepairResource reissues the credential an installed addon
// authenticates to Portainer with. It is a one-shot action, so it has no read
// or update: the resource records that a repair ran, and `triggers` is how
// another one is asked for.
var (
	_ resource.Resource              = (*addonRepairResource)(nil)
	_ resource.ResourceWithConfigure = (*addonRepairResource)(nil)
)

func newAddonRepairResource() resource.Resource { return &addonRepairResource{} }

type addonRepairResource struct{ client *APIClient }

type addonRepairModel struct {
	ID                     types.String `tfsdk:"id"`
	AddonID                types.String `tfsdk:"addon_id"`
	Triggers               types.Map    `tfsdk:"triggers"`
	Enabled                types.Bool   `tfsdk:"enabled"`
	ChartVersion           types.String `tfsdk:"chart_version"`
	LifecycleStatus        types.String `tfsdk:"lifecycle_status"`
	LifecycleStatusMessage types.String `tfsdk:"lifecycle_status_message"`
}

func (r *addonRepairResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_addon_repair"
}

func (r *addonRepairResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	keepString := []planmodifier.String{stringplanmodifier.UseStateForUnknown()}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the resource.", Computed: true, PlanModifiers: keepString},
			"addon_id": schema.StringAttribute{
				Required:      true,
				Description:   "Catalog identifier of the addon whose credential is repaired, for example `portal-template`. Repair the addon when its `health_status` reports `credential-invalid`.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
			},
			"triggers": schema.MapAttribute{
				Optional:      true,
				ElementType:   types.StringType,
				Description:   "Arbitrary values that force another repair when they change. A repair is a one-shot action, so without a trigger the resource runs once and then stays put.",
				PlanModifiers: []planmodifier.Map{addonTriggersRequiresReplace{}},
			},
			"enabled": schema.BoolAttribute{
				Computed:      true,
				Description:   "Whether the addon is enabled, as reported after the repair.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"chart_version":            schema.StringAttribute{Computed: true, Description: "Chart version of the addon, as reported after the repair.", PlanModifiers: keepString},
			"lifecycle_status":         schema.StringAttribute{Computed: true, Description: "Helm release lifecycle state of the addon, as reported after the repair.", PlanModifiers: keepString},
			"lifecycle_status_message": schema.StringAttribute{Computed: true, Description: "Explanation of a failed lifecycle operation, as reported after the repair.", PlanModifiers: keepString},
		},
	}
}

func (r *addonRepairResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *addonRepairResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var m addonRepairModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	addonRepairRun(ctx, r.client, &m, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// addonRepairRun repairs the addon credential; it is the operation of both
// the resource and the portainer_addon_repair action.
func addonRepairRun(ctx context.Context, client *APIClient, m *addonRepairModel, diags *diag.Diagnostics) {
	addonID := m.AddonID.ValueString()

	var result struct {
		ID                     string `json:"id"`
		Enabled                bool   `json:"enabled"`
		ChartVersion           string `json:"chartVersion"`
		LifecycleStatus        string `json:"lifecycleStatus"`
		LifecycleStatusMessage string `json:"lifecycleStatusMessage"`
	}
	endpoint := addonURL(client, addonID) + "/repair"
	if err := doJSON(ctx, client, http.MethodPost, endpoint, nil, &result); err != nil {
		addonDiagError(diags, fmt.Errorf("failed to repair the credential of addon %s: %w", addonID, err))
		return
	}

	m.Enabled = types.BoolValue(result.Enabled)
	m.ChartVersion = types.StringValue(result.ChartVersion)
	m.LifecycleStatus = types.StringValue(result.LifecycleStatus)
	m.LifecycleStatusMessage = types.StringValue(result.LifecycleStatusMessage)
	m.ID = types.StringValue(fmt.Sprintf("%s-repair-%d", addonID, makeTimestamp()))
}

// Read keeps the state: the repair is recorded, not read back.
func (r *addonRepairResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}

// Update only records a switch between an unset and an empty `triggers`,
// which SDKv2 did not tell apart; every real change replaces the resource.
func (r *addonRepairResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	var m addonRepairModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &m)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

func (r *addonRepairResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
