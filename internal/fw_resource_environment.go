package internal

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64default"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*environmentResource)(nil)
	_ resource.ResourceWithConfigure   = (*environmentResource)(nil)
	_ resource.ResourceWithImportState = (*environmentResource)(nil)
	_ resource.ResourceWithModifyPlan  = (*environmentResource)(nil)
	_ resource.ResourceWithIdentity    = (*environmentResource)(nil)
)

func newEnvironmentResource() resource.Resource { return &environmentResource{} }

type environmentResource struct {
	fwIDIdentity
	client *APIClient
}

type environmentResourceModel struct {
	ID                     types.String   `tfsdk:"id"`
	Name                   types.String   `tfsdk:"name"`
	DeleteClusterOnDestroy types.Bool     `tfsdk:"delete_cluster_on_destroy"`
	EnvironmentAddress     types.String   `tfsdk:"environment_address"`
	PublicIP               types.String   `tfsdk:"public_ip"`
	Type                   types.Int64    `tfsdk:"type"`
	GroupID                types.Int64    `tfsdk:"group_id"`
	TagIDs                 types.List     `tfsdk:"tag_ids"`
	TLSEnabled             types.Bool     `tfsdk:"tls_enabled"`
	TLSSkipVerify          types.Bool     `tfsdk:"tls_skip_verify"`
	TLSCACert              types.String   `tfsdk:"tls_ca_cert"`
	TLSCert                types.String   `tfsdk:"tls_cert"`
	TLSKey                 types.String   `tfsdk:"tls_key"`
	TLSKeyWO               types.String   `tfsdk:"tls_key_wo"`
	TLSKeyWOVersion        types.Int64    `tfsdk:"tls_key_wo_version"`
	TLSSkipClientVerify    types.Bool     `tfsdk:"tls_skip_client_verify"`
	EdgeKey                types.String   `tfsdk:"edge_key"`
	EdgeID                 types.String   `tfsdk:"edge_id"`
	UserAccessPolicies     types.Map      `tfsdk:"user_access_policies"`
	TeamAccessPolicies     types.Map      `tfsdk:"team_access_policies"`
	ContainerEngine        types.String   `tfsdk:"container_engine"`
	Timeouts               timeouts.Value `tfsdk:"timeouts"`

	AzureApplicationID              types.String `tfsdk:"azure_application_id"`
	AzureTenantID                   types.String `tfsdk:"azure_tenant_id"`
	AzureAuthenticationKey          types.String `tfsdk:"azure_authentication_key"`
	AzureAuthenticationKeyWO        types.String `tfsdk:"azure_authentication_key_wo"`
	AzureAuthenticationKeyWOVersion types.Int64  `tfsdk:"azure_authentication_key_wo_version"`
	EdgeCheckinInterval             types.Int64  `tfsdk:"edge_checkin_interval"`
	EdgeTunnelServerAddress         types.String `tfsdk:"edge_tunnel_server_address"`
	EdgeAsyncMode                   types.Bool   `tfsdk:"edge_async_mode"`
	EdgePingInterval                types.Int64  `tfsdk:"edge_ping_interval"`
	EdgeSnapshotInterval            types.Int64  `tfsdk:"edge_snapshot_interval"`
	EdgeCommandInterval             types.Int64  `tfsdk:"edge_command_interval"`
	CustomTemplateID                types.Int64  `tfsdk:"custom_template_id"`
	CustomTemplateContent           types.String `tfsdk:"custom_template_content"`
	CustomTemplateStackName         types.String `tfsdk:"custom_template_stack_name"`
}

func (r *environmentResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_environment"
}

