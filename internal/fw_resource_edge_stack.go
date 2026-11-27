package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-framework-timeouts/resource/timeouts"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*edgeStackResource)(nil)
	_ resource.ResourceWithConfigure   = (*edgeStackResource)(nil)
	_ resource.ResourceWithImportState = (*edgeStackResource)(nil)
	_ resource.ResourceWithModifyPlan  = (*edgeStackResource)(nil)
	_ resource.ResourceWithIdentity    = (*edgeStackResource)(nil)
)

func newEdgeStackResource() resource.Resource { return &edgeStackResource{} }

type edgeStackResource struct {
	fwIDIdentity
	client *APIClient
}

type edgeStackModel struct {
	ID                          types.String   `tfsdk:"id"`
	Name                        types.String   `tfsdk:"name"`
	StackFileContent            types.String   `tfsdk:"stack_file_content"`
	StackFilePath               types.String   `tfsdk:"stack_file_path"`
	PrePullImage                types.Bool     `tfsdk:"pre_pull_image"`
	RetryDeploy                 types.Bool     `tfsdk:"retry_deploy"`
	Dryrun                      types.Bool     `tfsdk:"dryrun"`
	RepositoryURL               types.String   `tfsdk:"repository_url"`
	SourceID                    types.Int64    `tfsdk:"source_id"`
	GitRepositoryAuthentication types.Bool     `tfsdk:"git_repository_authentication"`
	RepositoryUsername          types.String   `tfsdk:"repository_username"`
	RepositoryPassword          types.String   `tfsdk:"repository_password"`
	RepositoryPasswordWO        types.String   `tfsdk:"repository_password_wo"`
	RepositoryPasswordWOVersion types.Int64    `tfsdk:"repository_password_wo_version"`
	RepositoryReferenceName     types.String   `tfsdk:"repository_reference_name"`
	FilePathInRepository        types.String   `tfsdk:"file_path_in_repository"`
	DeploymentType              types.Int64    `tfsdk:"deployment_type"`
	EdgeGroups                  types.List     `tfsdk:"edge_groups"`
	Registries                  types.List     `tfsdk:"registries"`
	KubernetesPrune             types.Bool     `tfsdk:"kubernetes_prune"`
	KubernetesForceApply        types.Bool     `tfsdk:"kubernetes_force_apply"`
	UseManifestNamespaces       types.Bool     `tfsdk:"use_manifest_namespaces"`
	StackWebhook                types.Bool     `tfsdk:"stack_webhook"`
	ForceUpdate                 types.Bool     `tfsdk:"force_update"`
	UpdateInterval              types.String   `tfsdk:"update_interval"`
	PullImage                   types.Bool     `tfsdk:"pull_image"`
	WebhookID                   types.String   `tfsdk:"webhook_id"`
	WebhookURL                  types.String   `tfsdk:"webhook_url"`
	Environment                 types.Map      `tfsdk:"environment"`
	RelativePath                types.String   `tfsdk:"relative_path"`
	RepositoryGitCredentialID   types.Int64    `tfsdk:"repository_git_credential_id"`
	AlwaysClone                 types.Bool     `tfsdk:"always_clone"`
	HelmConfig                  types.List     `tfsdk:"helm_config"`
	StaggerConfig               types.List     `tfsdk:"stagger_config"`
	PerDeviceConfigsPath        types.String   `tfsdk:"per_device_configs_path"`
	PerDeviceConfigsMatchType   types.String   `tfsdk:"per_device_configs_match_type"`
	PerDeviceConfigsGroupMatch  types.String   `tfsdk:"per_device_configs_group_match_type"`
	Timeouts                    timeouts.Value `tfsdk:"timeouts"`
}

// edgeStackHelmAttrTypes is the object type of a helm_config block.
var edgeStackHelmAttrTypes = map[string]attr.Type{
	"chart_url":     types.StringType,
	"chart_name":    types.StringType,
	"chart_version": types.StringType,
	"namespace":     types.StringType,
	"values_inline": types.StringType,
	"atomic":        types.BoolType,
	"timeout":       types.StringType,
}

type edgeStackHelmModel struct {
	ChartURL     types.String `tfsdk:"chart_url"`
	ChartName    types.String `tfsdk:"chart_name"`
	ChartVersion types.String `tfsdk:"chart_version"`
	Namespace    types.String `tfsdk:"namespace"`
	ValuesInline types.String `tfsdk:"values_inline"`
	Atomic       types.Bool   `tfsdk:"atomic"`
	Timeout      types.String `tfsdk:"timeout"`
}

func (r *edgeStackResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_edge_stack"
}

