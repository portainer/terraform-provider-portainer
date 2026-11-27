package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework-validators/listvalidator"
	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// portainer_gitops_workflow manages a GitOps workflow: a named set of
// artifacts, each deployed from files in a GitOps source onto a set of edge
// groups.
//
// Portainer offers two ways to remove one, which is why `on_destroy` exists:
// `destroy` tears the deployed artifacts down with the workflow, `detach`
// leaves them running and only unlinks them from GitOps.
//
// Portainer 3.0 added a second engine. `reconcile` keeps Edge Kubernetes
// environments continuously matching the source instead of deploying Edge
// stacks; a workflow can be migrated to it in place, but never back.

var (
	_ resource.Resource                = (*gitopsWorkflowResource)(nil)
	_ resource.ResourceWithConfigure   = (*gitopsWorkflowResource)(nil)
	_ resource.ResourceWithImportState = (*gitopsWorkflowResource)(nil)
	_ resource.ResourceWithModifyPlan  = (*gitopsWorkflowResource)(nil)
)

func newGitopsWorkflowResource() resource.Resource { return &gitopsWorkflowResource{} }

type gitopsWorkflowResource struct {
	fwIDIdentity
	client *APIClient
}

type gitopsWorkflowResModel struct {
	ID                types.String `tfsdk:"id"`
	Name              types.String `tfsdk:"name"`
	Engine            types.String `tfsdk:"engine"`
	ReconcileInterval types.String `tfsdk:"reconcile_interval"`
	OnDestroy         types.String `tfsdk:"on_destroy"`
	Artifact          types.List   `tfsdk:"artifact"`
	WorkflowID        types.Int64  `tfsdk:"workflow_id"`
}

type gitopsArtifactResModel struct {
	Name                types.String `tfsdk:"name"`
	Type                types.String `tfsdk:"type"`
	DeploymentType      types.String `tfsdk:"deployment_type"`
	EdgeGroupIDs        types.List   `tfsdk:"edge_group_ids"`
	EnvironmentIDs      types.List   `tfsdk:"environment_ids"`
	EnvironmentGroupIDs types.List   `tfsdk:"environment_group_ids"`
	Dependencies        types.List   `tfsdk:"dependencies"`
	Helm                types.List   `tfsdk:"helm"`
	File                types.List   `tfsdk:"file"`
	Config              types.List   `tfsdk:"config"`
	ArtifactID          types.Int64  `tfsdk:"artifact_id"`
}

type gitopsHelmResModel struct {
	ChartPath   types.String `tfsdk:"chart_path"`
	ReleaseName types.String `tfsdk:"release_name"`
	Namespace   types.String `tfsdk:"namespace"`
	ValuesFiles types.List   `tfsdk:"values_files"`
	Install     types.List   `tfsdk:"install"`
	Upgrade     types.List   `tfsdk:"upgrade"`
}

type gitopsRemediationResModel struct {
	Retries              types.Int64  `tfsdk:"retries"`
	RemediateLastFailure types.Bool   `tfsdk:"remediate_last_failure"`
	Strategy             types.String `tfsdk:"strategy"`
}

type gitopsFileResModel struct {
	SourceID types.Int64  `tfsdk:"source_id"`
	Path     types.String `tfsdk:"path"`
	Ref      types.String `tfsdk:"ref"`
}

type gitopsArtifactConfigResModel struct {
	Environment                    types.Map    `tfsdk:"environment"`
	RegistryIDs                    types.List   `tfsdk:"registry_ids"`
	PrePullImage                   types.Bool   `tfsdk:"pre_pull_image"`
	RetryPeriod                    types.Int64  `tfsdk:"retry_period"`
	UseManifestNamespaces          types.Bool   `tfsdk:"use_manifest_namespaces"`
	AlwaysCloneGitRepo             types.Bool   `tfsdk:"always_clone_git_repo"`
	LocalFilesystemPath            types.String `tfsdk:"local_filesystem_path"`
	PerDeviceConfigsPath           types.String `tfsdk:"per_device_configs_path"`
	PerDeviceConfigsMatchType      types.String `tfsdk:"per_device_configs_match_type"`
	PerDeviceConfigsGroupMatchType types.String `tfsdk:"per_device_configs_group_match_type"`
	KubernetesPrune                types.Bool   `tfsdk:"kubernetes_prune"`
	KubernetesForceApply           types.Bool   `tfsdk:"kubernetes_force_apply"`
	DriftMode                      types.String `tfsdk:"drift_mode"`
	IgnorePaths                    types.List   `tfsdk:"ignore_paths"`
	Timeout                        types.String `tfsdk:"timeout"`
	Parallel                       types.List   `tfsdk:"parallel"`
}

type gitopsParallelResModel struct {
	BatchCount       types.Int64  `tfsdk:"batch_count"`
	BatchIncrementBy types.Int64  `tfsdk:"batch_increment_by"`
	Delay            types.String `tfsdk:"delay"`
	Timeout          types.String `tfsdk:"timeout"`
	FailureAction    types.String `tfsdk:"failure_action"`
}

func (r *gitopsWorkflowResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_gitops_workflow"
}