func (r *environmentResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the environment.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the Portainer environment (endpoint) as displayed in the UI.",
			},
			"delete_cluster_on_destroy": schema.BoolAttribute{
				Optional: true,
				Computed: true,
				Default:  booldefault.StaticBool(false),
				Description: "Portainer 3.0+: on destroy, send `deleteCluster=true` with DELETE /endpoints/{id}. For a KubeSolo environment installed by Portainer this uninstalls KubeSolo from the machine before the environment is removed; " +
					"Portainer does that in the background (HTTP 202) and the provider waits until the environment is gone, within the delete timeout. " +
					"Portainer answers 409 while KubeSolo is still being installed. Older releases ignore the parameter and only remove the environment. Changing it never touches the environment itself.",
			},
			"environment_address": schema.StringAttribute{
				Required:      true,
				Description:   "Connection URL of the environment (e.g. tcp://host:port for Docker, https://host for Kubernetes/Agent). Used as the endpoint URL Portainer connects to.",
				PlanModifiers: []planmodifier.String{environmentAddressSuppress{}},
			},
			"public_ip": schema.StringAttribute{
				Optional: true,
				// Portainer fills the public URL in from the environment address
				// when none is given.
				Computed:      true,
				Description:   "Public IP/URL used by Portainer for Published Ports (maps to PublicURL). When unset, Portainer uses the environment address.",
				PlanModifiers: []planmodifier.String{environmentPublicIPPlan{}},
			},
			"type": schema.Int64Attribute{
				Required:      true,
				Description:   "Environment type: 1 = Docker, 2 = Agent, 3 = Azure, 4 = Edge Agent, 5 = Kubernetes, 6 = Kubernetes via agent, 7 = Kubernetes Edge Agent",
				Validators:    []validator.Int64{int64validator.Between(1, 7)},
				PlanModifiers: []planmodifier.Int64{environmentTypeSuppress{}},
			},
			"group_id": schema.Int64Attribute{
				Optional:    true,
				Computed:    true,
				Default:     int64default.StaticInt64(1),
				Description: "ID of the Portainer endpoint group. Default is 1 (Unassigned).",
			},
			"tag_ids": schema.ListAttribute{
				Optional:    true,
				ElementType: types.Int64Type,
				Description: "List of tag IDs to assign to the environment.",
			},
			"tls_enabled": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether TLS is enabled for the connection to the environment.",
			},
			"tls_skip_verify": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether to skip server TLS certificate verification when connecting to the environment.",
			},
			"tls_ca_cert": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "PEM-encoded TLS CA certificate used to verify the environment's server certificate. Stored in state as a sensitive value.",
			},
			"tls_cert": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "PEM-encoded TLS client certificate presented by Portainer when connecting to the environment. Stored in state as a sensitive value.",
			},
			"tls_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "PEM-encoded TLS client private key paired with tls_cert. Stored in state as a sensitive value; use `tls_key_wo` to keep it out of state.",
				Validators:  []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("tls_key_wo"))},
			},
			"tls_key_wo": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				WriteOnly:   true,
				Description: "Write-only PEM-encoded TLS client private key (supports ephemeral values; never stored in Terraform state). Uploaded on create, whenever another TLS attribute changes, and when `tls_key_wo_version` changes. Requires Terraform 1.11 or later.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("tls_key")),
					stringvalidator.AlsoRequires(path.MatchRoot("tls_key_wo_version")),
				},
			},
			"tls_key_wo_version": schema.Int64Attribute{
				Optional:    true,
				Description: "Version of `tls_key_wo`. Increment it to upload a new key.",
			},
			"tls_skip_client_verify": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(true),
				Description: "Whether to skip client TLS certificate verification when Portainer connects to the environment.",
			},
			"edge_key": schema.StringAttribute{
				Computed:      true,
				Sensitive:     true,
				Description:   "Edge key generated by Portainer for type 4/7 (Edge Agent) environments. Used by the Edge Agent to register with Portainer.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"edge_id": schema.StringAttribute{
				Computed:      true,
				Description:   "Edge identifier (UUID) for type 4/7 (Edge Agent) environments, to pass to the Edge Agent as PORTAINER_EDGE_ID. Returned by Portainer when the EnforceEdgeID setting is enabled, otherwise generated by the provider at create time. Empty for all other environment types.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"user_access_policies": schema.MapAttribute{
				Optional:    true,
				ElementType: types.Int64Type,
				Description: "Map of user IDs to role IDs (e.g. userID -> roleID)",
			},
			"team_access_policies": schema.MapAttribute{
				Optional:    true,
				ElementType: types.Int64Type,
				Description: "Map of team IDs to role IDs (e.g. teamID -> roleID)",
			},
			"container_engine": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Default:       stringdefault.StaticString("docker"),
				Description:   "Container engine used by the environment(endpoint). Value must be one of: 'docker' or 'podman'",
				Validators:    []validator.String{stringvalidator.OneOf("docker", "podman")},
				PlanModifiers: []planmodifier.String{environmentContainerEngineSuppress{}},
			},
			"azure_application_id": schema.StringAttribute{
				Optional:    true,
				Description: "Azure (type 3) environments: Azure application (client) ID Portainer authenticates with. Sent on create and when changed; Portainer never returns it.",
			},
			"azure_tenant_id": schema.StringAttribute{
				Optional:    true,
				Description: "Azure (type 3) environments: Azure tenant ID. Sent on create and when changed; Portainer never returns it.",
			},
			"azure_authentication_key": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				Description: "Azure (type 3) environments: Azure authentication key (client secret). Stored in state as a sensitive value; use `azure_authentication_key_wo` to keep it out of state.",
				Validators:  []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("azure_authentication_key_wo"))},
			},
			"azure_authentication_key_wo": schema.StringAttribute{
				Optional:    true,
				Sensitive:   true,
				WriteOnly:   true,
				Description: "Write-only Azure authentication key (supports ephemeral values; never stored in Terraform state). Sent on create and when `azure_authentication_key_wo_version` changes. Requires Terraform 1.11 or later.",
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("azure_authentication_key")),
					stringvalidator.AlsoRequires(path.MatchRoot("azure_authentication_key_wo_version")),
				},
			},
			"azure_authentication_key_wo_version": schema.Int64Attribute{
				Optional:    true,
				Description: "Version of `azure_authentication_key_wo`. Increment it to send a new key.",
			},
			"edge_checkin_interval": schema.Int64Attribute{
				Optional: true,
				// Portainer always reports one, 0 when the environment uses the
				// default from the settings.
				Computed:      true,
				Description:   "Edge Agent (type 4/7) environments: check-in (poll) interval of the agent in seconds. 0 uses the default interval from the Portainer settings. When unset, the provider leaves the current value alone and reports it.",
				Validators:    []validator.Int64{int64validator.AtLeast(0)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"edge_tunnel_server_address": schema.StringAttribute{
				Optional: true,
				Description: "Edge Agent (type 4/7) environments: address (host:port) of the Portainer tunnel server the agent connects to, baked into the edge key at creation. Unset uses the address from the Portainer settings. " +
					"Changing it replaces the environment, as Portainer never regenerates the edge key.",
			},
			"edge_async_mode": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				Description:   "Portainer Business Edition, Edge Agent (type 4/7) environments: create the environment for an agent running in async mode. Portainer cannot change the mode later, so changing a configured value replaces the environment. When unset, Portainer's value is reported.",
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
			},
			"edge_ping_interval": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Portainer Business Edition, Edge Agent environments in async mode: ping interval of the agent in seconds. 0 uses the default from the Portainer settings. When unset, the provider leaves the current value alone and reports it.",
				Validators:    []validator.Int64{int64validator.AtLeast(0)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"edge_snapshot_interval": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Portainer Business Edition, Edge Agent environments in async mode: snapshot interval of the agent in seconds. 0 uses the default from the Portainer settings. When unset, the provider leaves the current value alone and reports it.",
				Validators:    []validator.Int64{int64validator.AtLeast(0)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"edge_command_interval": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				Description:   "Portainer Business Edition, Edge Agent environments in async mode: command interval of the agent in seconds. 0 uses the default from the Portainer settings. When unset, the provider leaves the current value alone and reports it.",
				Validators:    []validator.Int64{int64validator.AtLeast(0)},
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
			},
			"custom_template_id": schema.Int64Attribute{
				Optional: true,
				Description: "Portainer Business Edition: ID of a custom template Portainer deploys on the environment right after creating it (the KaaS / Kubernetes onboarding option). " +
					"Create-time only: changing it later neither redeploys nor replaces anything.",
				Validators: []validator.Int64{
					int64validator.AtLeast(1),
					int64validator.ConflictsWith(path.MatchRoot("custom_template_content")),
				},
			},
			"custom_template_content": schema.StringAttribute{
				Optional:    true,
				Description: "Portainer Business Edition: custom template content Portainer deploys on the environment right after creating it, instead of `custom_template_id`. Create-time only.",
			},
			"custom_template_stack_name": schema.StringAttribute{
				Optional:    true,
				Description: "Portainer Business Edition: stack name of the custom template deployment (`custom_template_id` / `custom_template_content`). Create-time only.",
			},
		},
		Blocks: map[string]schema.Block{
			// Only used when delete_cluster_on_destroy waits for Portainer to
			// uninstall KubeSolo and remove the environment in the background.
			"timeouts": timeouts.Block(ctx, timeouts.Opts{Delete: true}),
		},
	}
}

func (r *environmentResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (r *environmentResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	fwImportIDOrIdentity(ctx, req, resp)
}

// ---------------------------------------------------------------------------
// Plan modifiers replacing the SDKv2 DiffSuppressFuncs. Terraform accepts a
// planned value that differs from the configuration when it is the prior
// state value, which is how a suppressed difference is expressed here.

// environmentAddressSuppress: state written before Read kept the scheme holds
// the address the way Portainer stores it for agents, without the scheme.
type environmentAddressSuppress struct{}

func (environmentAddressSuppress) Description(context.Context) string {
	return "Keeps a scheme-less address from the state when only the scheme differs."
}

func (m environmentAddressSuppress) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (environmentAddressSuppress) PlanModifyString(_ context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	old, new := req.StateValue.ValueString(), req.ConfigValue.ValueString()
	if old != "" && old == trimURLScheme(old) && old == trimURLScheme(new) {
		resp.PlanValue = req.StateValue
	}
}

// environmentTypeSuppress: Portainer converts the type once the agent
// connects: an Edge Agent (4) on Kubernetes becomes a Kubernetes Edge Agent
// (7), and an Agent (2) on Kubernetes becomes Kubernetes via agent (6).
type environmentTypeSuppress struct{}

func (environmentTypeSuppress) Description(context.Context) string {
	return "Ignores the type conversions Portainer makes once an agent connects (4 to 7, 2 to 6)."
}

func (m environmentTypeSuppress) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (environmentTypeSuppress) PlanModifyInt64(_ context.Context, req planmodifier.Int64Request, resp *planmodifier.Int64Response) {
	if req.StateValue.IsNull() || req.ConfigValue.IsNull() || req.ConfigValue.IsUnknown() {
		return
	}
	old, new := req.StateValue.ValueInt64(), req.ConfigValue.ValueInt64()
	if (old == 7 && new == 4) || (old == 6 && new == 2) {
		resp.PlanValue = req.StateValue
	}
}

// environmentContainerEngineSuppress: container_engine is not relevant unless
// type == 4. State written before container_engine existed holds no value;
// Portainer treats an empty engine as Docker, so this must not plan a
// replacement. Never on create, where suppressing the default would send no
// engine at all.
type environmentContainerEngineSuppress struct{}

func (environmentContainerEngineSuppress) Description(context.Context) string {
	return "Ignores the container engine of environments other than Docker/Podman Edge Agents, and an empty engine against docker."
}

func (m environmentContainerEngineSuppress) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (environmentContainerEngineSuppress) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if req.StateValue.IsNull() || req.State.Raw.IsNull() || resp.PlanValue.IsUnknown() {
		return
	}
	var envType types.Int64
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("type"), &envType)...)
	if envType.IsUnknown() {
		return
	}
	if envType.ValueInt64() != 4 || normalizeContainerEngine(req.StateValue.ValueString()) == normalizeContainerEngine(resp.PlanValue.ValueString()) {
		resp.PlanValue = req.StateValue
	}
}

// environmentPublicIPPlan: an omitted public_ip keeps the state value, except
// when a directly-connected environment gets a new address, which Update also
// makes its public URL.
type environmentPublicIPPlan struct{}

func (environmentPublicIPPlan) Description(context.Context) string {
	return "Keeps the public URL from the state unless the configuration or the address changes it."
}

func (m environmentPublicIPPlan) MarkdownDescription(ctx context.Context) string {
	return m.Description(ctx)
}

