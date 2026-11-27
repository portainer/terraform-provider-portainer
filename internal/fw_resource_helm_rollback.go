package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource              = (*helmRollbackResource)(nil)
	_ resource.ResourceWithConfigure = (*helmRollbackResource)(nil)
)

func newHelmRollbackResource() resource.Resource { return &helmRollbackResource{} }

type helmRollbackResource struct{ client *APIClient }

type helmRollbackResourceModel struct {
	ID          types.String `tfsdk:"id"`
	EndpointID  types.Int64  `tfsdk:"endpoint_id"`
	ReleaseName types.String `tfsdk:"release_name"`
	Namespace   types.String `tfsdk:"namespace"`
	Revision    types.Int64  `tfsdk:"revision"`
	Wait        types.Bool   `tfsdk:"wait"`
	WaitForJobs types.Bool   `tfsdk:"wait_for_jobs"`
	Recreate    types.Bool   `tfsdk:"recreate"`
	Force       types.Bool   `tfsdk:"force"`
	Timeout     types.Int64  `tfsdk:"timeout"`
}

func (r *helmRollbackResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_helm_rollback"
}

func (r *helmRollbackResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replaceInt := []planmodifier.Int64{int64planmodifier.RequiresReplace()}
	replaceStr := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	replaceBool := []planmodifier.Bool{boolplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the rollback.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"endpoint_id": schema.Int64Attribute{
				Required:      true,
				Description:   "Environment (Endpoint) identifier",
				PlanModifiers: replaceInt,
			},
			"release_name": schema.StringAttribute{
				Required:      true,
				Description:   "Name of the Helm release to rollback",
				PlanModifiers: replaceStr,
			},
			"namespace": schema.StringAttribute{
				Optional:      true,
				Description:   "Kubernetes namespace of the release",
				PlanModifiers: replaceStr,
			},
			"revision": schema.Int64Attribute{
				Optional:      true,
				Description:   "Revision number to rollback to (defaults to previous revision if not specified)",
				PlanModifiers: replaceInt,
			},
			"wait": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(false),
				Description:   "Wait for resources to be ready",
				PlanModifiers: replaceBool,
			},
			"wait_for_jobs": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(false),
				Description:   "Wait for jobs to complete before marking the release as successful",
				PlanModifiers: replaceBool,
			},
			"recreate": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(true),
				Description:   "Perform pods restart for the resource if applicable",
				PlanModifiers: replaceBool,
			},
			"force": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Default:       booldefault.StaticBool(false),
				Description:   "Force resource update through delete/recreate if needed",
				PlanModifiers: replaceBool,
			},
			"timeout": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Default:       int64default.StaticInt64(300),
				Description:   "Time to wait for any individual Kubernetes operation in seconds",
				PlanModifiers: replaceInt,
			},
		},
	}
}

func (r *helmRollbackResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *helmRollbackResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	var plan helmRollbackResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	helmRollbackRun(ctx, r.client, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// helmRollbackRun rolls the release back; it is the operation of both the
// resource and the portainer_helm_rollback action.
func helmRollbackRun(ctx context.Context, client *APIClient, plan *helmRollbackResourceModel, diags *diag.Diagnostics) {
	endpointID := int(plan.EndpointID.ValueInt64())
	releaseName := plan.ReleaseName.ValueString()

	p := fmt.Sprintf("/endpoints/%d/kubernetes/helm/%s/rollback", endpointID, releaseName)

	// Build query parameters; like SDKv2's GetOk, zero values are left out
	// (so recreate=false is never sent).
	queryParams := ""
	separator := "?"
	add := func(kv string) {
		queryParams += separator + kv
		separator = "&"
	}
	if v := plan.Namespace.ValueString(); v != "" {
		add("namespace=" + v)
	}
	if v := plan.Revision.ValueInt64(); v != 0 {
		add("revision=" + strconv.FormatInt(v, 10))
	}
	if plan.Wait.ValueBool() {
		add("wait=true")
	}
	if plan.WaitForJobs.ValueBool() {
		add("waitForJobs=true")
	}
	if plan.Recreate.ValueBool() {
		add("recreate=" + strconv.FormatBool(true))
	}
	if plan.Force.ValueBool() {
		add("force=true")
	}
	if v := plan.Timeout.ValueInt64(); v != 0 {
		add("timeout=" + strconv.FormatInt(v, 10))
	}

	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+p+queryParams, nil, nil); err != nil {
		diags.AddError("Failed to rollback Helm release", fmt.Errorf("failed to rollback Helm release %s: %w", releaseName, err).Error())
		return
	}

	plan.ID = types.StringValue(fmt.Sprintf("helm-rollback-%d-%s-%d", endpointID, releaseName, makeTimestamp()))
}

// Read is a no-op: a rollback is a one-shot action.
func (r *helmRollbackResource) Read(_ context.Context, _ resource.ReadRequest, _ *resource.ReadResponse) {
}

// Update is never called: every argument requires replacement.
func (r *helmRollbackResource) Update(_ context.Context, _ resource.UpdateRequest, resp *resource.UpdateResponse) {
	resp.Diagnostics.AddError("Helm rollbacks cannot be updated", "Every argument of portainer_helm_rollback forces a new resource.")
}

// Delete only forgets the rollback.
func (r *helmRollbackResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}