func (r *edgeStackResource) Schema(ctx context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Computed:      true,
				Description:   "Identifier of the Edge stack.",
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the Portainer Edge stack.",
			},
			"stack_file_content": schema.StringAttribute{
				Optional:    true,
				Description: "Inline content of the Docker Compose or Kubernetes manifest used to deploy the Edge stack.",
			},
			"stack_file_path": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Local filesystem path to a Compose or manifest file used as the Edge stack definition. Changing this value forces resource recreation.",
			},
			"pre_pull_image": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether Portainer should pre-pull images on Edge agents before deploying the stack.",
			},
			"retry_deploy": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether the Edge agent should retry deployment on failure.",
			},
			"dryrun": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "If true, perform a dry-run of the Edge stack deployment without applying changes.",
			},
			"repository_url": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{edgeOptionalZeroString{}, edgeRequiresReplaceString()},
				Description:   "Git repository URL containing the Edge stack definition. Changing this value forces resource recreation.",
			},
			"source_id": schema.Int64Attribute{
				Optional:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				Description:   "ID of a Portainer Source (Git source) providing the repository URL and credentials. Portainer 2.43 and later. When set, `repository_url` and the repository credentials can be left out; `repository_reference_name` and `file_path_in_repository` still apply. Changing this value forces resource recreation.",
			},
			"git_repository_authentication": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether authentication is required to clone the Git repository.",
			},
			"repository_username": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{edgeOptionalZeroString{}, edgeRequiresReplaceString()},
				Description:   "Username used to authenticate against the Git repository. Changing this value forces resource recreation.",
			},
			"repository_password": schema.StringAttribute{
				Optional:      true,
				Sensitive:     true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{stringvalidator.ConflictsWith(path.MatchRoot("repository_password_wo"))},
				Description:   "Password or personal access token used to authenticate against the Git repository. Stored in state as sensitive value; use `repository_password_wo` to keep it out. Changing this value forces resource recreation.",
			},
			"repository_password_wo": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("repository_password")),
					stringvalidator.AlsoRequires(path.MatchRoot("repository_password_wo_version")),
				},
				Description: "Write-only Git password or personal access token (supports ephemeral values; never stored in Terraform state). Conflicts with `repository_password`. Requires Terraform 1.11 or later.",
			},
			"repository_password_wo_version": schema.Int64Attribute{
				Optional:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				Description:   "Version of `repository_password_wo`. Increment it to send a new value. Changing it forces resource recreation.",
			},
			"repository_reference_name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("refs/heads/main"),
				Description: "Git reference (branch or tag) to check out from the repository.",
			},
			"file_path_in_repository": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				Default:       stringdefault.StaticString("docker-compose.yml"),
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Description:   "Path to the Compose or manifest file within the Git repository. Changing this value forces resource recreation.",
			},
			"deployment_type": schema.Int64Attribute{
				Required:    true,
				Description: "0 = Docker Compose, 1 = Kubernetes",
			},
			"edge_groups": schema.ListAttribute{
				Required:    true,
				ElementType: types.Int64Type,
				Description: "List of Portainer Edge group IDs to which this Edge stack is deployed.",
			},
			"registries": schema.ListAttribute{
				Optional:    true,
				ElementType: types.Int64Type,
				Description: "List of Portainer registry IDs used to pull images for this Edge stack.",
			},
			"kubernetes_prune": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				Description:   "Kubernetes deployments (deployment_type = 1) only, Portainer 3.0 or later: whether a deployment deletes the Kubernetes resources an earlier deployment applied that the manifest no longer declares. Portainer prunes when this is left out; leaving it out keeps the choice Portainer has recorded and sends nothing, so older Portainer versions are unaffected. Not sent for a helm_config stack.",
			},
			"kubernetes_force_apply": schema.BoolAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Bool{boolplanmodifier.UseStateForUnknown()},
				Description:   "Kubernetes deployments (deployment_type = 1) only, Portainer 3.0 or later: whether a deployment deletes and recreates a Kubernetes resource whose immutable fields changed instead of failing on it. Recreating destroys the resource and everything tied to its identity. Portainer leaves it off when this is left out; leaving it out keeps the choice Portainer has recorded and sends nothing. Not sent for a helm_config stack.",
			},
			"use_manifest_namespaces": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "For Kubernetes deployments, whether to use namespaces declared inside the manifest instead of the default Portainer namespace.",
			},
			"stack_webhook": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Enable autoUpdate webhook (GitOps).",
			},
			"force_update": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether to prune unused services/networks during stack update (default: true)",
			},
			"update_interval": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{edgeOptionalZeroString{}},
				Description:   "GitOps update interval (e.g. \"5m\") at which Portainer polls the Git repository for Edge stack changes.",
			},
			"pull_image": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether to force pull the latest images when the stack is updated (default: false). Sent on updates (RepullImageAndRedeploy) and, with GitOps, as AutoUpdate.ForcePullImage; Portainer's create endpoints have no field for it, so a create without GitOps does not send it (use `pre_pull_image`).",
			},
			"webhook_id": schema.StringAttribute{
				Computed:    true,
				Description: "UUID of the GitOps webhook (read-only).",
			},
			"webhook_url": schema.StringAttribute{
				Computed:    true,
				Sensitive:   true,
				Description: "Full URL of the webhook trigger. Sensitive: anyone with the URL can redeploy the stack.",
			},
			"environment": schema.MapAttribute{
				Optional:    true,
				ElementType: types.StringType,
				Description: "Environment variables for the Edge Stack",
			},
			// SDKv2 declared Default "" and stored null for it; null and ""
			// are the same here.
			"relative_path": schema.StringAttribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.String{edgeOptionalZeroString{}, edgeRequiresReplaceString()},
				Description:   "Enable relative path volumes – also used as value for 'filesystemPath'.",
			},
			"repository_git_credential_id": schema.Int64Attribute{
				Optional:      true,
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{edgeOptionalZeroInt64{}},
				Description:   "ID of the Git credentials to use for authentication.",
			},
			"always_clone": schema.BoolAttribute{
				Optional:    true,
				Computed:    true,
				Default:     booldefault.StaticBool(false),
				Description: "Whether the agent must always clone the git repository for relative path. Only valid when relative_path is set.",
			},
			// Per-device configurations are only accepted when a git stack is
			// created; the update endpoint has no field for them.
			"per_device_configs_path": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators:    []validator.String{stringvalidator.AlsoRequires(path.MatchRoot("relative_path"))},
				Description:   "Git stacks with `relative_path` only, Business Edition only: path within the repository holding per-device configurations. Setting it turns per-device configurations on (SupportPerDeviceConfigs). Changing this value forces resource recreation.",
			},
			"per_device_configs_match_type": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					stringvalidator.OneOf("file", "dir"),
					stringvalidator.AlsoRequires(path.MatchRoot("per_device_configs_path")),
				},
				Description: "How per-device configurations are matched to a device: `file` or `dir`. Requires `per_device_configs_path`. Changing this value forces resource recreation.",
			},
			"per_device_configs_group_match_type": schema.StringAttribute{
				Optional:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
				Validators: []validator.String{
					stringvalidator.OneOf("file", "dir"),
					stringvalidator.AlsoRequires(path.MatchRoot("per_device_configs_path")),
				},
				Description: "How per-device configurations are matched to an Edge group: `file` or `dir`. Requires `per_device_configs_path`. Changing this value forces resource recreation.",
			},
		},
		Blocks: map[string]schema.Block{
			// Business Edition only. Setting this block is what selects the
			// Helm repository deployment path, the same way stack_file_content
			// selects the string one.
			"helm_config": schema.ListNestedBlock{
				Description: "Deploy the stack from a Helm chart repository instead of a compose file or a git repository. Business Edition only.",
				Validators:  []validator.List{listvalidator.SizeAtMost(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"chart_url": schema.StringAttribute{
							Required:    true,
							Description: "URL of the Helm chart repository.",
						},
						"chart_name": schema.StringAttribute{
							Required:    true,
							Description: "Name of the Helm chart within the repository.",
						},
						"chart_version": schema.StringAttribute{
							Optional:      true,
							Computed:      true,
							PlanModifiers: []planmodifier.String{edgeOptionalZeroString{}},
							Description:   "Version of the chart to deploy. Leave unset to deploy the latest published version - note that Portainer then picks the version, so a re-apply can move the stack.",
						},
						"namespace": schema.StringAttribute{
							Optional:      true,
							Computed:      true,
							PlanModifiers: []planmodifier.String{edgeOptionalZeroString{}},
							Description:   "Kubernetes namespace to deploy the chart into.",
						},
						"values_inline": schema.StringAttribute{
							Optional:      true,
							Computed:      true,
							PlanModifiers: []planmodifier.String{edgeOptionalZeroString{}},
							Description:   "Helm values as an inline YAML string, the equivalent of a values file passed to `helm install`.",
						},
						"atomic": schema.BoolAttribute{
							Optional:      true,
							Computed:      true,
							PlanModifiers: []planmodifier.Bool{edgeOptionalZeroBool{}},
							Description:   "Whether a failed deployment is rolled back automatically, the equivalent of `helm --atomic`.",
						},
						"timeout": schema.StringAttribute{
							Optional:      true,
							Computed:      true,
							PlanModifiers: []planmodifier.String{edgeOptionalZeroString{}},
							Description:   "Deadline for Helm operations, the equivalent of `helm --timeout` (for example `5m0s`).",
						},
					},
				},
			},
			"stagger_config": edgeStaggerConfigBlock(),
			"timeouts":       timeouts.Block(ctx, timeouts.Opts{Create: true, Update: true, Delete: true}),
		},
	}
}