func (environmentPublicIPPlan) PlanModifyString(ctx context.Context, req planmodifier.StringRequest, resp *planmodifier.StringResponse) {
	if !req.ConfigValue.IsNull() || req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var plan, state environmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if plan.EnvironmentAddress.IsUnknown() || plan.Type.IsUnknown() {
		return
	}
	isEdge := environmentIsEdge(plan.Type.ValueInt64())
	if !isEdge && plan.EnvironmentAddress.ValueString() != state.EnvironmentAddress.ValueString() {
		return
	}
	// Every other change leaves the public URL as Portainer holds it.
	resp.PlanValue = req.StateValue
}

// ModifyPlan forces replacement when an Edge Agent environment's address
// really changes.
//
// Portainer bakes the address into the edge key when the environment is
// created and never regenerates it, and the update endpoint does not accept a
// URL for edge types at all. Moving such an environment therefore requires a
// new environment with a new edge key and a redeployed agent. A difference in
// the scheme alone is not a move: it comes from Portainer's own host-only
// normalisation of endpoint.URL (issue #136) for state written by older
// provider versions.
func (r *environmentResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.State.Raw.IsNull() || req.Plan.Raw.IsNull() {
		return
	}
	var plan, state environmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() || plan.Type.IsUnknown() {
		return
	}
	envType := plan.Type.ValueInt64()
	if !environmentIsEdge(envType) {
		return
	}
	if plan.EnvironmentAddress.IsUnknown() || plan.ContainerEngine.IsUnknown() {
		return
	}
	if trimURLScheme(state.EnvironmentAddress.ValueString()) != trimURLScheme(plan.EnvironmentAddress.ValueString()) {
		resp.RequiresReplace = append(resp.RequiresReplace, path.Root("environment_address"))
		return
	}
	// Only a Docker Edge Agent has an engine, and an empty engine is Docker.
	if envType == 4 && normalizeContainerEngine(state.ContainerEngine.ValueString()) != normalizeContainerEngine(plan.ContainerEngine.ValueString()) {
		resp.RequiresReplace = append(resp.RequiresReplace, path.Root("container_engine"))
	}
	// The update endpoint has no async mode; unset is the standard mode.
	if !plan.EdgeAsyncMode.IsUnknown() && plan.EdgeAsyncMode.ValueBool() != state.EdgeAsyncMode.ValueBool() {
		resp.RequiresReplace = append(resp.RequiresReplace, path.Root("edge_async_mode"))
	}
	if environmentTunnelMoved(&plan, &state) {
		resp.RequiresReplace = append(resp.RequiresReplace, path.Root("edge_tunnel_server_address"))
	}
}

// environmentTunnelMoved reports whether a configured tunnel server address
// differs from the one the edge key was created with. Removing it from the
// configuration changes nothing in Portainer. State without one (written
// before the attribute existed, or imported) is compared with the address in
// the edge key, so adopting the address already in use is no move.
func environmentTunnelMoved(plan, state *environmentResourceModel) bool {
	if plan.EdgeTunnelServerAddress.IsNull() || plan.EdgeTunnelServerAddress.IsUnknown() {
		return false
	}
	want := plan.EdgeTunnelServerAddress.ValueString()
	if !state.EdgeTunnelServerAddress.IsNull() {
		return want != state.EdgeTunnelServerAddress.ValueString()
	}
	return want != edgeTunnelFromKey(state.EdgeKey.ValueString())
}

// ---------------------------------------------------------------------------
// CRUD

func (r *environmentResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan environmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	wo := writeOnlyConfigString(ctx, req.Config, path.Root("tls_key_wo"), &resp.Diagnostics)
	azureWO := writeOnlyConfigString(ctx, req.Config, path.Root("azure_authentication_key_wo"), &resp.Diagnostics)
	var configPublicIP types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("public_ip"), &configPublicIP)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if err := environmentCheckBusinessEdition(ctx, r.client, environmentBusinessEditionAttrs(&plan, nil)); err != nil {
		resp.Diagnostics.AddError("Failed to create environment", err.Error())
		return
	}

	m := plan
	m.EdgeID = types.StringValue("")
	m.EdgeKey = types.StringValue("")
	if m.PublicIP.IsUnknown() {
		m.PublicIP = types.StringValue("")
	}
	op := environmentOp{client: r.client, prior: nil, wo: wo, azureWO: azureWO, configPublicIP: configPublicIP}
	name := plan.Name.ValueString()

	existingID, err := findExistingEnvironmentByName(ctx, r.client, name)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create environment", fmt.Errorf("failed to check for existing environment: %w", err).Error())
		return
	}
	if existingID != 0 {
		m.ID = types.StringValue(strconv.Itoa(existingID))
		if !op.update(ctx, &m, &resp.Diagnostics) {
			return
		}
		environmentEnsureEdgeID(&m)
		r.finish(ctx, &m, &plan, &resp.State, &resp.Diagnostics)
		return
	}

	envType := plan.Type.ValueInt64()
	endpointCreationType := envType
	if envType == 6 {
		endpointCreationType = 2
	}

	// POST /endpoints is multipart/form-data. Empty string fields are left out,
	// as the generated SDK this replaced did.
	var fields [][2]string
	addField := func(name, value string) {
		if value != "" {
			fields = append(fields, [2]string{name, value})
		}
	}
	var files []multipartFile
	addField("Name", name)
	addField("EndpointCreationType", strconv.FormatInt(endpointCreationType, 10))

	// Edge Agent creation (type 4) is ambiguous on the Portainer side: without a
	// container engine hint Portainer provisions the endpoint as a Kubernetes
	// Edge Agent (type 7) instead of a Docker or Podman Edge Agent (type 4).
	// Sending ContainerEngine as podman or docker pins it to the edge type the
	// user asked for (issues #140 / #93). type 7 intentionally leaves it unset
	// so Portainer keeps the Kubernetes edge type.
	if envType == 4 {
		addField("ContainerEngine", normalizeContainerEngine(plan.ContainerEngine.ValueString()))
	}

	addField("URL", strings.TrimSpace(plan.EnvironmentAddress.ValueString()))
	addField("GroupID", strconv.FormatInt(plan.GroupID.ValueInt64(), 10))

	if v := configPublicIP.ValueString(); v != "" {
		addField("PublicURL", v)
	}

	// Edge Agent environments (type 4, 7) do not support TagIds in the
	// multipart Create request. Tags are applied via a follow-up Update call.
	isEdgeAgent := environmentIsEdge(envType)
	// Portainer accepts TagIds only as a single part holding a JSON array
	// ("[1,2,3]"); repeated TagIds parts, one per id, are rejected with
	// "Invalid TagIds parameter".
	tags := int64ListToInts(ctx, plan.TagIDs, &resp.Diagnostics)
	if len(tags) > 0 && !isEdgeAgent {
		tagIDs := make([]string, 0, len(tags))
		for _, id := range tags {
			tagIDs = append(tagIDs, strconv.Itoa(id))
		}
		addField("TagIds", "["+strings.Join(tagIDs, ",")+"]")
	}

	// Portainer refuses TLS outright for Edge Agent environments ("TLS is not
	// supported for Edge Agent environments" as soon as payload.TLS is true),
	// and the agent reaches Portainer through the edge tunnel anyway, so the
	// TLS fields are not sent for them - the same exclusion Update applies.
	if !isEdgeAgent {
		tlsEnabled := plan.TLSEnabled.ValueBool()
		addField("TLS", strconv.FormatBool(tlsEnabled))
		tlsSkipVerify := plan.TLSSkipVerify.ValueBool()
		addField("TLSSkipVerify", strconv.FormatBool(tlsSkipVerify))
		tlsSkipClientVerify := plan.TLSSkipClientVerify.ValueBool()
		addField("TLSSkipClientVerify", strconv.FormatBool(tlsSkipClientVerify))

		// Portainer reads the CA only when server verification is on and the
		// client certificate and key only when client verification is on.
		if tlsEnabled && !tlsSkipVerify {
			if v := plan.TLSCACert.ValueString(); v != "" {
				files = append(files, multipartFile{Field: "TLSCACertFile", Filename: "ca.pem", Content: v})
			}
		}
		if tlsEnabled && !tlsSkipClientVerify {
			if v := plan.TLSCert.ValueString(); v != "" {
				files = append(files, multipartFile{Field: "TLSCertFile", Filename: "cert.pem", Content: v})
			}
			if key := environmentTLSKeyValue(&m, wo); key != "" {
				files = append(files, multipartFile{Field: "TLSKeyFile", Filename: "key.pem", Content: key})
			}
		}
	}
	environmentCreateExtraFields(&m, azureWO, addField)
	if resp.Diagnostics.HasError() {
		return
	}

	var created portainerEndpoint
	if err := doMultipart(ctx, r.client, http.MethodPost, r.client.Endpoint+"/endpoints", fields, files, &created); err != nil {
		resp.Diagnostics.AddError("Failed to create environment", fmt.Errorf("failed to create environment: %w", err).Error())
		return
	}

	m.ID = types.StringValue(strconv.FormatInt(created.ID, 10))
	if created.EdgeKey != "" {
		m.EdgeKey = types.StringValue(created.EdgeKey)
	}
	if created.EdgeID != "" {
		m.EdgeID = types.StringValue(created.EdgeID)
	}
	// Save the ID right away, so a failure below leaves the environment in
	// the state rather than orphaned.
	resp.Diagnostics.Append(resp.State.SetAttribute(ctx, path.Root("id"), m.ID)...)

	// For edge agents, tags and the public URL are not honoured by the
	// multipart Create request and must be applied via a follow-up Update.
	if isEdgeAgent && (len(tags) > 0 || configPublicIP.ValueString() != "") {
		if !op.update(ctx, &m, &resp.Diagnostics) {
			return
		}
	}
	if len(environmentInt64Map(m.UserAccessPolicies)) > 0 {
		if !op.update(ctx, &m, &resp.Diagnostics) {
			return
		}
	}
	if len(environmentInt64Map(m.TeamAccessPolicies)) > 0 {
		if !op.update(ctx, &m, &resp.Diagnostics) {
			return
		}
	}

	found, err := environmentRead(ctx, r.client, &m)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read environment", err.Error())
		return
	}
	if !found {
		resp.Diagnostics.AddError("Failed to read environment", fmt.Sprintf("environment %s disappeared right after it was created", m.ID.ValueString()))
		return
	}
	environmentEnsureEdgeID(&m)
	r.finish(ctx, &m, &plan, &resp.State, &resp.Diagnostics)
}