func (r *gitopsWorkflowResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	fileOrDir := []validator.String{stringvalidator.OneOf("file", "dir")}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"name": schema.StringAttribute{
				Required:    true,
				Description: "Name of the workflow.",
			},
			"engine": schema.StringAttribute{
				Optional: true,
				// Computed so an unset engine follows whatever the server
				// runs, and so an imported reconcile workflow does not plan a
				// replacement: Portainer has no engine field to read, it is
				// derived from the artifact types.
				Computed:      true,
				Validators:    []validator.String{stringvalidator.OneOf("edgeStack", "reconcile")},
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
				Description:   "Engine that deploys the artifacts and keeps them in step with their source: `edgeStack` deploys each artifact as an Edge stack, `reconcile` continuously reconciles Edge Kubernetes environments against the source and reports or corrects drift. Left unset, Portainer uses `edgeStack`. Changing `edgeStack` to `reconcile` migrates the workflow in place: Portainer removes its Edge stacks and adopts what they deployed. The migration is one-way, so changing `reconcile` to anything else forces a new resource. Requires Portainer Business Edition 3.0 or later.",
			},
			"reconcile_interval": schema.StringAttribute{
				Optional:    true,
				Validators:  []validator.String{durationValidator{}},
				Description: "How often every artifact reconciles even when its source has not moved, which is what makes drift detectable, for example `5m`. Required when `engine` is set to `reconcile`, rejected for any other engine. Removing it leaves the recorded interval alone. Requires Portainer Business Edition 3.0 or later.",
			},
			"on_destroy": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Default:     stringdefault.StaticString("destroy"),
				Validators:  []validator.String{stringvalidator.OneOf("destroy", "detach")},
				Description: "What happens to the deployed artifacts when the workflow is destroyed: `destroy` tears them down, `detach` leaves them running and only unlinks them from GitOps. Defaults to `destroy`.",
			},
			"workflow_id": schema.Int64Attribute{
				Computed:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
				Description:   "Identifier of the workflow.",
			},
		},
		Blocks: map[string]schema.Block{
			"artifact": schema.ListNestedBlock{
				Description: "The artifacts the workflow deploys. Repeatable.",
				Validators:  []validator.List{listvalidator.IsRequired(), listvalidator.SizeAtLeast(1)},
				NestedObject: schema.NestedBlockObject{
					Attributes: map[string]schema.Attribute{
						"name": schema.StringAttribute{
							Required: true,
							// Portainer's update payload has no name field at
							// all, so an artifact cannot be renamed in place.
							PlanModifiers: []planmodifier.String{stringplanmodifier.RequiresReplace()},
							Description:   "Name of the artifact. Portainer cannot rename an artifact, so changing it forces a new resource.",
						},
						"type": schema.StringAttribute{
							Optional:    true,
							Computed:    true,
							Default:     stringdefault.StaticString("edgeStack"),
							Validators:  []validator.String{stringvalidator.OneOf("stack", "edgeStack")},
							Description: "What the artifact deploys as: `stack` or `edgeStack`. Defaults to `edgeStack`. Ignored on the `reconcile` engine, which deploys every artifact as a reconciled deployment.",
						},
						"deployment_type": schema.StringAttribute{
							Required:    true,
							Description: "Deployment type of the artifact, for example `compose` or `kubernetes`.",
						},
						"edge_group_ids": schema.ListAttribute{
							Optional:    true,
							ElementType: types.Int64Type,
							Description: "Edge groups the artifact is deployed to. Every artifact needs at least one target: `edge_group_ids`, `environment_ids` or `environment_group_ids`. The `edgeStack` engine only deploys to Edge groups.",
						},
						"environment_ids": schema.ListAttribute{
							Optional:    true,
							ElementType: types.Int64Type,
							Description: "Environments the artifact is deployed to directly, rather than through a group. `reconcile` engine only. Requires Portainer Business Edition 3.0 or later.",
						},
						"environment_group_ids": schema.ListAttribute{
							Optional:    true,
							ElementType: types.Int64Type,
							Description: "Environment groups the artifact is deployed to: every supported environment of each group, including ones added to it later. `reconcile` engine only. Requires Portainer Business Edition 3.0 or later.",
						},
						"dependencies": schema.ListAttribute{
							Optional:    true,
							ElementType: types.StringType,
							Description: "Names of other artifacts in this workflow that must be deployed before this one. Requires Portainer Business Edition 3.0 or later.",
						},
						"artifact_id": schema.Int64Attribute{
							Computed:      true,
							PlanModifiers: []planmodifier.Int64{int64planmodifier.UseStateForUnknown()},
							Description:   "Identifier Portainer assigned to the artifact, which the update call needs. On the `reconcile` engine it identifies the reconciled deployment, as `portainer_gitops_workflow_deployment_resources` takes it.",
						},
					},
					Blocks: map[string]schema.Block{
						"helm": schema.ListNestedBlock{
							Description: "Deploys the artifact as a Helm chart rather than as plain manifests. The artifact's `file` paths must point at the chart directory. `reconcile` engine only. Requires Portainer Business Edition 3.0 or later.",
							Validators:  []validator.List{listvalidator.SizeAtMost(1)},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"chart_path": schema.StringAttribute{
										Required:    true,
										Description: "Directory of the chart within the artifact's sources, for example `charts/my-app`.",
									},
									"release_name": schema.StringAttribute{
										Optional:    true,
										Description: "Name of the Helm release. Portainer generates one when it is left out.",
									},
									"namespace": schema.StringAttribute{
										Optional:    true,
										Description: "Namespace the release is installed into. Left out, Helm uses the cluster's default namespace.",
									},
									"values_files": schema.ListAttribute{
										Optional:    true,
										ElementType: types.StringType,
										Description: "Values file paths within the artifact's sources, merged over the chart's own `values.yaml` in the order given.",
									},
								},
								Blocks: map[string]schema.Block{
									"install": gitopsHelmRemediationBlock("the first install of the release fails", "removing the release it could not complete (`uninstall`)"),
									"upgrade": gitopsHelmRemediationBlock("an upgrade of the release fails", "rolling back (`rollback`)"),
								},
							},
						},
						"file": schema.ListNestedBlock{
							Description: "Files in a GitOps source the artifact is built from. Repeatable.",
							Validators:  []validator.List{listvalidator.IsRequired(), listvalidator.SizeAtLeast(1)},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"source_id": schema.Int64Attribute{
										Required:    true,
										Description: "Identifier of the GitOps source the file comes from.",
									},
									"path": schema.StringAttribute{
										Required:    true,
										Description: "Path of the file within the source, for example `portainer.yaml`.",
									},
									"ref": schema.StringAttribute{
										Required:    true,
										Description: "Git reference to read the file at, for example `refs/heads/main`.",
									},
								},
							},
						},
						"config": schema.ListNestedBlock{
							Description: "Deployment options for the artifact.",
							Validators:  []validator.List{listvalidator.SizeAtMost(1)},
							NestedObject: schema.NestedBlockObject{
								Attributes: map[string]schema.Attribute{
									"environment": schema.MapAttribute{
										Optional:    true,
										ElementType: types.StringType,
										Description: "Environment variables injected into the deployment.",
									},
									"registry_ids": schema.ListAttribute{
										Optional:    true,
										ElementType: types.Int64Type,
										Description: "Registries the deployment pulls images from.",
									},
									"pre_pull_image": schema.BoolAttribute{
										Optional:    true,
										Description: "Whether agents pull the images before deploying.",
									},
									"retry_period": schema.Int64Attribute{
										Optional:    true,
										Description: "How long an agent keeps retrying a failed deployment, in seconds.",
									},
									"use_manifest_namespaces": schema.BoolAttribute{
										Optional:    true,
										Description: "Whether to use the namespaces in the manifest rather than the default one.",
									},
									"always_clone_git_repo": schema.BoolAttribute{
										Optional:    true,
										Description: "Whether the agent always clones the git repository for relative paths.",
									},
									"local_filesystem_path": schema.StringAttribute{
										Optional:    true,
										Description: "Path on the agent used for relative path volumes.",
									},
									"per_device_configs_path": schema.StringAttribute{
										Optional:    true,
										Description: "Path within the repository holding per-device configurations.",
									},
									"per_device_configs_match_type": schema.StringAttribute{
										Optional:    true,
										Validators:  fileOrDir,
										Description: "How per-device configurations are matched: `file` or `dir`.",
									},
									"per_device_configs_group_match_type": schema.StringAttribute{
										Optional:    true,
										Validators:  fileOrDir,
										Description: "How per-device group configurations are matched: `file` or `dir`.",
									},
									"kubernetes_prune": schema.BoolAttribute{
										Optional:    true,
										Description: "Whether a deployment deletes the Kubernetes resources a previous one applied that the manifests no longer declare. Only sent when configured; Portainer prunes when it is left out. Requires Portainer Business Edition 3.0 or later.",
									},
									"kubernetes_force_apply": schema.BoolAttribute{
										Optional:    true,
										Description: "Whether a Kubernetes resource whose immutable fields changed is deleted and recreated, instead of failing the deployment on it. Only sent when configured. Requires Portainer Business Edition 3.0 or later.",
									},
									"drift_mode": schema.StringAttribute{
										Optional:    true,
										Validators:  []validator.String{stringvalidator.OneOf("disabled", "warn", "enabled")},
										Description: "What a reconcile does about resources that no longer match the source: `disabled` ignores drift, `warn` reports it, `enabled` corrects it. `reconcile` engine only. Requires Portainer Business Edition 3.0 or later.",
									},
									"ignore_paths": schema.ListAttribute{
										Optional:    true,
										ElementType: types.StringType,
										Description: "JSON pointers whose differences are not drift, for fields a manifest declares but another controller owns, for example `/spec/replicas`. `reconcile` engine only. Requires Portainer Business Edition 3.0 or later.",
									},
									"timeout": schema.StringAttribute{
										Optional:    true,
										Validators:  []validator.String{durationValidator{}},
										Description: "How long a reconcile waits for what it applied to become ready, for example `5m`. `reconcile` engine only. Requires Portainer Business Edition 3.0 or later.",
									},
								},
								Blocks: map[string]schema.Block{
									"parallel": schema.ListNestedBlock{
										Description: "Staggered rollout settings.",
										Validators:  []validator.List{listvalidator.SizeAtMost(1)},
										NestedObject: schema.NestedBlockObject{
											Attributes: map[string]schema.Attribute{
												"batch_count": schema.Int64Attribute{
													Optional:    true,
													Description: "Devices updated at once. A value above zero turns parallel deployment on.",
												},
												"batch_increment_by": schema.Int64Attribute{
													Optional:    true,
													Description: "How much the batch grows each round, for an incremental rollout.",
												},
												"delay": schema.StringAttribute{
													Optional:    true,
													Description: "Pause between batches.",
												},
												"timeout": schema.StringAttribute{
													Optional:    true,
													Description: "How long a batch may take before it is given up on.",
												},
												"failure_action": schema.StringAttribute{
													Optional:    true,
													Description: "What happens when a batch fails.",
												},
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}

// gitopsHelmRemediationBlock is the install or upgrade block of a Helm
// artifact: what Portainer does when that release operation fails.
func gitopsHelmRemediationBlock(when, defaultStrategy string) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: "What happens when " + when + ".",
		Validators:  []validator.List{listvalidator.SizeAtMost(1)},
		NestedObject: schema.NestedBlockObject{
			Attributes: map[string]schema.Attribute{
				"retries": schema.Int64Attribute{
					Optional:    true,
					Validators:  []validator.Int64{int64validator.AtLeast(0)},
					Description: "How many further attempts are made before the failure is remediated.",
				},
				"remediate_last_failure": schema.BoolAttribute{
					Optional:    true,
					Description: "Whether the final failure is undone. Only sent when configured; Portainer undoes it when `retries` asked for further attempts.",
				},
				"strategy": schema.StringAttribute{
					Optional:    true,
					Validators:  []validator.String{stringvalidator.OneOf("rollback", "uninstall")},
					Description: "How a failure is undone: `rollback` or `uninstall`. Portainer defaults to " + defaultStrategy + ".",
				},
			},
		},
	}
}

func (r *gitopsWorkflowResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// gitopsConfiguredBool returns the bool and whether the configuration sets
// it, so a configured false can be told apart from an attribute left out -
// Portainer reads a missing field as "keep" or "the default", not as false.
// The value comes from the plan, which for an optional, non-computed
// attribute is exactly the configuration: null when left out.
func gitopsConfiguredBool(v types.Bool) (bool, bool) {
	if v.IsNull() || v.IsUnknown() {
		return false, false
	}
	return v.ValueBool(), true
}

// gitopsArtifactConfigPayload builds the config half of an artifact. Each
// field is omitted when empty so a workflow never sends a zero value
// Portainer would read as a deliberate setting - and never sends a Portainer
// 3.0 field to a server that does not know it.
func gitopsArtifactConfigPayload(ctx context.Context, l types.List, diags *diag.Diagnostics) (map[string]interface{}, bool) {
	var blocks []gitopsArtifactConfigResModel
	gitopsResObjects(ctx, l, &blocks, diags)
	if len(blocks) == 0 {
		return nil, false
	}
	block := blocks[0]

	config := map[string]interface{}{}
	if env := stringMapToGo(ctx, block.Environment, diags); len(env) > 0 {
		pairs := make([]map[string]string, 0, len(env))
		for name, value := range env {
			pairs = append(pairs, map[string]string{"name": name, "value": value})
		}
		config["envVars"] = pairs
	}
	if v := int64ListToInts(ctx, block.RegistryIDs, diags); len(v) > 0 {
		config["registries"] = v
	}
	if block.PrePullImage.ValueBool() {
		config["prePullImage"] = true
	}
	if v := block.RetryPeriod.ValueInt64(); v != 0 {
		config["retryPeriod"] = v
	}
	if block.UseManifestNamespaces.ValueBool() {
		config["useManifestNamespaces"] = true
	}
	if block.AlwaysCloneGitRepo.ValueBool() {
		config["alwaysCloneGitRepoForRelativePath"] = true
	}
	for key, v := range map[string]types.String{
		"localFilesystemPath":            block.LocalFilesystemPath,
		"perDeviceConfigsPath":           block.PerDeviceConfigsPath,
		"perDeviceConfigsMatchType":      block.PerDeviceConfigsMatchType,
		"perDeviceConfigsGroupMatchType": block.PerDeviceConfigsGroupMatchType,
		"driftMode":                      block.DriftMode,
		"timeout":                        block.Timeout,
	} {
		if s := gitopsResString(v); s != "" {
			config[key] = s
		}
	}
	// Portainer prunes when kubernetesPrune is left out of a create and keeps
	// the recorded choice when it is left out of an update, so false is only
	// meaningful when sent - and only sent when configured.
	if v, ok := gitopsConfiguredBool(block.KubernetesPrune); ok {
		config["kubernetesPrune"] = v
	}
	if v, ok := gitopsConfiguredBool(block.KubernetesForceApply); ok {
		config["kubernetesForceApply"] = v
	}
	if v := gitopsResStrings(ctx, block.IgnorePaths, diags); len(v) > 0 {
		config["ignorePaths"] = v
	}

	var parallels []gitopsParallelResModel
	gitopsResObjects(ctx, block.Parallel, &parallels, diags)
	if len(parallels) > 0 {
		parallel := parallels[0]
		encoded := map[string]interface{}{}
		if v := parallel.BatchCount.ValueInt64(); v != 0 {
			encoded["batchCount"] = v
		}
		if v := parallel.BatchIncrementBy.ValueInt64(); v != 0 {
			encoded["batchIncrementBy"] = v
		}
		for key, v := range map[string]types.String{
			"delay": parallel.Delay, "timeout": parallel.Timeout, "failureAction": parallel.FailureAction,
		} {
			if s := gitopsResString(v); s != "" {
				encoded[key] = s
			}
		}
		if len(encoded) > 0 {
			config["parallelConfig"] = encoded
		}
	}

	if len(config) == 0 {
		return nil, false
	}
	return config, true
}

// gitopsHelmPayload builds the chart of a Helm artifact, nil when the
// artifact applies plain manifests.
func gitopsHelmPayload(ctx context.Context, l types.List, diags *diag.Diagnostics) (map[string]interface{}, bool) {
	var blocks []gitopsHelmResModel
	gitopsResObjects(ctx, l, &blocks, diags)
	if len(blocks) == 0 {
		return nil, false
	}
	block := blocks[0]

	helm := map[string]interface{}{"chartPath": block.ChartPath.ValueString()}
	for key, v := range map[string]types.String{"releaseName": block.ReleaseName, "namespace": block.Namespace} {
		if s := gitopsResString(v); s != "" {
			helm[key] = s
		}
	}
	if v := gitopsResStrings(ctx, block.ValuesFiles, diags); len(v) > 0 {
		helm["valuesFiles"] = v
	}
	for phase, l := range map[string]types.List{"install": block.Install, "upgrade": block.Upgrade} {
		var remediations []gitopsRemediationResModel
		gitopsResObjects(ctx, l, &remediations, diags)
		if len(remediations) == 0 {
			continue
		}
		remediation := remediations[0]
		encoded := map[string]interface{}{}
		if v := remediation.Retries.ValueInt64(); v > 0 {
			encoded["retries"] = v
		}
		if v, ok := gitopsConfiguredBool(remediation.RemediateLastFailure); ok {
			encoded["remediateLastFailure"] = v
		}
		if s := gitopsResString(remediation.Strategy); s != "" {
			encoded["strategy"] = s
		}
		if len(encoded) > 0 {
			helm[phase] = encoded
		}
	}
	return helm, true
}

// gitopsArtifactsPayload builds the artifact list. On update Portainer keys
// artifacts by their identifier and takes their type, where on create it
// takes the name and deployment type - so the two payloads differ by more than
// a field name. The update payload has no name field, which is why the name
// forces replacement. A Helm chart differs too: a create carries it in the
// config, an update beside it.
//
// priorEngine is the engine the artifacts are on before this apply, which
// decides the type an update names them by: a reconciled deployment is a
// gitopsDeployment, while during a migration they are still the Edge stacks
// being converted, which Portainer matches by their Edge stack identifier.
func gitopsArtifactsPayload(ctx context.Context, artifacts []gitopsArtifactResModel, forUpdate bool, priorEngine string, diags *diag.Diagnostics) []map[string]interface{} {
	reconciled := priorEngine == "reconcile"

	out := make([]map[string]interface{}, 0, len(artifacts))
	for _, block := range artifacts {
		var files []gitopsFileResModel
		gitopsResObjects(ctx, block.File, &files, diags)
		encodedFiles := make([]map[string]interface{}, 0, len(files))
		for _, file := range files {
			encodedFiles = append(encodedFiles, map[string]interface{}{
				"sourceId": file.SourceID.ValueInt64(), "path": file.Path.ValueString(), "ref": file.Ref.ValueString(),
			})
		}

		// The three targets are alternatives, so only the configured ones are
		// sent: a Portainer 2.45 artifact only ever has Edge groups.
		targets := map[string]interface{}{}
		for key, l := range map[string]types.List{
			"edgeGroups":        block.EdgeGroupIDs,
			"environments":      block.EnvironmentIDs,
			"environmentGroups": block.EnvironmentGroupIDs,
		} {
			if v := int64ListToInts(ctx, l, diags); len(v) > 0 {
				targets[key] = v
			}
		}
		if len(targets) == 0 {
			targets["edgeGroups"] = []int{}
		}

		artifact := map[string]interface{}{
			"files":   encodedFiles,
			"targets": targets,
		}
		if forUpdate {
			artifact["id"] = block.ArtifactID.ValueInt64()
			artifact["type"] = block.Type.ValueString()
			if reconciled {
				artifact["type"] = "gitopsDeployment"
			}
		} else {
			artifact["name"] = block.Name.ValueString()
		}
		if s := gitopsResString(block.DeploymentType); s != "" {
			artifact["deploymentType"] = s
		}
		if v := gitopsResStrings(ctx, block.Dependencies, diags); len(v) > 0 {
			artifact["dependencies"] = v
		}

		config, hasConfig := gitopsArtifactConfigPayload(ctx, block.Config, diags)
		if helm, ok := gitopsHelmPayload(ctx, block.Helm, diags); ok {
			if forUpdate {
				artifact["helm"] = helm
			} else {
				if !hasConfig {
					config, hasConfig = map[string]interface{}{}, true
				}
				config["helm"] = helm
			}
		}
		if hasConfig {
			artifact["config"] = config
		}

		out = append(out, artifact)
	}
	return out
}

// ModifyPlan turns what Portainer would only reject at apply time, or not at
// all, into plan errors, and replaces a workflow asked to leave the reconcile
// engine: Portainer can migrate a workflow onto it, but has no way back.
func (r *gitopsWorkflowResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var plan gitopsWorkflowResModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var config gitopsWorkflowResModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	exists := !req.State.Raw.IsNull()
	var state gitopsWorkflowResModel
	if exists {
		resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// The engine in state before this plan; a create has none.
	priorEngine := ""
	if exists {
		priorEngine = gitopsResString(state.Engine)
	}
	engineChanged := !plan.Engine.IsUnknown() && plan.Engine.ValueString() != priorEngine

	if exists && engineChanged && priorEngine == "reconcile" && plan.Engine.ValueString() != "reconcile" {
		resp.RequiresReplace = append(resp.RequiresReplace, path.Root("engine"))
	}
	// A migration onto the reconcile engine turns every artifact into a
	// reconciled deployment with an identifier of its own.
	if exists && engineChanged && plan.Engine.ValueString() == "reconcile" && !plan.Artifact.IsUnknown() && !plan.Artifact.IsNull() {
		var artifacts []gitopsArtifactResModel
		resp.Diagnostics.Append(plan.Artifact.ElementsAs(ctx, &artifacts, false)...)
		if resp.Diagnostics.HasError() {
			return
		}
		for i := range artifacts {
			artifacts[i].ArtifactID = types.Int64Unknown()
		}
		list, d := types.ListValueFrom(ctx, plan.Artifact.ElementType(ctx), artifacts)
		resp.Diagnostics.Append(d...)
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("artifact"), list)...)
		if resp.Diagnostics.HasError() {
			return
		}
	}

	// The engine the plan settles on, and whether it is known yet. An unset
	// engine is unknown on create, being computed, but means `edgeStack`
	// there; one taken from a value not known until apply cannot be checked.
	engine, known := "", false
	switch {
	case !plan.Engine.IsUnknown():
		engine, known = plan.Engine.ValueString(), true
	case config.Engine.IsNull():
		known = true
	}
	if known && !plan.ReconcileInterval.IsUnknown() {
		interval := gitopsResString(plan.ReconcileInterval)
		switch {
		case interval != "" && engine != "reconcile":
			resp.Diagnostics.AddError("reconcile_interval only applies to engine = \"reconcile\"", "")
			return
		case interval == "" && engine == "reconcile" && engineChanged:
			resp.Diagnostics.AddError("reconcile_interval is required to create or migrate a workflow on engine = \"reconcile\"", "")
			return
		}
	}

	if plan.Artifact.IsUnknown() || plan.Artifact.IsNull() {
		return
	}
	for _, element := range plan.Artifact.Elements() {
		obj, ok := element.(types.Object)
		if !ok || obj.IsUnknown() || obj.IsNull() {
			continue
		}
		attrs := obj.Attributes()
		targeted, allKnown := false, true
		for _, field := range []string{"edge_group_ids", "environment_ids", "environment_group_ids"} {
			l, _ := attrs[field].(types.List)
			if l.IsUnknown() {
				allKnown = false
			}
			if gitopsResListLen(l) > 0 {
				targeted = true
			}
		}
		if allKnown && !targeted {
			name, _ := attrs["name"].(types.String)
			resp.Diagnostics.AddError(fmt.Sprintf("artifact %q needs a target: set edge_group_ids, environment_ids or environment_group_ids", name.ValueString()), "")
			return
		}
	}
}

// gitopsEngineReported derives the engine from the artifact types, as
// Portainer reports no engine of its own: a reconciled deployment is a
// gitopsDeployment artifact. Empty when there is no artifact to tell by.
func gitopsEngineReported(artifactTypes []string) string {
	engine := ""
	for _, t := range artifactTypes {
		switch t {
		case "gitopsDeployment":
			return "reconcile"
		case "stack", "edgeStack":
			engine = "edgeStack"
		}
	}
	return engine
}

// read refreshes m from Portainer. found is false when the
// workflow is gone.
func (r *gitopsWorkflowResource) read(ctx context.Context, m *gitopsWorkflowResModel, keepName bool, diags *diag.Diagnostics) (found bool) {
	var workflow struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		Artifacts []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"artifacts"`
	}
	id := m.ID.ValueString()
	workflowURL := fmt.Sprintf("%s/gitops/workflows/%s", r.client.Endpoint, id)
	if err := doJSON(ctx, r.client, http.MethodGet, workflowURL, nil, &workflow); err != nil {
		if isAPINotFound(err) {
			return false
		}
		gitopsResErrorf(diags, "failed to read GitOps workflow %s: %w", id, err)
		return true
	}

	// Only the identifiers are read back into the artifact blocks. The update
	// payload needs them, but the rest of an artifact is driven by the
	// configuration: Portainer normalises the file and config fields, and
	// reading them back would turn a stable configuration into a churning plan.
	identifiers := make(map[string]int, len(workflow.Artifacts))
	reportedTypes := make([]string, 0, len(workflow.Artifacts))
	for _, artifact := range workflow.Artifacts {
		identifiers[artifact.Name] = artifact.ID
		reportedTypes = append(reportedTypes, artifact.Type)
	}

	if !m.Artifact.IsNull() && !m.Artifact.IsUnknown() {
		var artifacts []gitopsArtifactResModel
		diags.Append(m.Artifact.ElementsAs(ctx, &artifacts, false)...)
		if diags.HasError() {
			return true
		}
		for i := range artifacts {
			if id, ok := identifiers[artifacts[i].Name.ValueString()]; ok {
				artifacts[i].ArtifactID = types.Int64Value(int64(id))
			} else if artifacts[i].ArtifactID.IsUnknown() {
				artifacts[i].ArtifactID = types.Int64Value(0)
			}
		}
		list, d := types.ListValueFrom(ctx, m.Artifact.ElementType(ctx), artifacts)
		diags.Append(d...)
		m.Artifact = list
	}

	m.WorkflowID = types.Int64Value(int64(workflow.ID))
	if !keepName {
		m.Name = types.StringValue(workflow.Name)
	}
	// The engine is only recorded once it is known to matter: a reconciled
	// workflow always, an Edge stack one only when an engine is already in
	// state. An unset engine on Portainer 2.45 so stays unset, with no drift.
	if reported := gitopsEngineReported(reportedTypes); reported == "reconcile" || (reported != "" && gitopsResString(m.Engine) != "") {
		m.Engine = types.StringValue(reported)
	} else if m.Engine.IsUnknown() {
		m.Engine = types.StringValue("")
	}
	return true
}

// gitopsCheckEngine fails an apply that asked for the reconcile engine when the read
// shows Portainer did not take it up: a server before 3.0 ignores the engine
// field it does not know and deploys Edge stacks instead.
func gitopsCheckEngine(m *gitopsWorkflowResModel, wanted string, diags *diag.Diagnostics) {
	if wanted == "reconcile" && gitopsResString(m.Engine) != "reconcile" {
		gitopsResErrorf(diags, "the reconcile engine was not applied to GitOps workflow %s: Portainer still reports its artifacts as Edge stacks; the reconcile engine requires %s", m.ID.ValueString(), requiresBE30)
	}
}

func (r *gitopsWorkflowResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan gitopsWorkflowResModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	var artifacts []gitopsArtifactResModel
	gitopsResObjects(ctx, plan.Artifact, &artifacts, &resp.Diagnostics)
	payload := map[string]interface{}{
		"name":      plan.Name.ValueString(),
		"artifacts": gitopsArtifactsPayload(ctx, artifacts, false, "", &resp.Diagnostics),
	}
	if resp.Diagnostics.HasError() {
		return
	}
	// Both only when configured, so a Portainer 2.45 workflow is created
	// exactly as before.
	wanted := gitopsResString(plan.Engine)
	if wanted != "" {
		payload["engine"] = wanted
	}
	if v := gitopsResString(plan.ReconcileInterval); v != "" {
		payload["reconcile"] = map[string]interface{}{"interval": v}
	}

	var workflow struct {
		ID int `json:"id"`
	}
	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/gitops/workflows", payload, &workflow); err != nil {
		gitopsResErrorf(&resp.Diagnostics, "failed to create GitOps workflow %s: %w", plan.Name.ValueString(), err)
		return
	}

	plan.ID = types.StringValue(strconv.Itoa(workflow.ID))
	if !r.read(ctx, &plan, true, &resp.Diagnostics) {
		// Gone straight after the create: record nothing.
		return
	}
	if !resp.Diagnostics.HasError() {
		gitopsCheckEngine(&plan, wanted, &resp.Diagnostics)
	}
	// Recorded even when the read or the engine check failed, as SDKv2 kept
	// the ID it had set: the workflow exists in Portainer either way.
	gitopsSettleUnknowns(ctx, &plan, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.Private.SetKey(ctx, gitopsWorkflowPrivateKey, gitopsWorkflowPrivateMarker)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

// gitopsSettleUnknowns replaces the computed values a failed read left
// unknown with what SDKv2 recorded for them: zero values.
func gitopsSettleUnknowns(ctx context.Context, m *gitopsWorkflowResModel, diags *diag.Diagnostics) {
	if m.Engine.IsUnknown() {
		m.Engine = types.StringValue("")
	}
	if m.WorkflowID.IsUnknown() {
		m.WorkflowID = types.Int64Value(0)
	}
	if m.Artifact.IsNull() || m.Artifact.IsUnknown() {
		return
	}
	var artifacts []gitopsArtifactResModel
	diags.Append(m.Artifact.ElementsAs(ctx, &artifacts, false)...)
	for i := range artifacts {
		if artifacts[i].ArtifactID.IsUnknown() {
			artifacts[i].ArtifactID = types.Int64Value(0)
		}
	}
	list, d := types.ListValueFrom(ctx, m.Artifact.ElementType(ctx), artifacts)
	diags.Append(d...)
	m.Artifact = list
}

func (r *gitopsWorkflowResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	fwSetIDIdentity(ctx, req.State, resp.Identity, &resp.Diagnostics)
	var state gitopsWorkflowResModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if !r.read(ctx, &state, false, &resp.Diagnostics) {
		resp.State.RemoveResource(ctx)
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	// State SDKv2 wrote (v3.0.0 and earlier) holds false, 0 and empty lists
	// for every optional argument of an artifact the configuration left
	// out, which this provider - like the configuration - holds as null.
	// That state carries no marker, so it is converted once.
	marker, d := req.Private.GetKey(ctx, gitopsWorkflowPrivateKey)
	resp.Diagnostics.Append(d...)
	if len(marker) == 0 && !state.Artifact.IsNull() && !state.Artifact.IsUnknown() {
		if list, ok := gitopsNullZeroes(state.Artifact, gitopsArtifactKeptZeroes).(types.List); ok {
			state.Artifact = list
		}
	}
	resp.Diagnostics.Append(resp.Private.SetKey(ctx, gitopsWorkflowPrivateKey, gitopsWorkflowPrivateMarker)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
}

// gitopsWorkflowPrivateKey marks state this provider wrote, as opposed to
// state SDKv2 wrote with zero values for unset arguments.
const gitopsWorkflowPrivateKey = "framework_state"

var gitopsWorkflowPrivateMarker = []byte(`{"v":1}`)

// gitopsArtifactKeptZeroes are the artifact attributes whose zero value is a
// value: the computed identifier and the defaulted type.
var gitopsArtifactKeptZeroes = map[string]bool{"artifact_id": true, "type": true}

func (r *gitopsWorkflowResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan, state gitopsWorkflowResModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	priorEngine := gitopsResString(state.Engine)

	var artifacts, prior []gitopsArtifactResModel
	gitopsResObjects(ctx, plan.Artifact, &artifacts, &resp.Diagnostics)
	gitopsResObjects(ctx, state.Artifact, &prior, &resp.Diagnostics)
	// An identifier the plan leaves to the apply (a migration renumbers the
	// artifacts) is still the recorded one in this update.
	for i := range artifacts {
		if artifacts[i].ArtifactID.IsUnknown() && i < len(prior) {
			artifacts[i].ArtifactID = prior[i].ArtifactID
		}
	}
	payload := map[string]interface{}{
		"name":      plan.Name.ValueString(),
		"artifacts": gitopsArtifactsPayload(ctx, artifacts, true, priorEngine, &resp.Diagnostics),
	}
	if resp.Diagnostics.HasError() {
		return
	}
	// engine is a migration rather than a setting: only "reconcile" is
	// accepted, and only sent when the workflow is moving onto it. The
	// interval is sent whenever configured; left out, Portainer keeps the
	// recorded one.
	wanted := gitopsResString(plan.Engine)
	if wanted != priorEngine && wanted == "reconcile" {
		payload["engine"] = wanted
	}
	if v := gitopsResString(plan.ReconcileInterval); v != "" {
		payload["reconcile"] = map[string]interface{}{"interval": v}
	}

	plan.ID = state.ID
	workflowURL := fmt.Sprintf("%s/gitops/workflows/%s", r.client.Endpoint, plan.ID.ValueString())
	if err := doJSON(ctx, r.client, http.MethodPut, workflowURL, payload, nil); err != nil {
		gitopsResErrorf(&resp.Diagnostics, "failed to update GitOps workflow %s: %w", plan.ID.ValueString(), err)
		return
	}

	if !r.read(ctx, &plan, true, &resp.Diagnostics) {
		resp.State.RemoveResource(ctx)
		return
	}
	if resp.Diagnostics.HasError() {
		return
	}
	gitopsCheckEngine(&plan, wanted, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.Private.SetKey(ctx, gitopsWorkflowPrivateKey, gitopsWorkflowPrivateMarker)...)
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
}

func (r *gitopsWorkflowResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var state gitopsWorkflowResModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}

	mode := gitopsResString(state.OnDestroy)
	deleteURL := fmt.Sprintf("%s/gitops/workflows/%s/%s", r.client.Endpoint, state.ID.ValueString(), mode)
	if err := doJSON(ctx, r.client, http.MethodDelete, deleteURL, nil, nil); err != nil && !isAPINotFound(err) {
		gitopsResErrorf(&resp.Diagnostics, "failed to %s GitOps workflow %s: %w", mode, state.ID.ValueString(), err)
	}
}

func (r *gitopsWorkflowResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	fwImportIDOrIdentity(ctx, req, resp)
}