func (r *edgeStackResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// ModifyPlan rejects the Kubernetes apply policies on a Docker Compose edge
// stack, where Portainer ignores them, and keeps the webhook outputs of an
// update that leaves stack_webhook alone (an update keeps the webhook).
func (r *edgeStackResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var deploymentType types.Int64
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("deployment_type"), &deploymentType)...)
	if !deploymentType.IsUnknown() && !deploymentType.IsNull() && deploymentType.ValueInt64() != 1 {
		for _, key := range []string{"kubernetes_prune", "kubernetes_force_apply"} {
			var v types.Bool
			resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root(key), &v)...)
			if !v.IsNull() {
				resp.Diagnostics.AddAttributeError(path.Root(key), "Invalid attribute", fmt.Sprintf("%s only applies to deployment_type = 1 (Kubernetes)", key))
			}
		}
	}
	if resp.Diagnostics.HasError() || req.State.Raw.IsNull() {
		return
	}

	var planned, prior types.Bool
	resp.Diagnostics.Append(resp.Plan.GetAttribute(ctx, path.Root("stack_webhook"), &planned)...)
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("stack_webhook"), &prior)...)
	if !planned.IsUnknown() && planned.Equal(prior) {
		for _, key := range []string{"webhook_id", "webhook_url"} {
			var v types.String
			resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root(key), &v)...)
			resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root(key), v)...)
		}
	}
}

// edgeStackHelmConfig turns the helm_config block into the payload Portainer
// expects, or returns false when the stack is not a Helm repository one.
//
// The keys are PascalCase where the other edge stack payloads are camelCase.
// That is what the Helm endpoints accept, so the inconsistency is Portainer's
// rather than something to normalise away here.
func edgeStackHelmConfig(ctx context.Context, m *edgeStackModel, diags *diag.Diagnostics) (map[string]interface{}, bool) {
	if m.HelmConfig.IsNull() || m.HelmConfig.IsUnknown() || len(m.HelmConfig.Elements()) == 0 {
		return nil, false
	}
	var blocks []edgeStackHelmModel
	diags.Append(m.HelmConfig.ElementsAs(ctx, &blocks, false)...)
	if len(blocks) == 0 {
		return nil, false
	}
	block := blocks[0]

	config := map[string]interface{}{
		"ChartURL":  block.ChartURL.ValueString(),
		"ChartName": block.ChartName.ValueString(),
	}
	// The optional fields are omitted when empty so an update never clears a
	// value that was set outside Terraform.
	for key, v := range map[string]types.String{
		"ChartVersion": block.ChartVersion,
		"Namespace":    block.Namespace,
		"ValuesInline": block.ValuesInline,
		"Timeout":      block.Timeout,
	} {
		if v.ValueString() != "" {
			config[key] = v.ValueString()
		}
	}
	if block.Atomic.ValueBool() {
		config["Atomic"] = true
	}
	return config, true
}

// edgeStackDeployerOptions returns the DeployerOptions payload carrying the
// Kubernetes apply policies the configuration sets, or nil when it sets none.
// Nothing is sent then: Portainer keeps its recorded choice (or its default on
// create), and a Portainer older than 3.0 never sees the fields. The Docker
// options in the same object are not managed here, so it is only sent for a
// Kubernetes deployment.
func edgeStackDeployerOptions(ctx context.Context, config tfsdk.Config, m *edgeStackModel, diags *diag.Diagnostics) map[string]interface{} {
	if m.DeploymentType.ValueInt64() != 1 {
		return nil
	}
	opts := map[string]interface{}{}
	for key, field := range map[string]string{"kubernetes_prune": "KubernetesPrune", "kubernetes_force_apply": "KubernetesForceApply"} {
		var v types.Bool
		diags.Append(config.GetAttribute(ctx, path.Root(key), &v)...)
		if !v.IsNull() && !v.IsUnknown() {
			opts[field] = v.ValueBool()
		}
	}
	if len(opts) == 0 {
		return nil
	}
	return opts
}

// edgeStackEnvVars is the envVars payload of the environment map.
func edgeStackEnvVars(ctx context.Context, m *edgeStackModel, diags *diag.Diagnostics) []map[string]string {
	env := stringMapToGo(ctx, m.Environment, diags)
	envVars := make([]map[string]string, 0, len(env))
	for k, v := range env {
		envVars = append(envVars, map[string]string{"name": k, "value": v})
	}
	return envVars
}

// edgeStackRepositoryPassword is the repository password: the write-only
// twin when its version is set and it carries a value, the stored attribute
// otherwise.
func edgeStackRepositoryPassword(ctx context.Context, config tfsdk.Config, m *edgeStackModel, diags *diag.Diagnostics) string {
	if m.RepositoryPasswordWOVersion.ValueInt64() != 0 {
		if wo := writeOnlyConfigString(ctx, config, path.Root("repository_password_wo"), diags); wo != "" {
			return wo
		}
	}
	return m.RepositoryPassword.ValueString()
}

func (r *edgeStackResource) webhookURL(webhookID string) string {
	baseURL := strings.TrimSuffix(r.client.Endpoint, "/api")
	return fmt.Sprintf("%s/api/edge_stacks/webhooks/%s", baseURL, webhookID)
}