// finish writes the result of Create or Update: what Read saw, except for
// the values the plan already fixed, which Terraform requires back unchanged
// (a difference shows on the next refresh, as with SDKv2).
func (r *environmentResource) finish(ctx context.Context, m, plan *environmentResourceModel, state *tfsdk.State, diags *diag.Diagnostics) {
	out := *m
	keepString := func(dst *types.String, p types.String) {
		if !p.IsUnknown() {
			*dst = p
		}
	}
	keepInt := func(dst *types.Int64, p types.Int64) {
		if !p.IsUnknown() {
			*dst = p
		}
	}
	keepBool := func(dst *types.Bool, p types.Bool) {
		if !p.IsUnknown() {
			*dst = p
		}
	}
	keepString(&out.Name, plan.Name)
	keepString(&out.EnvironmentAddress, plan.EnvironmentAddress)
	keepString(&out.PublicIP, plan.PublicIP)
	keepString(&out.ContainerEngine, plan.ContainerEngine)
	keepString(&out.TLSCACert, plan.TLSCACert)
	keepString(&out.TLSCert, plan.TLSCert)
	keepString(&out.TLSKey, plan.TLSKey)
	keepString(&out.EdgeKey, plan.EdgeKey)
	keepString(&out.EdgeID, plan.EdgeID)
	keepInt(&out.Type, plan.Type)
	keepInt(&out.GroupID, plan.GroupID)
	keepInt(&out.TLSKeyWOVersion, plan.TLSKeyWOVersion)
	keepBool(&out.DeleteClusterOnDestroy, plan.DeleteClusterOnDestroy)
	keepBool(&out.TLSEnabled, plan.TLSEnabled)
	keepBool(&out.TLSSkipVerify, plan.TLSSkipVerify)
	keepBool(&out.TLSSkipClientVerify, plan.TLSSkipClientVerify)
	if !plan.TagIDs.IsUnknown() {
		out.TagIDs = plan.TagIDs
	}
	out.UserAccessPolicies = plan.UserAccessPolicies
	out.TeamAccessPolicies = plan.TeamAccessPolicies
	out.TLSKeyWO = types.StringNull()
	keepString(&out.AzureApplicationID, plan.AzureApplicationID)
	keepString(&out.AzureTenantID, plan.AzureTenantID)
	keepString(&out.AzureAuthenticationKey, plan.AzureAuthenticationKey)
	keepInt(&out.AzureAuthenticationKeyWOVersion, plan.AzureAuthenticationKeyWOVersion)
	out.AzureAuthenticationKeyWO = types.StringNull()
	keepInt(&out.EdgeCheckinInterval, plan.EdgeCheckinInterval)
	keepString(&out.EdgeTunnelServerAddress, plan.EdgeTunnelServerAddress)
	keepBool(&out.EdgeAsyncMode, plan.EdgeAsyncMode)
	keepInt(&out.EdgePingInterval, plan.EdgePingInterval)
	keepInt(&out.EdgeSnapshotInterval, plan.EdgeSnapshotInterval)
	keepInt(&out.EdgeCommandInterval, plan.EdgeCommandInterval)
	keepInt(&out.CustomTemplateID, plan.CustomTemplateID)
	keepString(&out.CustomTemplateContent, plan.CustomTemplateContent)
	keepString(&out.CustomTemplateStackName, plan.CustomTemplateStackName)
	out.Timeouts = plan.Timeouts
	diags.Append(state.Set(ctx, &out)...)
}

func (r *environmentResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	fwSetIDIdentity(ctx, req.State, resp.Identity, &resp.Diagnostics)
	var state environmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	found, err := environmentRead(ctx, r.client, &state)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read environment", err.Error())
		return
	}
	if !found {
		resp.State.RemoveResource(ctx)
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

func (r *environmentResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan, state environmentResourceModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	wo := writeOnlyConfigString(ctx, req.Config, path.Root("tls_key_wo"), &resp.Diagnostics)
	azureWO := writeOnlyConfigString(ctx, req.Config, path.Root("azure_authentication_key_wo"), &resp.Diagnostics)
	var configPublicIP types.String
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("public_ip"), &configPublicIP)...)
	if resp.Diagnostics.HasError() {
		return
	}

	m := plan
	m.ID = state.ID
	m.EdgeKey = state.EdgeKey
	m.EdgeID = state.EdgeID
	if m.PublicIP.IsUnknown() {
		m.PublicIP = state.PublicIP
	}

	// delete_cluster_on_destroy only changes what destroy does, and the
	// create-time settings cannot be sent to an existing environment; when they
	// are the only change there is nothing to send.
	if environmentOnlyLocalChanged(&plan, &state) {
		found, err := environmentRead(ctx, r.client, &m)
		if err != nil {
			resp.Diagnostics.AddError("Failed to read environment", err.Error())
			return
		}
		if !found {
			resp.State.RemoveResource(ctx)
			return
		}
		r.finish(ctx, &m, &plan, &resp.State, &resp.Diagnostics)
		return
	}

	if err := environmentCheckBusinessEdition(ctx, r.client, environmentBusinessEditionAttrs(&plan, &state)); err != nil {
		resp.Diagnostics.AddError("Failed to update environment", err.Error())
		return
	}
	op := environmentOp{client: r.client, prior: &state, wo: wo, azureWO: azureWO, configPublicIP: configPublicIP}
	if !op.update(ctx, &m, &resp.Diagnostics) {
		return
	}
	r.finish(ctx, &m, &plan, &resp.State, &resp.Diagnostics)
}