func findExistingEdgeStackByName(ctx context.Context, client *APIClient, name string) (int, error) {
	var stacks []map[string]interface{}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/edge_stacks", client.Endpoint), nil, &stacks); err != nil {
		return 0, fmt.Errorf("failed to list edge stacks: %w", err)
	}

	for _, stack := range stacks {
		if stack["Name"] == name {
			if id, ok := stack["Id"].(float64); ok {
				return int(id), nil
			}
		}
	}
	return 0, nil
}

func (r *edgeStackResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan edgeStackModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	timeout, d := plan.Timeouts.Create(ctx, 15*time.Minute)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	name := plan.Name.ValueString()
	existingID, err := findExistingEdgeStackByName(ctx, r.client, name)
	if err != nil {
		resp.Diagnostics.AddError("Failed to create edge stack", fmt.Sprintf("failed to check for existing edge stack: %s", err))
		return
	}
	if existingID != 0 {
		// A stack of that name is adopted and updated, as the SDKv2 version did.
		plan.ID = types.StringValue(strconv.Itoa(existingID))
		r.update(ctx, req.Config, &plan, &resp.State, &resp.Diagnostics)
		edgeKeepCreatedOnError(ctx, req.Plan, plan.ID.ValueString(), &resp.State, &resp.Diagnostics)
		return
	}

	id, created := r.create(ctx, req.Config, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	if !created {
		// SDKv2 stored nothing either, which Terraform reported as an error.
		resp.Diagnostics.AddError("Edge stack dry run", "dryrun = true only simulates the deployment: Portainer created no edge stack for Terraform to manage.")
		return
	}
	plan.ID = types.StringValue(strconv.Itoa(id))
	r.readInto(ctx, &plan, &resp.State, false, &resp.Diagnostics)
	edgeKeepCreatedOnError(ctx, req.Plan, plan.ID.ValueString(), &resp.State, &resp.Diagnostics)
}

// create runs the create call of the method the configuration selects and
// returns the new stack's ID; created is false for a dry run, which leaves
// nothing behind.
func (r *edgeStackResource) create(ctx context.Context, config tfsdk.Config, plan *edgeStackModel, diags *diag.Diagnostics) (id int, created bool) {
	edgeGroups := int64ListToInts(ctx, plan.EdgeGroups, diags)
	registries := int64ListToInts(ctx, plan.Registries, diags)
	if registries == nil {
		registries = []int{}
	}
	if edgeGroups == nil {
		edgeGroups = []int{}
	}
	name := plan.Name.ValueString()
	deployType := int(plan.DeploymentType.ValueInt64())
	useManifest := plan.UseManifestNamespaces.ValueBool()
	if diags.HasError() {
		return 0, false
	}

	createJSON := func(payload map[string]interface{}, endpoint string) (int, bool) {
		var result struct {
			ID int `json:"Id"`
		}
		if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+endpoint, payload, &result); err != nil {
			diags.AddError("Failed to create edge stack", fmt.Sprintf("failed to create edge stack: %s", err))
			return 0, false
		}
		return result.ID, true
	}

	// Method: Helm repository. Checked first because a Helm stack carries none
	// of the fields the other branches dispatch on.
	if helmConfig, ok := edgeStackHelmConfig(ctx, plan, diags); ok {
		payload := map[string]interface{}{
			"Name":       name,
			"EdgeGroups": edgeGroups,
			"HelmConfig": helmConfig,
		}
		if stagger := edgeStaggerPayload(ctx, plan.StaggerConfig, diags); stagger != nil {
			payload["StaggerConfig"] = stagger
		}
		return createJSON(payload, "/edge_stacks/create/helmRepo")
	}

	// Method: stackFileContent (string)
	if content := plan.StackFileContent.ValueString(); content != "" {
		payload := map[string]interface{}{
			"name":                  name,
			"deploymentType":        deployType,
			"edgeGroups":            edgeGroups,
			"stackFileContent":      content,
			"useManifestNamespaces": useManifest,
			"registries":            registries,
		}
		if opts := edgeStackDeployerOptions(ctx, config, plan, diags); opts != nil {
			payload["deployerOptions"] = opts
		}
		if len(plan.Environment.Elements()) > 0 {
			payload["envVars"] = edgeStackEnvVars(ctx, plan, diags)
		}
		if stagger := edgeStaggerPayload(ctx, plan.StaggerConfig, diags); stagger != nil {
			payload["staggerConfig"] = stagger
		}
		return createJSON(payload, "/edge_stacks/create/string")
	}

	// Method: stackFilePath (file)
	if filePath := plan.StackFilePath.ValueString(); filePath != "" {
		file, err := os.Open(filePath)
		if err != nil {
			diags.AddError("Failed to create edge stack", fmt.Sprintf("failed to open stack file: %s", err))
			return 0, false
		}
		defer file.Close()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)
		_ = writer.WriteField("Name", name)
		_ = writer.WriteField("DeploymentType", strconv.Itoa(deployType))
		_ = writer.WriteField("EdgeGroups", toJSONString(edgeGroups))
		_ = writer.WriteField("UseManifestNamespaces", strconv.FormatBool(useManifest))
		_ = writer.WriteField("Registries", toJSONString(registries))
		_ = writer.WriteField("PrePullImage", strconv.FormatBool(plan.PrePullImage.ValueBool()))
		_ = writer.WriteField("RetryDeploy", strconv.FormatBool(plan.RetryDeploy.ValueBool()))
		if opts := edgeStackDeployerOptions(ctx, config, plan, diags); opts != nil {
			_ = writer.WriteField("DeployerOptions", toJSONString(opts))
		}
		if stagger := edgeStaggerPayload(ctx, plan.StaggerConfig, diags); stagger != nil {
			_ = writer.WriteField("StaggerConfig", toJSONString(stagger))
		}
		part, err := writer.CreateFormFile("file", filepath.Base(filePath))
		if err != nil {
			diags.AddError("Failed to create edge stack", err.Error())
			return 0, false
		}
		_, _ = io.Copy(part, file)
		writer.Close()

		// Build query string for dryrun
		endpoint := fmt.Sprintf("%s/edge_stacks/create/file", r.client.Endpoint)
		if plan.Dryrun.ValueBool() {
			endpoint += "?dryrun=true"
		}
		status, data, err := edgePostMultipart(ctx, r.client, http.MethodPost, endpoint, writer.FormDataContentType(), body, false)
		if err != nil {
			diags.AddError("Failed to create edge stack", err.Error())
			return 0, false
		}
		if status < 200 || status >= 300 {
			diags.AddError("Failed to create edge stack", fmt.Sprintf("failed to create edge stack from file: %s", string(data)))
			return 0, false
		}
		var result struct {
			ID int `json:"Id"`
		}
		_ = json.Unmarshal(data, &result)
		// A dry run deploys nothing, so there is nothing to keep in state.
		return result.ID, !plan.Dryrun.ValueBool()
	}

	// Method: repository (an inline URL or a Portainer Source)
	repoURL, sourceID := plan.RepositoryURL.ValueString(), int(plan.SourceID.ValueInt64())
	if repoURL != "" || sourceID != 0 {
		payload := map[string]interface{}{
			"name":                      name,
			"deploymentType":            deployType,
			"edgeGroups":                edgeGroups,
			"repositoryURL":             repoURL,
			"repositoryAuthentication":  plan.GitRepositoryAuthentication.ValueBool(),
			"repositoryUsername":        plan.RepositoryUsername.ValueString(),
			"repositoryPassword":        edgeStackRepositoryPassword(ctx, config, plan, diags),
			"repositoryReferenceName":   plan.RepositoryReferenceName.ValueString(),
			"repositoryGitCredentialID": int(plan.RepositoryGitCredentialID.ValueInt64()),
			"filePathInRepository":      plan.FilePathInRepository.ValueString(),
			"useManifestNamespaces":     useManifest,
			"registries":                registries,
		}
		if sourceID != 0 {
			payload["sourceID"] = sourceID
		}
		if opts := edgeStackDeployerOptions(ctx, config, plan, diags); opts != nil {
			payload["deployerOptions"] = opts
		}
		if relPath := plan.RelativePath.ValueString(); relPath != "" {
			payload["supportRelativePath"] = true
			payload["filesystemPath"] = relPath
			payload["AlwaysCloneGitRepoForRelativePath"] = plan.AlwaysClone.ValueBool()
		}
		if p := plan.PerDeviceConfigsPath.ValueString(); p != "" {
			payload["supportPerDeviceConfigs"] = true
			payload["perDeviceConfigsPath"] = p
			if v := plan.PerDeviceConfigsMatchType.ValueString(); v != "" {
				payload["perDeviceConfigsMatchType"] = v
			}
			if v := plan.PerDeviceConfigsGroupMatch.ValueString(); v != "" {
				payload["perDeviceConfigsGroupMatchType"] = v
			}
		}
		if len(plan.Environment.Elements()) > 0 {
			payload["envVars"] = edgeStackEnvVars(ctx, plan, diags)
		}
		if stagger := edgeStaggerPayload(ctx, plan.StaggerConfig, diags); stagger != nil {
			payload["staggerConfig"] = stagger
		}

		stackWebhook := plan.StackWebhook.ValueBool()
		if stackWebhook || plan.UpdateInterval.ValueString() != "" {
			webhookID := ""
			if stackWebhook {
				webhookID = uuid.New().String()
			}
			payload["autoUpdate"] = map[string]interface{}{
				"forcePullImage": plan.PullImage.ValueBool(),
				"forceUpdate":    plan.ForceUpdate.ValueBool(),
				"interval":       plan.UpdateInterval.ValueString(),
				"webhook":        webhookID,
			}
		}
		if diags.HasError() {
			return 0, false
		}
		return createJSON(payload, "/edge_stacks/create/repository")
	}

	diags.AddError("Failed to create edge stack", "one of 'stack_file_content', 'stack_file_path', 'repository_url', 'source_id', or 'helm_config' must be provided")
	return 0, false
}

func (r *edgeStackResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan, state edgeStackModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	timeout, d := plan.Timeouts.Update(ctx, 15*time.Minute)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// The webhook in state is kept: a new ID on every update broke every CI
	// job that had been given the old URL.
	if plan.WebhookID.IsUnknown() {
		plan.WebhookID = state.WebhookID
	}
	r.update(ctx, req.Config, &plan, &resp.State, &resp.Diagnostics)
}

// update runs the update call of the method the configuration selects, then
// reads the stack back into state.
func (r *edgeStackResource) update(ctx context.Context, config tfsdk.Config, plan *edgeStackModel, state *tfsdk.State, diags *diag.Diagnostics) {
	id := plan.ID.ValueString()
	deploymentType := int(plan.DeploymentType.ValueInt64())
	edgeGroups := int64ListToInts(ctx, plan.EdgeGroups, diags)
	if edgeGroups == nil {
		edgeGroups = []int{}
	}
	registries := int64ListToInts(ctx, plan.Registries, diags)
	if registries == nil {
		registries = []int{}
	}
	if diags.HasError() {
		return
	}

	helmConfig, isHelm := edgeStackHelmConfig(ctx, plan, diags)
	switch {
	case isHelm:
		// A Helm stack is updated through its own endpoint, which takes the
		// chart configuration rather than a stack file.
		payload := map[string]interface{}{
			"EdgeGroups": edgeGroups,
			"HelmConfig": helmConfig,
		}
		if stagger := edgeStaggerPayload(ctx, plan.StaggerConfig, diags); stagger != nil {
			payload["StaggerConfig"] = stagger
		}
		url := fmt.Sprintf("%s/edge_stacks/%s/helmRepo", r.client.Endpoint, id)
		if err := doJSON(ctx, r.client, http.MethodPut, url, payload, nil); err != nil {
			diags.AddError("Failed to update edge stack", fmt.Sprintf("failed to update edge stack %s from its Helm repository: %s", id, err))
			return
		}

	case plan.StackFileContent.ValueString() != "" || plan.StackFilePath.ValueString() != "":
		payload := map[string]interface{}{
			"name":                  plan.Name.ValueString(),
			"deploymentType":        deploymentType,
			"edgeGroups":            edgeGroups,
			"updateVersion":         true,
			"useManifestNamespaces": plan.UseManifestNamespaces.ValueBool(),
			"envVars":               edgeStackEnvVars(ctx, plan, diags),
			"prePullImage":          plan.PrePullImage.ValueBool(),
			"rePullImage":           plan.PullImage.ValueBool(),
			"registries":            registries,
		}
		if v := plan.StackFileContent.ValueString(); v != "" {
			payload["stackFileContent"] = v
		}
		edgeStackAddRepull(plan, payload)
		if stagger := edgeStaggerPayload(ctx, plan.StaggerConfig, diags); stagger != nil {
			payload["staggerConfig"] = stagger
		}
		if opts := edgeStackDeployerOptions(ctx, config, plan, diags); opts != nil {
			payload["deployerOptions"] = opts
		}
		if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/edge_stacks/%s", r.client.Endpoint, id), payload, nil); err != nil {
			diags.AddError("Failed to update edge stack", fmt.Sprintf("failed to update edge stack: %s", err))
			return
		}

	// Repository-based update via /git
	// Portainer's PUT /edge_stacks/{id}/git endpoint silently ignores any
	// `repositoryURL` / `filePathInRepository` fields in the payload — only
	// `refName` and the authentication block are honored. Changing the source
	// URL or in-repo file path requires recreating the stack (enforced via
	// RequiresReplace on the matching schema attributes).
	case plan.RepositoryURL.ValueString() != "" || plan.SourceID.ValueInt64() != 0:
		payload := map[string]interface{}{
			"deploymentType": deploymentType,
			"groupIds":       edgeGroups,
			"refName":        plan.RepositoryReferenceName.ValueString(),
			"envVars":        edgeStackEnvVars(ctx, plan, diags),
			"updateVersion":  true,
			"prePullImage":   plan.PrePullImage.ValueBool(),
			"rePullImage":    plan.PullImage.ValueBool(),
			"registries":     registries,
			"retryDeploy":    plan.RetryDeploy.ValueBool(),
		}
		if opts := edgeStackDeployerOptions(ctx, config, plan, diags); opts != nil {
			payload["deployerOptions"] = opts
		}
		if relPath := plan.RelativePath.ValueString(); relPath != "" {
			payload["supportRelativePath"] = true
			payload["filesystemPath"] = relPath
			payload["AlwaysCloneGitRepoForRelativePath"] = plan.AlwaysClone.ValueBool()
			// The git update endpoint ignores the top-level fields above (kept
			// for older servers) and reads always_clone from RelativePath.
			payload["relativePath"] = map[string]interface{}{"AlwaysCloneGitRepoForRelativePath": plan.AlwaysClone.ValueBool()}
		}
		edgeStackAddRepull(plan, payload)
		if stagger := edgeStaggerPayload(ctx, plan.StaggerConfig, diags); stagger != nil {
			payload["staggerConfig"] = stagger
		}
		if plan.GitRepositoryAuthentication.ValueBool() {
			payload["authentication"] = map[string]interface{}{
				"username":        plan.RepositoryUsername.ValueString(),
				"password":        edgeStackRepositoryPassword(ctx, config, plan, diags),
				"gitCredentialID": int(plan.RepositoryGitCredentialID.ValueInt64()),
			}
		}
		if plan.StackWebhook.ValueBool() || plan.UpdateInterval.ValueString() != "" {
			// Keep the existing webhook: a new ID on every update broke every
			// CI job that had been given the old URL.
			webhookID := ""
			if plan.StackWebhook.ValueBool() {
				webhookID = plan.WebhookID.ValueString()
				if plan.WebhookID.IsUnknown() || webhookID == "" {
					webhookID = uuid.New().String()
				}
			}
			payload["autoUpdate"] = map[string]interface{}{
				"forcePullImage": plan.PullImage.ValueBool(),
				"forceUpdate":    plan.ForceUpdate.ValueBool(),
				"interval":       plan.UpdateInterval.ValueString(),
				"webhook":        webhookID,
			}
		}
		if diags.HasError() {
			return
		}
		if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/edge_stacks/%s/git", r.client.Endpoint, id), payload, nil); err != nil {
			diags.AddError("Failed to update edge stack", fmt.Sprintf("failed to update repository-based edge stack: %s", err))
			return
		}

	default:
		diags.AddError("Failed to update edge stack", "one of 'stack_file_content', 'stack_file_path', 'repository_url', 'source_id', or 'helm_config' must be provided for update")
		return
	}

	r.readInto(ctx, plan, state, false, diags)
}