func (r *environmentResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state environmentResourceModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, _ := strconv.ParseInt(state.ID.ValueString(), 10, 64)

	deleteCluster := state.DeleteClusterOnDestroy.ValueBool()
	url := fmt.Sprintf("%s/endpoints/%d", r.client.Endpoint, id)
	if deleteCluster {
		url += "?deleteCluster=true"
	}
	if err := doJSON(ctx, r.client, http.MethodDelete, url, nil, nil); err != nil {
		if isAPINotFound(err) {
			return
		}
		var se *apiStatusError
		if deleteCluster && errors.As(err, &se) && se.StatusCode == http.StatusConflict {
			resp.Diagnostics.AddError("Failed to delete environment", fmt.Errorf("failed to delete environment: KubeSolo is still being installed on the environment's machine; retry once the installation has finished: %w", err).Error())
			return
		}
		resp.Diagnostics.AddError("Failed to delete environment", fmt.Errorf("failed to delete environment: %w", err).Error())
		return
	}

	if deleteCluster {
		// Portainer 3.0 answers 202 for a KubeSolo environment: it uninstalls
		// KubeSolo and only then removes the environment, in the background.
		// After a 204 (every other case) the first check already sees 404.
		timeout, diags := state.Timeouts.Delete(ctx, 20*time.Minute)
		resp.Diagnostics.Append(diags...)
		if resp.Diagnostics.HasError() {
			return
		}
		if err := waitEnvironmentRemoved(ctx, r.client, id, timeout); err != nil {
			resp.Diagnostics.AddError("Failed to delete environment", err.Error())
		}
	}
}

// ---------------------------------------------------------------------------
// Shared logic

func environmentIsEdge(envType int64) bool { return envType == 4 || envType == 7 }

// environmentOnlyLocalChanged reports whether the plan differs from the
// state only in attributes PUT /endpoints/{id} cannot carry:
// delete_cluster_on_destroy and the create-time settings (a real change of
// edge_async_mode or edge_tunnel_server_address is a replacement instead).
func environmentOnlyLocalChanged(plan, state *environmentResourceModel) bool {
	if plan.DeleteClusterOnDestroy.Equal(state.DeleteClusterOnDestroy) &&
		plan.EdgeAsyncMode.Equal(state.EdgeAsyncMode) &&
		plan.EdgeTunnelServerAddress.Equal(state.EdgeTunnelServerAddress) &&
		plan.CustomTemplateID.Equal(state.CustomTemplateID) &&
		plan.CustomTemplateContent.Equal(state.CustomTemplateContent) &&
		plan.CustomTemplateStackName.Equal(state.CustomTemplateStackName) {
		return false
	}
	p, s := *plan, *state
	p.DeleteClusterOnDestroy, s.DeleteClusterOnDestroy = types.BoolNull(), types.BoolNull()
	return !environmentConfigChanged(&p, &s)
}

// environmentConfigChanged reports whether any configurable attribute differs,
// the way SDKv2's HasChange compared them (zero values equal null).
func environmentConfigChanged(p, s *environmentResourceModel) bool {
	return environmentStringChanged(p.Name, s.Name) ||
		environmentStringChanged(p.EnvironmentAddress, s.EnvironmentAddress) ||
		(!p.PublicIP.IsUnknown() && environmentStringChanged(p.PublicIP, s.PublicIP)) ||
		environmentInt64Changed(p.Type, s.Type) ||
		environmentInt64Changed(p.GroupID, s.GroupID) ||
		environmentListChanged(p.TagIDs, s.TagIDs) ||
		environmentBoolChanged(p.TLSEnabled, s.TLSEnabled) ||
		environmentBoolChanged(p.TLSSkipVerify, s.TLSSkipVerify) ||
		environmentBoolChanged(p.TLSSkipClientVerify, s.TLSSkipClientVerify) ||
		environmentStringChanged(p.TLSCACert, s.TLSCACert) ||
		environmentStringChanged(p.TLSCert, s.TLSCert) ||
		environmentStringChanged(p.TLSKey, s.TLSKey) ||
		environmentInt64Changed(p.TLSKeyWOVersion, s.TLSKeyWOVersion) ||
		environmentMapChanged(p.UserAccessPolicies, s.UserAccessPolicies) ||
		environmentMapChanged(p.TeamAccessPolicies, s.TeamAccessPolicies) ||
		environmentStringChanged(p.ContainerEngine, s.ContainerEngine) ||
		environmentBoolChanged(p.DeleteClusterOnDestroy, s.DeleteClusterOnDestroy) ||
		environmentStringChanged(p.AzureApplicationID, s.AzureApplicationID) ||
		environmentStringChanged(p.AzureTenantID, s.AzureTenantID) ||
		environmentStringChanged(p.AzureAuthenticationKey, s.AzureAuthenticationKey) ||
		environmentInt64Changed(p.AzureAuthenticationKeyWOVersion, s.AzureAuthenticationKeyWOVersion) ||
		environmentInt64Changed(p.EdgeCheckinInterval, s.EdgeCheckinInterval) ||
		environmentEdgeIntervalsChanged(p, s)
}

func environmentEdgeIntervalsChanged(p, s *environmentResourceModel) bool {
	return environmentInt64Changed(p.EdgePingInterval, s.EdgePingInterval) ||
		environmentInt64Changed(p.EdgeSnapshotInterval, s.EdgeSnapshotInterval) ||
		environmentInt64Changed(p.EdgeCommandInterval, s.EdgeCommandInterval)
}

func environmentStringChanged(a, b types.String) bool { return a.ValueString() != b.ValueString() }
func environmentInt64Changed(a, b types.Int64) bool   { return a.ValueInt64() != b.ValueInt64() }
func environmentBoolChanged(a, b types.Bool) bool     { return a.ValueBool() != b.ValueBool() }

func environmentListChanged(a, b types.List) bool {
	ai, bi := environmentInt64s(a), environmentInt64s(b)
	if len(ai) != len(bi) {
		return true
	}
	for i := range ai {
		if ai[i] != bi[i] {
			return true
		}
	}
	return false
}

func environmentMapChanged(a, b types.Map) bool {
	am, bm := environmentInt64Map(a), environmentInt64Map(b)
	if len(am) != len(bm) {
		return true
	}
	for k, v := range am {
		if w, ok := bm[k]; !ok || w != v {
			return true
		}
	}
	return false
}

func environmentInt64s(l types.List) []int64 {
	if l.IsNull() || l.IsUnknown() {
		return nil
	}
	var out []int64
	for _, e := range l.Elements() {
		if n, ok := e.(types.Int64); ok {
			out = append(out, n.ValueInt64())
		}
	}
	return out
}

func environmentInt64Map(m types.Map) map[string]int64 {
	if m.IsNull() || m.IsUnknown() {
		return nil
	}
	out := map[string]int64{}
	for k, e := range m.Elements() {
		if n, ok := e.(types.Int64); ok {
			out[k] = n.ValueInt64()
		}
	}
	return out
}

// environmentTLSKeyValue returns the client key from tls_key, or from the
// write-only tls_key_wo while tls_key_wo_version is set.
func environmentTLSKeyValue(m *environmentResourceModel, wo string) string {
	if m.TLSKeyWOVersion.ValueInt64() != 0 && wo != "" {
		return wo
	}
	return m.TLSKey.ValueString()
}

// environmentEnsureEdgeID gives Edge Agent environments a usable edge_id when
// Portainer does not supply one. The server only assigns an Edge ID at
// creation while the EnforceEdgeID setting is enabled; otherwise the
// endpoint's EdgeID stays empty until an agent connects and reports one,
// leaving nothing to template PORTAINER_EDGE_ID from (issue #142). The
// Portainer UI covers the same gap by generating a UUID client-side, and the
// server adopts the ID the agent reports while its own is empty.
//
// It must stay create-time only: a plan does not persist the state it
// refreshes, so minting one in Read would show phantom drift on every plan
// for an environment whose stored edge_id is empty.
//
// The type is the one the API reported, because Portainer's creation enum
// stops at 5 and can hand back a non-edge environment for a type it does not
// recognise.
func environmentEnsureEdgeID(m *environmentResourceModel) {
	if !environmentIsEdge(m.Type.ValueInt64()) {
		return
	}
	if m.EdgeID.ValueString() != "" {
		return
	}
	m.EdgeID = types.StringValue(uuid.New().String())
}