// edgeStackAddRepull adds RepullImageAndRedeploy, which replaces the
// rePullImage flag Portainer deprecated in 2.36, when pull_image is on.
// rePullImage stays for older servers; false is not sent, the same as the
// deprecated flag left at its default.
func edgeStackAddRepull(plan *edgeStackModel, payload map[string]interface{}) {
	if plan.PullImage.ValueBool() {
		payload["repullImageAndRedeploy"] = true
	}
}

func toJSONString(input interface{}) string {
	data, _ := json.Marshal(input)
	return string(data)
}

func (r *edgeStackResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	fwSetIDIdentity(ctx, req.State, resp.Identity, &resp.Diagnostics)
	var state edgeStackModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	// An import reads every argument Portainer reports, including the ones
	// later reads only refresh while configured (per-device configurations,
	// stagger_config): there is no configuration yet to tell them apart.
	imported, d := req.Private.GetKey(ctx, edgeStackImportedKey)
	resp.Diagnostics.Append(d...)
	r.readInto(ctx, &state, &resp.State, len(imported) > 0, &resp.Diagnostics)
	if len(imported) > 0 {
		resp.Diagnostics.Append(resp.Private.SetKey(ctx, edgeStackImportedKey, nil)...)
	}
}

// edgeStackImportedKey marks, in private state, an edge stack imported and
// not read yet.
const edgeStackImportedKey = "imported"

// edgeStaggerIsDefault reports the stagger configuration Business Edition
// records for every stack that sets none: all at once, nothing else set.
func edgeStaggerIsDefault(api *edgeStaggerAPI) bool {
	return api == nil || ((api.StaggerOption == 0 || api.StaggerOption == 1) && *api == edgeStaggerAPI{StaggerOption: api.StaggerOption})
}

// edgeStaggerImported is the stagger_config block an import reads: none for
// Portainer's default (see edgeStaggerIsDefault), so a configuration without
// the block matches, else the reported one with zero values left unset.
func edgeStaggerImported(ctx context.Context, api *edgeStaggerAPI, diags *diag.Diagnostics) types.List {
	objType := types.ObjectType{AttrTypes: edgeStaggerAttrTypes}
	if edgeStaggerIsDefault(api) {
		return types.ListValueMust(objType, []attr.Value{})
	}
	prior, d := types.ObjectValue(edgeStaggerAttrTypes, map[string]attr.Value{
		"stagger_option": types.StringNull(), "parallel_option": types.StringNull(),
		"device_number": types.Int64Null(), "device_number_start_from": types.Int64Null(), "device_number_increment_by": types.Int64Null(),
		"timeout": types.StringNull(), "update_delay": types.StringNull(), "update_failure_action": types.StringNull(),
	})
	diags.Append(d...)
	return edgeStaggerFromAPI(ctx, types.ListValueMust(objType, []attr.Value{prior}), api, diags)
}