// environmentOp carries what an update needs besides the working model: the
// prior state (nil on create, where every set value counts as changed), the
// write-only keys and the configured public_ip.
type environmentOp struct {
	client         *APIClient
	prior          *environmentResourceModel
	wo             string
	azureWO        string
	configPublicIP types.String
}

func (op *environmentOp) old() *environmentResourceModel {
	if op.prior != nil {
		return op.prior
	}
	return &environmentResourceModel{}
}

// update sends PUT /endpoints/{id} (after uploading changed TLS files) and
// reads the environment back into m, as the SDKv2 Update did.
func (op *environmentOp) update(ctx context.Context, m *environmentResourceModel, diags *diag.Diagnostics) bool {
	client := op.client
	old := op.old()
	body := environmentUpdateBody{
		Name:    m.Name.ValueString(),
		GroupID: int(m.GroupID.ValueInt64()),
	}

	// Edge Agent environments (type 4, 7) trigger proxy/tunnel registration on
	// update when URL or TLS fields are included, which fails when the agent is
	// not yet connected. Only send metadata fields for edge agents.
	isEdgeAgent := environmentIsEdge(m.Type.ValueInt64())
	if !isEdgeAgent {
		url := m.EnvironmentAddress.ValueString()
		body.URL = &url
		body.TLS = new(m.TLSEnabled.ValueBool())
		body.TLSSkipVerify = new(m.TLSSkipVerify.ValueBool())
		body.TLSSkipClientVerify = new(m.TLSSkipClientVerify.ValueBool())

		if err := op.uploadTLSFiles(ctx, m); err != nil {
			diags.AddError("Failed to update environment", fmt.Errorf("failed to upload TLS files for environment: %w", err).Error())
			return false
		}
	}

	// public_ip is computed, so only a configured value is sent. Without one,
	// a new connection URL becomes the public URL as well, which is how
	// Portainer defaults it for directly-connected environments (edge agents
	// connect back to Portainer, so their address is no public URL).
	if v := op.configPublicIP.ValueString(); v != "" {
		body.PublicURL = v
	} else if !isEdgeAgent && environmentStringChanged(m.EnvironmentAddress, old.EnvironmentAddress) {
		body.PublicURL = m.EnvironmentAddress.ValueString()
	}

	// An empty list clears the tags, but only when tag_ids itself changed, so
	// tags assigned outside Terraform survive when the attribute is not set.
	tags := environmentInt64s(m.TagIDs)
	if len(tags) > 0 || environmentListChanged(m.TagIDs, old.TagIDs) {
		body.TagIDs = make([]int, 0, len(tags))
		for _, t := range tags {
			body.TagIDs = append(body.TagIDs, int(t))
		}
	}
	body.UserAccessPolicies = environmentExpandPolicies(m.UserAccessPolicies, old.UserAccessPolicies)
	body.TeamAccessPolicies = environmentExpandPolicies(m.TeamAccessPolicies, old.TeamAccessPolicies)
	op.setAzureCredentials(m, &body)
	op.setEdgeIntervals(m, &body)

	if err := doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/endpoints/%s", client.Endpoint, m.ID.ValueString()), body, nil); err != nil {
		diags.AddError("Failed to update environment", fmt.Errorf("failed to update environment: %w", err).Error())
		return false
	}

	// A real address change on an edge environment never reaches Update:
	// ModifyPlan forces replacement for it.
	found, err := environmentRead(ctx, client, m)
	if err != nil {
		diags.AddError("Failed to read environment", err.Error())
		return false
	}
	if !found {
		diags.AddError("Failed to read environment", fmt.Sprintf("environment %s was not found after the update", m.ID.ValueString()))
		return false
	}
	return true
}

// setAzureCredentials adds the Azure credentials that changed. Portainer
// keeps a credential the payload leaves out, and nothing clears one.
func (op *environmentOp) setAzureCredentials(m *environmentResourceModel, body *environmentUpdateBody) {
	old := op.old()
	changed := func(v string, isChanged bool) *string {
		if v == "" || !isChanged {
			return nil
		}
		return &v
	}
	body.AzureApplicationID = changed(m.AzureApplicationID.ValueString(), environmentStringChanged(m.AzureApplicationID, old.AzureApplicationID))
	body.AzureTenantID = changed(m.AzureTenantID.ValueString(), environmentStringChanged(m.AzureTenantID, old.AzureTenantID))
	body.AzureAuthenticationKey = changed(environmentAzureKeyValue(m, op.azureWO),
		environmentStringChanged(m.AzureAuthenticationKey, old.AzureAuthenticationKey) ||
			environmentInt64Changed(m.AzureAuthenticationKeyWOVersion, old.AzureAuthenticationKeyWOVersion))
}

// setEdgeIntervals adds the edge agent intervals that are configured and
// differ from the state (on create: that are configured). An unset interval
// is unknown on create and the state value on update. The async intervals go
// as one object; Portainer keeps the ones left at 0 on its default.
func (op *environmentOp) setEdgeIntervals(m *environmentResourceModel, body *environmentUpdateBody) {
	old := op.old()
	known := func(v types.Int64) bool { return !v.IsNull() && !v.IsUnknown() }
	changed := func(cur, prev types.Int64) bool {
		return known(cur) && (op.prior == nil || environmentInt64Changed(cur, prev))
	}
	if changed(m.EdgeCheckinInterval, old.EdgeCheckinInterval) {
		body.EdgeCheckinInterval = new(m.EdgeCheckinInterval.ValueInt64())
	}
	if changed(m.EdgePingInterval, old.EdgePingInterval) || changed(m.EdgeSnapshotInterval, old.EdgeSnapshotInterval) || changed(m.EdgeCommandInterval, old.EdgeCommandInterval) {
		current := func(cur, prev types.Int64) int64 {
			if known(cur) {
				return cur.ValueInt64()
			}
			return prev.ValueInt64()
		}
		body.Edge = &environmentEdgeUpdate{
			PingInterval:     current(m.EdgePingInterval, old.EdgePingInterval),
			SnapshotInterval: current(m.EdgeSnapshotInterval, old.EdgeSnapshotInterval),
			CommandInterval:  current(m.EdgeCommandInterval, old.EdgeCommandInterval),
		}
	}
}

// environmentCreateExtraFields adds the optional create-only and Azure/Edge
// fields of POST /endpoints that are set; unset ones are left out.
func environmentCreateExtraFields(m *environmentResourceModel, azureWO string, addField func(name, value string)) {
	addField("AzureApplicationID", m.AzureApplicationID.ValueString())
	addField("AzureTenantID", m.AzureTenantID.ValueString())
	addField("AzureAuthenticationKey", environmentAzureKeyValue(m, azureWO))
	addField("EdgeTunnelServerAddress", m.EdgeTunnelServerAddress.ValueString())
	if !m.EdgeAsyncMode.IsNull() && !m.EdgeAsyncMode.IsUnknown() {
		addField("EdgeAsyncMode", strconv.FormatBool(m.EdgeAsyncMode.ValueBool()))
	}
	addIntField := func(name string, v types.Int64) {
		if !v.IsNull() && !v.IsUnknown() {
			addField(name, strconv.FormatInt(v.ValueInt64(), 10))
		}
	}
	addIntField("EdgeCheckinInterval", m.EdgeCheckinInterval)
	addIntField("EdgePingInterval", m.EdgePingInterval)
	addIntField("EdgeSnapshotInterval", m.EdgeSnapshotInterval)
	addIntField("EdgeCommandInterval", m.EdgeCommandInterval)
	addIntField("CustomTemplateID", m.CustomTemplateID)
	addField("CustomTemplateContent", m.CustomTemplateContent.ValueString())
	addField("StackName", m.CustomTemplateStackName.ValueString())
}

// environmentAzureKeyValue returns the Azure key from azure_authentication_key,
// or from the write-only attribute while its version is set.
func environmentAzureKeyValue(m *environmentResourceModel, wo string) string {
	if m.AzureAuthenticationKeyWOVersion.ValueInt64() != 0 && wo != "" {
		return wo
	}
	return m.AzureAuthenticationKey.ValueString()
}