// edgeStackAPI is what GET /edge_stacks/{id} is read from.
type edgeStackAPI struct {
	Name                              string `json:"Name"`
	EdgeGroups                        []int  `json:"EdgeGroups"`
	DeploymentType                    int    `json:"DeploymentType"`
	UseManifestNamespaces             bool   `json:"UseManifestNamespaces"`
	Registries                        []int  `json:"Registries"`
	PrePullImage                      bool   `json:"PrePullImage"`
	RePullImage                       bool   `json:"RePullImage"`
	RetryDeploy                       bool   `json:"RetryDeploy"`
	SupportRelativePath               bool   `json:"SupportRelativePath"`
	FilesystemPath                    string `json:"FilesystemPath"`
	AlwaysCloneGitRepoForRelativePath bool   `json:"AlwaysCloneGitRepoForRelativePath"`
	SupportPerDeviceConfigs           bool   `json:"SupportPerDeviceConfigs"`
	PerDeviceConfigsPath              string `json:"PerDeviceConfigsPath"`
	PerDeviceConfigsMatchType         string `json:"PerDeviceConfigsMatchType"`
	PerDeviceConfigsGroupMatchType    string `json:"PerDeviceConfigsGroupMatchType"`
	// Business Edition only.
	StaggerConfig *edgeStaggerAPI `json:"StaggerConfig"`
	EnvVars       []struct {
		Name  string `json:"name"`
		Value string `json:"value"`
	} `json:"EnvVars"`
	GitConfig *struct {
		URL            string `json:"URL"`
		ReferenceName  string `json:"ReferenceName"`
		ConfigFilePath string `json:"ConfigFilePath"`
		Authentication *struct {
			Username        string `json:"Username"`
			GitCredentialID int    `json:"GitCredentialID"`
		} `json:"Authentication"`
	} `json:"GitConfig"`
	AutoUpdate *struct {
		Interval       string `json:"Interval"`
		Webhook        string `json:"Webhook"`
		ForcePullImage bool   `json:"ForcePullImage"`
		ForceUpdate    bool   `json:"ForceUpdate"`
	} `json:"AutoUpdate,omitempty"`
	// Portainer 3.0 and later. Older versions leave it out, and a nil
	// Prune means pruning is on, so neither is read back unless returned.
	KubernetesOptions *struct {
		Prune      *bool `json:"Prune"`
		ForceApply *bool `json:"ForceApply"`
	} `json:"KubernetesOptions"`
	HelmConfig *struct {
		ChartURL     string `json:"ChartURL"`
		ChartName    string `json:"ChartName"`
		ChartVersion string `json:"ChartVersion"`
		Namespace    string `json:"Namespace"`
		ValuesInline string `json:"ValuesInline"`
		Atomic       bool   `json:"Atomic"`
		Timeout      string `json:"Timeout"`
	} `json:"HelmConfig"`
}