// environmentBusinessEditionAttrs lists the Business Edition only attributes
// a create (state nil) or an update would send. Community Edition accepts and
// ignores them, which would otherwise show as drift on every plan.
func environmentBusinessEditionAttrs(plan, state *environmentResourceModel) []string {
	var attrs []string
	if state == nil {
		if plan.EdgeAsyncMode.ValueBool() {
			attrs = append(attrs, "edge_async_mode")
		}
		if !plan.CustomTemplateID.IsNull() || plan.CustomTemplateContent.ValueString() != "" {
			attrs = append(attrs, "custom_template_id/custom_template_content")
		}
		state = &environmentResourceModel{}
	}
	for _, a := range []struct {
		name     string
		cur, old types.Int64
	}{
		{"edge_ping_interval", plan.EdgePingInterval, state.EdgePingInterval},
		{"edge_snapshot_interval", plan.EdgeSnapshotInterval, state.EdgeSnapshotInterval},
		{"edge_command_interval", plan.EdgeCommandInterval, state.EdgeCommandInterval},
	} {
		if a.cur.ValueInt64() != 0 && environmentInt64Changed(a.cur, a.old) {
			attrs = append(attrs, a.name)
		}
	}
	return attrs
}

// environmentCheckBusinessEdition fails when attrs is not empty and the server
// is not Business Edition. The version is only asked for when such an
// attribute is sent; a server that cannot tell is not blocked.
func environmentCheckBusinessEdition(ctx context.Context, client *APIClient, attrs []string) error {
	if len(attrs) == 0 {
		return nil
	}
	var version struct {
		ServerVersion string `json:"ServerVersion"`
		ServerEdition string `json:"ServerEdition"`
	}
	known := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/system/version", nil, &version) == nil && version.ServerVersion != ""
	if !known || version.ServerEdition == "EE" || version.ServerEdition == "BE" {
		return nil
	}
	return fmt.Errorf("%s require %s; this server is Portainer CE %s", strings.Join(attrs, ", "), requiresBE, version.ServerVersion)
}

func environmentExpandPolicies(cur, old types.Map) map[string]environmentRoleRef {
	raw := environmentInt64Map(cur)
	if len(raw) == 0 && !environmentMapChanged(cur, old) {
		return nil
	}
	policies := make(map[string]environmentRoleRef, len(raw))
	for id, role := range raw {
		policies[id] = environmentRoleRef{RoleID: int(role)}
	}
	return policies
}

// uploadTLSFiles stores the configured CA, client certificate and key in the
// environment's TLS folder. Portainer only reads these files from disk: PUT
// /endpoints/{id} points the TLS config at them but cannot carry their
// content, so a rotated certificate has to be uploaded before the update that
// reloads the proxy.
func (op *environmentOp) uploadTLSFiles(ctx context.Context, m *environmentResourceModel) error {
	if !m.TLSEnabled.ValueBool() {
		return nil
	}
	old := op.old()
	changed := environmentBoolChanged(m.TLSEnabled, old.TLSEnabled) ||
		environmentBoolChanged(m.TLSSkipVerify, old.TLSSkipVerify) ||
		environmentBoolChanged(m.TLSSkipClientVerify, old.TLSSkipClientVerify) ||
		environmentStringChanged(m.TLSCACert, old.TLSCACert) ||
		environmentStringChanged(m.TLSCert, old.TLSCert) ||
		environmentStringChanged(m.TLSKey, old.TLSKey) ||
		environmentInt64Changed(m.TLSKeyWOVersion, old.TLSKeyWOVersion)
	if !changed {
		return nil
	}
	files := []struct {
		kind, content string
		skip          bool
	}{
		{"ca", m.TLSCACert.ValueString(), m.TLSSkipVerify.ValueBool()},
		{"cert", m.TLSCert.ValueString(), m.TLSSkipClientVerify.ValueBool()},
		{"key", environmentTLSKeyValue(m, op.wo), m.TLSSkipClientVerify.ValueBool()},
	}
	for _, f := range files {
		if f.content == "" || f.skip {
			continue
		}
		if err := uploadTLSFile(ctx, op.client, m.ID.ValueString(), f.kind, f.content); err != nil {
			return err
		}
	}
	return nil
}

// environmentRead refreshes m from GET /endpoints/{id}, using the values m
// already holds where the SDKv2 Read used the prior state. It reports false
// when the environment is gone.
func environmentRead(ctx context.Context, client *APIClient, m *environmentResourceModel) (bool, error) {
	id, _ := strconv.ParseInt(m.ID.ValueString(), 10, 64)

	var env portainerEndpoint
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/endpoints/%d", client.Endpoint, id), nil, &env); err != nil {
		if isAPINotFound(err) {
			return false, nil
		}
		return false, fmt.Errorf("failed to read environment: %w", err)
	}

	m.Name = types.StringValue(env.Name)
	// Destroy-time setting with no server-side counterpart: keep what state
	// holds, and store the default after an import instead of planning it.
	if m.DeleteClusterOnDestroy.IsNull() || m.DeleteClusterOnDestroy.IsUnknown() {
		m.DeleteClusterOnDestroy = types.BoolValue(false)
	}
	m.Type = types.Int64Value(env.Type)
	m.GroupID = types.Int64Value(env.GroupID)
	envType := env.Type
	// Until an agent associates, Portainer reports an empty EdgeID (unless the
	// EnforceEdgeID setting is enabled), so for Edge Agent environments keep the
	// ID generated at create time rather than wiping it (issue #142). A
	// server-reported ID always wins, and non-edge environments keep reflecting
	// the API verbatim.
	edgeID := env.EdgeID
	if edgeID == "" && environmentIsEdge(envType) {
		edgeID = m.EdgeID.ValueString()
	}
	m.EdgeID = types.StringValue(edgeID)
	m.EdgeKey = types.StringValue(env.EdgeKey)

	// Portainer stores the Edge Agent URL as a bare host (issue #136). The edge
	// key still carries the address verbatim, so prefer it and fall back to
	// the endpoint URL only when the key is missing or unparsable, keeping the
	// scheme already in state in that case. Agent environments lose the scheme
	// the same way ("tcp://host:9001" comes back as "host:9001"), so any
	// environment keeps the address in state when only the scheme is missing.
	address := env.URL
	fromKey := ""
	if environmentIsEdge(envType) {
		fromKey = edgeAddressFromKey(env.EdgeKey)
	}
	if fromKey != "" {
		address = fromKey
	} else if s := m.EnvironmentAddress.ValueString(); s != "" && s != address && trimURLScheme(s) == trimURLScheme(address) {
		address = s
	}
	m.EnvironmentAddress = types.StringValue(address)

	m.PublicIP = types.StringValue(env.PublicURL)

	// v3.0.0 kept an omitted tag_ids null in the state; an empty list of
	// tags stays null there.
	if len(env.TagIds) == 0 && (m.TagIDs.IsNull() || m.TagIDs.IsUnknown()) {
		m.TagIDs = types.ListNull(types.Int64Type)
	} else {
		tagIDs := make([]int, 0, len(env.TagIds))
		for _, tid := range env.TagIds {
			tagIDs = append(tagIDs, int(tid))
		}
		m.TagIDs = int64ListValue(tagIDs)
	}

	// Edge environments are left alone: Portainer keeps TLS off for them
	// whatever the schema defaults say. The skip flags are only meaningful
	// with TLS on (Portainer clears them when it is off), and client
	// verification has no flag of its own: it is on when a client certificate
	// is stored. Azure environments (3) are created without a TLS
	// configuration at all.
	if envType != 3 && !environmentIsEdge(envType) && env.TLSConfig != nil {
		tls := env.TLSConfig
		m.TLSEnabled = types.BoolValue(tls.TLS)
		if tls.TLS {
			m.TLSSkipVerify = types.BoolValue(tls.TLSSkipVerify)
			m.TLSSkipClientVerify = types.BoolValue(tls.TLSCert == "")
		}
	}

	// Edge Agent environments created by an older Portainer store an empty
	// ContainerEngine for Docker, and no migration fills it in.
	m.ContainerEngine = types.StringValue(normalizeContainerEngine(env.ContainerEngine))

	// 0 means the default interval from the Portainer settings. The Azure
	// credentials are never returned, and the tunnel server address and the
	// custom template only exist at creation: they keep what m holds.
	m.EdgeCheckinInterval = types.Int64Value(env.EdgeCheckinInterval)
	m.EdgeAsyncMode = types.BoolValue(env.Edge.AsyncMode)
	m.EdgePingInterval = types.Int64Value(env.Edge.PingInterval)
	m.EdgeSnapshotInterval = types.Int64Value(env.Edge.SnapshotInterval)
	m.EdgeCommandInterval = types.Int64Value(env.Edge.CommandInterval)
	return true, nil
}