// readInto refreshes m from Portainer. imported is set for the read right
// after an import, which also reads what is otherwise read only while
// configured.
func (r *edgeStackResource) readInto(ctx context.Context, m *edgeStackModel, state *tfsdk.State, imported bool, diags *diag.Diagnostics) {
	var stack edgeStackAPI
	if err := doJSON(ctx, r.client, http.MethodGet, fmt.Sprintf("%s/edge_stacks/%s", r.client.Endpoint, m.ID.ValueString()), nil, &stack); err != nil {
		if isAPINotFound(err) {
			state.RemoveResource(ctx)
			return
		}
		diags.AddError("Failed to read edge stack", fmt.Sprintf("failed to read edge stack: %s", err))
		return
	}

	m.Name = types.StringValue(stack.Name)
	m.DeploymentType = types.Int64Value(int64(stack.DeploymentType))
	m.Dryrun = types.BoolValue(false) // write-only creation flag, API never returns it
	m.EdgeGroups = int64ListValue(stack.EdgeGroups)
	m.Registries = edgeIntListFromAPI(m.Registries, stack.Registries)
	m.UseManifestNamespaces = types.BoolValue(stack.UseManifestNamespaces)
	m.PrePullImage = types.BoolValue(stack.PrePullImage)
	m.RetryDeploy = types.BoolValue(stack.RetryDeploy)
	m.AlwaysClone = types.BoolValue(stack.AlwaysCloneGitRepoForRelativePath)

	if len(stack.EnvVars) > 0 {
		envMap := make(map[string]string, len(stack.EnvVars))
		for _, env := range stack.EnvVars {
			envMap[env.Name] = env.Value
		}
		m.Environment = stringMapValue(envMap)
	}

	if stack.DeploymentType == 1 && stack.KubernetesOptions != nil {
		if stack.KubernetesOptions.Prune != nil {
			m.KubernetesPrune = types.BoolValue(*stack.KubernetesOptions.Prune)
		}
		if stack.KubernetesOptions.ForceApply != nil {
			m.KubernetesForceApply = types.BoolValue(*stack.KubernetesOptions.ForceApply)
		}
	}
	// Nothing to read back: Portainer has no recorded choice to report.
	if m.KubernetesPrune.IsUnknown() {
		m.KubernetesPrune = types.BoolNull()
	}
	if m.KubernetesForceApply.IsUnknown() {
		m.KubernetesForceApply = types.BoolNull()
	}

	if stack.SupportRelativePath {
		m.RelativePath = types.StringValue(stack.FilesystemPath)
	}
	// Only read back when configured or right after an import: the attributes
	// force a replacement, so a stack adopted (created under an existing name)
	// with per-device configurations set outside Terraform must not be
	// planned for destruction by an upgrade. A server without them (Community
	// Edition) reports none; the configured values stay then.
	if stack.SupportPerDeviceConfigs && (imported || !m.PerDeviceConfigsPath.IsNull()) {
		m.PerDeviceConfigsPath = types.StringValue(stack.PerDeviceConfigsPath)
		m.PerDeviceConfigsMatchType = edgeStringFromAPI(m.PerDeviceConfigsMatchType, stack.PerDeviceConfigsMatchType)
		m.PerDeviceConfigsGroupMatch = edgeStringFromAPI(m.PerDeviceConfigsGroupMatch, stack.PerDeviceConfigsGroupMatchType)
	}
	if imported {
		m.StaggerConfig = edgeStaggerImported(ctx, stack.StaggerConfig, diags)
	} else {
		m.StaggerConfig = edgeStaggerFromAPI(ctx, m.StaggerConfig, stack.StaggerConfig, diags)
	}

	// Portainer also fills HelmConfig for a git-backed stack that deploys a
	// chart from the cloned repository. ChartURL is what distinguishes a Helm
	// repository stack, so reading the block back is gated on it - otherwise a
	// git stack would grow a helm_config it never declared.
	if stack.HelmConfig != nil && stack.HelmConfig.ChartURL != "" {
		var prior edgeStackHelmModel
		if !m.HelmConfig.IsNull() && !m.HelmConfig.IsUnknown() && len(m.HelmConfig.Elements()) > 0 {
			var blocks []edgeStackHelmModel
			diags.Append(m.HelmConfig.ElementsAs(ctx, &blocks, false)...)
			prior = blocks[0]
		} else {
			prior = edgeStackHelmModel{
				ChartVersion: types.StringNull(), Namespace: types.StringNull(), ValuesInline: types.StringNull(),
				Atomic: types.BoolNull(), Timeout: types.StringNull(),
			}
		}
		atomic := types.BoolValue(stack.HelmConfig.Atomic)
		if !stack.HelmConfig.Atomic && prior.Atomic.IsNull() {
			atomic = types.BoolNull()
		}
		obj, d := types.ObjectValue(edgeStackHelmAttrTypes, map[string]attr.Value{
			"chart_url":     types.StringValue(stack.HelmConfig.ChartURL),
			"chart_name":    types.StringValue(stack.HelmConfig.ChartName),
			"chart_version": edgeStringFromAPI(prior.ChartVersion, stack.HelmConfig.ChartVersion),
			"namespace":     edgeStringFromAPI(prior.Namespace, stack.HelmConfig.Namespace),
			"values_inline": edgeStringFromAPI(prior.ValuesInline, stack.HelmConfig.ValuesInline),
			"atomic":        atomic,
			"timeout":       edgeStringFromAPI(prior.Timeout, stack.HelmConfig.Timeout),
		})
		diags.Append(d...)
		m.HelmConfig = types.ListValueMust(types.ObjectType{AttrTypes: edgeStackHelmAttrTypes}, []attr.Value{obj})
	} else if m.HelmConfig.IsNull() || m.HelmConfig.IsUnknown() {
		m.HelmConfig = types.ListValueMust(types.ObjectType{AttrTypes: edgeStackHelmAttrTypes}, []attr.Value{})
	}

	if stack.GitConfig != nil {
		m.RepositoryURL = edgeStringFromAPI(m.RepositoryURL, stack.GitConfig.URL)
		m.RepositoryReferenceName = types.StringValue(stack.GitConfig.ReferenceName)
		m.FilePathInRepository = types.StringValue(stack.GitConfig.ConfigFilePath)
		if stack.GitConfig.Authentication != nil {
			m.GitRepositoryAuthentication = types.BoolValue(true)
			m.RepositoryUsername = edgeStringFromAPI(m.RepositoryUsername, stack.GitConfig.Authentication.Username)
			m.RepositoryGitCredentialID = edgeInt64FromAPI(m.RepositoryGitCredentialID, int64(stack.GitConfig.Authentication.GitCredentialID))
		} else {
			m.GitRepositoryAuthentication = types.BoolValue(false)
		}
	}

	// pull_image is sent to two different places in Create/Update payloads:
	// top-level rePullImage (always) and autoUpdate.forcePullImage (only when
	// GitOps is configured). The API persists both. Use top-level RePullImage
	// as the source of truth, and let AutoUpdate.ForcePullImage override when
	// AutoUpdate exists since that's the value the user-configured GitOps block
	// carries.
	//
	// Without GitOps, Portainer only records pull_image through an update:
	// none of its create endpoints (string, file, repository, helmRepo) has a
	// re-pull field, only PrePullImage. A stack created with pull_image = true
	// therefore reports false until its first update; the configured true is
	// kept then instead of failing the create with an inconsistent result or
	// planning an update (and redeploy) only to record the flag.
	if !stack.RePullImage && m.PullImage.ValueBool() && stack.AutoUpdate == nil {
		m.PullImage = types.BoolValue(true)
	} else {
		m.PullImage = types.BoolValue(stack.RePullImage)
	}

	if stack.AutoUpdate != nil {
		m.UpdateInterval = edgeStringFromAPI(m.UpdateInterval, stack.AutoUpdate.Interval)
		m.PullImage = types.BoolValue(stack.AutoUpdate.ForcePullImage)
		m.ForceUpdate = types.BoolValue(stack.AutoUpdate.ForceUpdate)
		if stack.AutoUpdate.Webhook != "" {
			m.StackWebhook = types.BoolValue(true)
			m.WebhookID = types.StringValue(stack.AutoUpdate.Webhook)
			m.WebhookURL = types.StringValue(r.webhookURL(stack.AutoUpdate.Webhook))
		} else {
			// AutoUpdate exists but webhook was cleared — drop any stale
			// computed outputs so state reflects reality.
			m.StackWebhook = types.BoolValue(false)
			m.WebhookID = types.StringValue("")
			m.WebhookURL = types.StringValue("")
		}
	} else {
		// No GitOps configured — clear all AutoUpdate-derived fields,
		// including webhook outputs that may have been set previously.
		m.ForceUpdate = types.BoolValue(false)
		m.StackWebhook = types.BoolValue(false)
		m.UpdateInterval = edgeStringFromAPI(m.UpdateInterval, "")
		m.WebhookID = types.StringValue("")
		m.WebhookURL = types.StringValue("")
	}

	// What the API did not report stays unset.
	for _, v := range []*types.String{&m.RepositoryURL, &m.RepositoryUsername, &m.UpdateInterval} {
		if v.IsUnknown() {
			*v = types.StringNull()
		}
	}
	if m.RelativePath.IsUnknown() {
		m.RelativePath = types.StringValue("")
	}
	if m.RepositoryGitCredentialID.IsUnknown() {
		m.RepositoryGitCredentialID = types.Int64Null()
	}

	diags.Append(state.Set(ctx, m)...)
}

func (r *edgeStackResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state edgeStackModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	timeout, d := state.Timeouts.Delete(ctx, 10*time.Minute)
	resp.Diagnostics.Append(d...)
	if resp.Diagnostics.HasError() {
		return
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := doJSON(ctx, r.client, http.MethodDelete, fmt.Sprintf("%s/edge_stacks/%s", r.client.Endpoint, state.ID.ValueString()), nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete edge stack", fmt.Sprintf("failed to delete edge stack: %s", err))
	}
}

// ImportState takes the edge stack ID (or an identity with it); the read
// that follows fills every argument Portainer reports (see Read).
func (r *edgeStackResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	fwImportIDOrIdentity(ctx, req, resp)
	resp.Diagnostics.Append(resp.Private.SetKey(ctx, edgeStackImportedKey, []byte(`true`))...)
}