// normalizeContainerEngine maps the empty engine that older Portainer versions
// store for Docker Edge Agent environments to "docker".
func normalizeContainerEngine(engine string) string {
	if engine == "" {
		return "docker"
	}
	return engine
}

// trimURLScheme strips a leading http://, https:// or tcp:// from an
// environment address so values can be compared the way Portainer stores them
// for Edge Agents and Agents (scheme-less host).
func trimURLScheme(s string) string {
	for _, scheme := range []string{"https://", "http://", "tcp://"} {
		if trimmed, ok := strings.CutPrefix(s, scheme); ok {
			return trimmed
		}
	}
	return s
}

// edgeAddressFromKey recovers the Portainer address an Edge Agent environment
// was created with. Portainer stores endpoint.URL for edge types as a bare host
// (edge.ParseHostForEdge drops the scheme, the port and the path), but the edge
// key keeps the submitted URL verbatim in its first field:
//
//	base64("<portainer url>|<tunnel host:port>|<tls fingerprint>|<endpoint id>")
//
// so the key - not endpoint.URL - is the lossless source for
// environment_address. Returns "" when the key is absent or not in that shape,
// which leaves the caller on its endpoint.URL fallback.
func edgeAddressFromKey(edgeKey string) string {
	return edgeKeyField(edgeKey, 0)
}

// edgeTunnelFromKey returns the tunnel server address (host:port) an Edge
// Agent environment was created with: the second field of the edge key.
func edgeTunnelFromKey(edgeKey string) string {
	return edgeKeyField(edgeKey, 1)
}

func edgeKeyField(edgeKey string, i int) string {
	if edgeKey == "" {
		return ""
	}
	decoded, err := base64.StdEncoding.DecodeString(edgeKey)
	if err != nil {
		// Portainer emits the key unpadded (base64.RawStdEncoding).
		decoded, err = base64.RawStdEncoding.DecodeString(edgeKey)
		if err != nil {
			return ""
		}
	}
	fields := strings.Split(string(decoded), "|")
	if len(fields) != 4 {
		return ""
	}
	return strings.TrimSpace(fields[i])
}

func findExistingEnvironmentByName(ctx context.Context, client *APIClient, name string) (int, error) {
	envs, err := listEnvironments(ctx, client)
	if err != nil {
		return 0, err
	}
	for _, e := range envs {
		if e.Name == name {
			return int(e.ID), nil
		}
	}
	return 0, nil
}

// portainerEndpoint is the part of a Portainer environment (endpoint) the
// provider reads.
type portainerEndpoint struct {
	ID              int64               `json:"Id"`
	Name            string              `json:"Name"`
	Type            int64               `json:"Type"`
	GroupID         int64               `json:"GroupId"`
	URL             string              `json:"URL"`
	PublicURL       string              `json:"PublicURL"`
	EdgeID          string              `json:"EdgeID"`
	EdgeKey         string              `json:"EdgeKey"`
	TagIds          []int64             `json:"TagIds"`
	TLSConfig       *portainerTLSConfig `json:"TLSConfig"`
	ContainerEngine string              `json:"ContainerEngine"`

	EdgeCheckinInterval int64                 `json:"EdgeCheckinInterval"`
	Edge                portainerEndpointEdge `json:"Edge"`
}

// portainerEndpointEdge holds the Edge Agent async-mode settings.
type portainerEndpointEdge struct {
	AsyncMode        bool  `json:"AsyncMode"`
	PingInterval     int64 `json:"PingInterval"`
	SnapshotInterval int64 `json:"SnapshotInterval"`
	CommandInterval  int64 `json:"CommandInterval"`
}

type portainerTLSConfig struct {
	TLS           bool   `json:"TLS"`
	TLSSkipVerify bool   `json:"TLSSkipVerify"`
	TLSCert       string `json:"TLSCert"`
}

// listEnvironments returns every environment (GET /endpoints).
func listEnvironments(ctx context.Context, client *APIClient) ([]portainerEndpoint, error) {
	var envs []portainerEndpoint
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/endpoints", nil, &envs); err != nil {
		return nil, fmt.Errorf("failed to list environments: %w", err)
	}
	return envs, nil
}

// environmentUpdateBody is the PUT /endpoints/{id} payload. Portainer treats a
// missing (null) field as "keep the current value", so the pointers, slices
// and maps here are nil unless the field is meant to be written.
type environmentUpdateBody struct {
	Name                string                        `json:"name,omitempty"`
	GroupID             int                           `json:"groupID,omitempty"`
	URL                 *string                       `json:"url,omitempty"`
	PublicURL           string                        `json:"publicURL,omitempty"`
	TLS                 *bool                         `json:"tls,omitempty"`
	TLSSkipVerify       *bool                         `json:"tlsskipVerify,omitempty"`
	TLSSkipClientVerify *bool                         `json:"tlsskipClientVerify,omitempty"`
	TagIDs              []int                         `json:"tagIDs"`
	UserAccessPolicies  map[string]environmentRoleRef `json:"userAccessPolicies"`
	TeamAccessPolicies  map[string]environmentRoleRef `json:"teamAccessPolicies"`

	AzureApplicationID     *string                `json:"AzureApplicationID,omitempty"`
	AzureTenantID          *string                `json:"AzureTenantID,omitempty"`
	AzureAuthenticationKey *string                `json:"AzureAuthenticationKey,omitempty"`
	EdgeCheckinInterval    *int64                 `json:"EdgeCheckinInterval,omitempty"`
	Edge                   *environmentEdgeUpdate `json:"Edge,omitempty"`
}

// environmentEdgeUpdate is the Business Edition "Edge" object of PUT
// /endpoints/{id}: the async-mode intervals.
type environmentEdgeUpdate struct {
	PingInterval     int64 `json:"PingInterval"`
	SnapshotInterval int64 `json:"SnapshotInterval"`
	CommandInterval  int64 `json:"CommandInterval"`
}

type environmentRoleRef struct {
	RoleID int `json:"RoleId"`
}

func uploadTLSFile(ctx context.Context, client *APIClient, folder, kind, content string) error {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("folder", folder); err != nil {
		return err
	}
	part, err := writer.CreateFormFile("file", kind+".pem")
	if err != nil {
		return err
	}
	if _, err := io.WriteString(part, content); err != nil {
		return err
	}
	if err := writer.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.Endpoint+"/upload/tls/"+kind, bytes.NewReader(body.Bytes()))
	if err != nil {
		return err
	}
	if err := setAuthHeader(req, client); err != nil {
		return err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to upload TLS %s file: %w", kind, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		respBody, _ := io.ReadAll(resp.Body)
		return &apiStatusError{Method: http.MethodPost, URL: req.URL.String(), StatusCode: resp.StatusCode, Body: string(respBody)}
	}
	return nil
}

// environmentRemovePollInterval is how often waitEnvironmentRemoved checks;
// tests shorten it.
var environmentRemovePollInterval = 5 * time.Second

// waitEnvironmentRemoved polls GET /endpoints/{id} until it answers 404, the
// timeout elapses or ctx is done.
func waitEnvironmentRemoved(ctx context.Context, client *APIClient, id int64, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("%s/endpoints/%d?excludeSnapshot=true", client.Endpoint, id)
	for {
		err := doJSON(ctx, client, http.MethodGet, url, nil, nil)
		if isAPINotFound(err) {
			return nil
		}
		if err != nil && ctx.Err() != nil {
			return ctx.Err()
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("environment %d still exists %s after the delete request (deleteCluster=true); Portainer may still be uninstalling KubeSolo or the removal failed, check the environment in Portainer", id, timeout)
		}
		if err := sleepCtx(ctx, environmentRemovePollInterval); err != nil {
			return err
		}
	}
}
