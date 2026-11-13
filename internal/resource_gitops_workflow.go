package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// resourceGitopsWorkflow manages a GitOps workflow: a named set of artifacts,
// each deployed from files in a GitOps source onto a set of edge groups.
//
// Portainer offers two ways to remove one, which is why `on_destroy` exists:
// `destroy` tears the deployed artifacts down with the workflow, `detach`
// leaves them running and only unlinks them from GitOps.
//
// Portainer 3.0 added a second engine. `reconcile` keeps Edge Kubernetes
// environments continuously matching the source instead of deploying Edge
// stacks; a workflow can be migrated to it in place, but never back.
func resourceGitopsWorkflow() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceGitopsWorkflowCreate,
		ReadContext:   resourceGitopsWorkflowRead,
		UpdateContext: resourceGitopsWorkflowUpdate,
		DeleteContext: resourceGitopsWorkflowDelete,
		CustomizeDiff: resourceGitopsWorkflowCustomizeDiff,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the workflow.",
			},
			"engine": {
				Type:     schema.TypeString,
				Optional: true,
				// Computed so an unset engine follows whatever the server
				// runs, and so an imported reconcile workflow does not plan a
				// replacement: Portainer has no engine field to read, it is
				// derived from the artifact types.
				Computed:     true,
				ValidateFunc: validation.StringInSlice([]string{"edgeStack", "reconcile"}, false),
				Description:  "Engine that deploys the artifacts and keeps them in step with their source: `edgeStack` deploys each artifact as an Edge stack, `reconcile` continuously reconciles Edge Kubernetes environments against the source and reports or corrects drift. Left unset, Portainer uses `edgeStack`. Changing `edgeStack` to `reconcile` migrates the workflow in place: Portainer removes its Edge stacks and adopts what they deployed. The migration is one-way, so changing `reconcile` to anything else forces a new resource. Requires Portainer Business Edition 3.0 or later.",
			},
			"reconcile_interval": {
				Type:         schema.TypeString,
				Optional:     true,
				ValidateFunc: validateDuration,
				Description:  "How often every artifact reconciles even when its source has not moved, which is what makes drift detectable, for example `5m`. Required when `engine` is set to `reconcile`, rejected for any other engine. Removing it leaves the recorded interval alone. Requires Portainer Business Edition 3.0 or later.",
			},
			"on_destroy": {
				Type:         schema.TypeString,
				Optional:     true,
				Default:      "destroy",
				ValidateFunc: validation.StringInSlice([]string{"destroy", "detach"}, false),
				Description:  "What happens to the deployed artifacts when the workflow is destroyed: `destroy` tears them down, `detach` leaves them running and only unlinks them from GitOps. Defaults to `destroy`.",
			},
			"artifact": {
				Type:        schema.TypeList,
				Required:    true,
				Description: "The artifacts the workflow deploys. Repeatable.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
							// Portainer's update payload has no name field at
							// all, so an artifact cannot be renamed in place.
							// Without ForceNew a rename would be accepted into
							// state and never sent, leaving the configuration
							// and Portainer quietly disagreeing.
							ForceNew:    true,
							Description: "Name of the artifact. Portainer cannot rename an artifact, so changing it forces a new resource.",
						},
						"type": {
							Type:         schema.TypeString,
							Optional:     true,
							Default:      "edgeStack",
							ValidateFunc: validation.StringInSlice([]string{"stack", "edgeStack"}, false),
							Description:  "What the artifact deploys as: `stack` or `edgeStack`. Defaults to `edgeStack`. Ignored on the `reconcile` engine, which deploys every artifact as a reconciled deployment.",
						},
						"deployment_type": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Deployment type of the artifact, for example `compose` or `kubernetes`.",
						},
						"edge_group_ids": {
							Type:     schema.TypeList,
							Optional: true,
							// Required until Portainer 3.0 made Edge groups one of
							// three alternative targets; the plan still insists
							// on at least one of them.
							Elem:        &schema.Schema{Type: schema.TypeInt},
							Description: "Edge groups the artifact is deployed to. Every artifact needs at least one target: `edge_group_ids`, `environment_ids` or `environment_group_ids`. The `edgeStack` engine only deploys to Edge groups.",
						},
						"environment_ids": {
							Type:        schema.TypeList,
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeInt},
							Description: "Environments the artifact is deployed to directly, rather than through a group. `reconcile` engine only. Requires Portainer Business Edition 3.0 or later.",
						},
						"environment_group_ids": {
							Type:        schema.TypeList,
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeInt},
							Description: "Environment groups the artifact is deployed to: every supported environment of each group, including ones added to it later. `reconcile` engine only. Requires Portainer Business Edition 3.0 or later.",
						},
						"dependencies": {
							Type:        schema.TypeList,
							Optional:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Names of other artifacts in this workflow that must be deployed before this one. Requires Portainer Business Edition 3.0 or later.",
						},
						"helm": {
							Type:        schema.TypeList,
							Optional:    true,
							MaxItems:    1,
							Description: "Deploys the artifact as a Helm chart rather than as plain manifests. The artifact's `file` paths must point at the chart directory. `reconcile` engine only. Requires Portainer Business Edition 3.0 or later.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"chart_path": {
										Type:        schema.TypeString,
										Required:    true,
										Description: "Directory of the chart within the artifact's sources, for example `charts/my-app`.",
									},
									"release_name": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Name of the Helm release. Portainer generates one when it is left out.",
									},
									"namespace": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Namespace the release is installed into. Left out, Helm uses the cluster's default namespace.",
									},
									"values_files": {
										Type:        schema.TypeList,
										Optional:    true,
										Elem:        &schema.Schema{Type: schema.TypeString},
										Description: "Values file paths within the artifact's sources, merged over the chart's own `values.yaml` in the order given.",
									},
									"install": gitopsHelmRemediationSchema("the first install of the release fails", "removing the release it could not complete (`uninstall`)"),
									"upgrade": gitopsHelmRemediationSchema("an upgrade of the release fails", "rolling back (`rollback`)"),
								},
							},
						},
						"file": {
							Type:        schema.TypeList,
							Required:    true,
							Description: "Files in a GitOps source the artifact is built from. Repeatable.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"source_id": {
										Type:        schema.TypeInt,
										Required:    true,
										Description: "Identifier of the GitOps source the file comes from.",
									},
									"path": {
										Type:        schema.TypeString,
										Required:    true,
										Description: "Path of the file within the source, for example `portainer.yaml`.",
									},
									"ref": {
										Type:        schema.TypeString,
										Required:    true,
										Description: "Git reference to read the file at, for example `refs/heads/main`.",
									},
								},
							},
						},
						"config": {
							Type:        schema.TypeList,
							Optional:    true,
							MaxItems:    1,
							Description: "Deployment options for the artifact.",
							Elem: &schema.Resource{
								Schema: map[string]*schema.Schema{
									"environment": {
										Type:        schema.TypeMap,
										Optional:    true,
										Elem:        &schema.Schema{Type: schema.TypeString},
										Description: "Environment variables injected into the deployment.",
									},
									"registry_ids": {
										Type:        schema.TypeList,
										Optional:    true,
										Elem:        &schema.Schema{Type: schema.TypeInt},
										Description: "Registries the deployment pulls images from.",
									},
									"pre_pull_image": {
										Type:        schema.TypeBool,
										Optional:    true,
										Description: "Whether agents pull the images before deploying.",
									},
									"retry_period": {
										Type:        schema.TypeInt,
										Optional:    true,
										Description: "How long an agent keeps retrying a failed deployment, in seconds.",
									},
									"use_manifest_namespaces": {
										Type:        schema.TypeBool,
										Optional:    true,
										Description: "Whether to use the namespaces in the manifest rather than the default one.",
									},
									"always_clone_git_repo": {
										Type:        schema.TypeBool,
										Optional:    true,
										Description: "Whether the agent always clones the git repository for relative paths.",
									},
									"local_filesystem_path": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Path on the agent used for relative path volumes.",
									},
									"per_device_configs_path": {
										Type:        schema.TypeString,
										Optional:    true,
										Description: "Path within the repository holding per-device configurations.",
									},
									"per_device_configs_match_type": {
										Type:         schema.TypeString,
										Optional:     true,
										ValidateFunc: validation.StringInSlice([]string{"file", "dir"}, false),
										Description:  "How per-device configurations are matched: `file` or `dir`.",
									},
									"per_device_configs_group_match_type": {
										Type:         schema.TypeString,
										Optional:     true,
										ValidateFunc: validation.StringInSlice([]string{"file", "dir"}, false),
										Description:  "How per-device group configurations are matched: `file` or `dir`.",
									},
									"kubernetes_prune": {
										Type:        schema.TypeBool,
										Optional:    true,
										Description: "Whether a deployment deletes the Kubernetes resources a previous one applied that the manifests no longer declare. Only sent when configured; Portainer prunes when it is left out. Requires Portainer Business Edition 3.0 or later.",
									},
									"kubernetes_force_apply": {
										Type:        schema.TypeBool,
										Optional:    true,
										Description: "Whether a Kubernetes resource whose immutable fields changed is deleted and recreated, instead of failing the deployment on it. Only sent when configured. Requires Portainer Business Edition 3.0 or later.",
									},
									"drift_mode": {
										Type:         schema.TypeString,
										Optional:     true,
										ValidateFunc: validation.StringInSlice([]string{"disabled", "warn", "enabled"}, false),
										Description:  "What a reconcile does about resources that no longer match the source: `disabled` ignores drift, `warn` reports it, `enabled` corrects it. `reconcile` engine only. Requires Portainer Business Edition 3.0 or later.",
									},
									"ignore_paths": {
										Type:        schema.TypeList,
										Optional:    true,
										Elem:        &schema.Schema{Type: schema.TypeString},
										Description: "JSON pointers whose differences are not drift, for fields a manifest declares but another controller owns, for example `/spec/replicas`. `reconcile` engine only. Requires Portainer Business Edition 3.0 or later.",
									},
									"timeout": {
										Type:         schema.TypeString,
										Optional:     true,
										ValidateFunc: validateDuration,
										Description:  "How long a reconcile waits for what it applied to become ready, for example `5m`. `reconcile` engine only. Requires Portainer Business Edition 3.0 or later.",
									},
									"parallel": {
										Type:        schema.TypeList,
										Optional:    true,
										MaxItems:    1,
										Description: "Staggered rollout settings.",
										Elem: &schema.Resource{
											Schema: map[string]*schema.Schema{
												"batch_count": {
													Type:        schema.TypeInt,
													Optional:    true,
													Description: "Devices updated at once. A value above zero turns parallel deployment on.",
												},
												"batch_increment_by": {
													Type:        schema.TypeInt,
													Optional:    true,
													Description: "How much the batch grows each round, for an incremental rollout.",
												},
												"delay": {
													Type:        schema.TypeString,
													Optional:    true,
													Description: "Pause between batches.",
												},
												"timeout": {
													Type:        schema.TypeString,
													Optional:    true,
													Description: "How long a batch may take before it is given up on.",
												},
												"failure_action": {
													Type:        schema.TypeString,
													Optional:    true,
													Description: "What happens when a batch fails.",
												},
											},
										},
									},
								},
							},
						},
						// Computed attributes
						"artifact_id": {
							Type:        schema.TypeInt,
							Computed:    true,
							Description: "Identifier Portainer assigned to the artifact, which the update call needs. On the `reconcile` engine it identifies the reconciled deployment, as `portainer_gitops_workflow_deployment_resources` takes it.",
						},
					},
				},
			},
			// Computed attributes
			"workflow_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Identifier of the workflow.",
			},
		},
	}
}

// gitopsHelmRemediationSchema is the install or upgrade block of a Helm
// artifact: what Portainer does when that release operation fails.
func gitopsHelmRemediationSchema(when, defaultStrategy string) *schema.Schema {
	return &schema.Schema{
		Type:        schema.TypeList,
		Optional:    true,
		MaxItems:    1,
		Description: "What happens when " + when + ".",
		Elem: &schema.Resource{
			Schema: map[string]*schema.Schema{
				"retries": {
					Type:         schema.TypeInt,
					Optional:     true,
					ValidateFunc: validation.IntAtLeast(0),
					Description:  "How many further attempts are made before the failure is remediated.",
				},
				"remediate_last_failure": {
					Type:        schema.TypeBool,
					Optional:    true,
					Description: "Whether the final failure is undone. Only sent when configured; Portainer undoes it when `retries` asked for further attempts.",
				},
				"strategy": {
					Type:         schema.TypeString,
					Optional:     true,
					ValidateFunc: validation.StringInSlice([]string{"rollback", "uninstall"}, false),
					Description:  "How a failure is undone: `rollback` or `uninstall`. Portainer defaults to " + defaultStrategy + ".",
				},
			},
		},
	}
}

// configuredBoolAt returns the bool at path and whether the configuration
// sets it, so a configured false can be told apart from an attribute left
// out - Portainer reads a missing field as "keep" or "the default", not as
// false. value is the attribute as the caller already holds it. Without a raw
// configuration (unit tests) only true counts as set.
func configuredBoolAt(d *schema.ResourceData, path cty.Path, value interface{}) (bool, bool) {
	v, _ := value.(bool)
	if d.GetRawConfig().IsNull() {
		return v, v
	}
	raw, diags := d.GetRawConfigAt(path)
	if diags.HasError() || !raw.IsKnown() || raw.IsNull() {
		return false, false
	}
	return v, true
}

// gitopsArtifactConfig builds the config half of an artifact. Each field is
// omitted when empty so a workflow never sends a zero value Portainer would
// read as a deliberate setting - and never sends a Portainer 3.0 field to a
// server that does not know it. path is the config block in the
// configuration, for the switches whose false has to be sent.
func gitopsArtifactConfig(d *schema.ResourceData, path cty.Path, raw interface{}) (map[string]interface{}, bool) {
	list, _ := raw.([]interface{})
	if len(list) == 0 {
		return nil, false
	}
	block, ok := list[0].(map[string]interface{})
	if !ok {
		return nil, false
	}
	path = path.IndexInt(0)

	config := map[string]interface{}{}
	if v, ok := block["environment"].(map[string]interface{}); ok && len(v) > 0 {
		pairs := make([]map[string]string, 0, len(v))
		for name, value := range v {
			pairs = append(pairs, map[string]string{"name": name, "value": fmt.Sprintf("%v", value)})
		}
		config["envVars"] = pairs
	}
	if v, ok := block["registry_ids"].([]interface{}); ok && len(v) > 0 {
		config["registries"] = toIntSlice(v)
	}
	if v, ok := block["pre_pull_image"].(bool); ok && v {
		config["prePullImage"] = v
	}
	if v, ok := block["retry_period"].(int); ok && v != 0 {
		config["retryPeriod"] = v
	}
	if v, ok := block["use_manifest_namespaces"].(bool); ok && v {
		config["useManifestNamespaces"] = v
	}
	if v, ok := block["always_clone_git_repo"].(bool); ok && v {
		config["alwaysCloneGitRepoForRelativePath"] = v
	}
	for field, key := range map[string]string{
		"local_filesystem_path":               "localFilesystemPath",
		"per_device_configs_path":             "perDeviceConfigsPath",
		"per_device_configs_match_type":       "perDeviceConfigsMatchType",
		"per_device_configs_group_match_type": "perDeviceConfigsGroupMatchType",
		"drift_mode":                          "driftMode",
		"timeout":                             "timeout",
	} {
		if v, ok := block[field].(string); ok && v != "" {
			config[key] = v
		}
	}
	// Portainer prunes when kubernetesPrune is left out of a create and keeps
	// the recorded choice when it is left out of an update, so false is only
	// meaningful when sent - and only sent when configured.
	for field, key := range map[string]string{
		"kubernetes_prune":       "kubernetesPrune",
		"kubernetes_force_apply": "kubernetesForceApply",
	} {
		if v, ok := configuredBoolAt(d, path.GetAttr(field), block[field]); ok {
			config[key] = v
		}
	}
	if v, ok := block["ignore_paths"].([]interface{}); ok && len(v) > 0 {
		config["ignorePaths"] = expandStringList(v)
	}

	if parallels, ok := block["parallel"].([]interface{}); ok && len(parallels) > 0 {
		if parallel, ok := parallels[0].(map[string]interface{}); ok {
			encoded := map[string]interface{}{}
			if v, ok := parallel["batch_count"].(int); ok && v != 0 {
				encoded["batchCount"] = v
			}
			if v, ok := parallel["batch_increment_by"].(int); ok && v != 0 {
				encoded["batchIncrementBy"] = v
			}
			for field, key := range map[string]string{
				"delay": "delay", "timeout": "timeout", "failure_action": "failureAction",
			} {
				if v, ok := parallel[field].(string); ok && v != "" {
					encoded[key] = v
				}
			}
			if len(encoded) > 0 {
				config["parallelConfig"] = encoded
			}
		}
	}

	if len(config) == 0 {
		return nil, false
	}
	return config, true
}

// gitopsHelm builds the chart of a Helm artifact, nil when the artifact
// applies plain manifests. path is the helm block in the configuration.
func gitopsHelm(d *schema.ResourceData, path cty.Path, raw interface{}) (map[string]interface{}, bool) {
	list, _ := raw.([]interface{})
	if len(list) == 0 {
		return nil, false
	}
	block, ok := list[0].(map[string]interface{})
	if !ok {
		return nil, false
	}
	path = path.IndexInt(0)

	helm := map[string]interface{}{"chartPath": block["chart_path"]}
	for field, key := range map[string]string{"release_name": "releaseName", "namespace": "namespace"} {
		if v, ok := block[field].(string); ok && v != "" {
			helm[key] = v
		}
	}
	if v, ok := block["values_files"].([]interface{}); ok && len(v) > 0 {
		helm["valuesFiles"] = expandStringList(v)
	}
	for _, phase := range []string{"install", "upgrade"} {
		phases, _ := block[phase].([]interface{})
		if len(phases) == 0 {
			continue
		}
		remediation, ok := phases[0].(map[string]interface{})
		if !ok {
			continue
		}
		encoded := map[string]interface{}{}
		if v, ok := remediation["retries"].(int); ok && v > 0 {
			encoded["retries"] = v
		}
		if v, ok := configuredBoolAt(d, path.GetAttr(phase).IndexInt(0).GetAttr("remediate_last_failure"), remediation["remediate_last_failure"]); ok {
			encoded["remediateLastFailure"] = v
		}
		if v, ok := remediation["strategy"].(string); ok && v != "" {
			encoded["strategy"] = v
		}
		if len(encoded) > 0 {
			helm[phase] = encoded
		}
	}
	return helm, true
}

// gitopsArtifacts builds the artifact list. On update Portainer keys artifacts
// by their identifier and takes their type, where on create it takes the name
// and deployment type - so the two payloads differ by more than a field name.
// The update payload has no name field, which is why the name is ForceNew.
// A Helm chart differs too: a create carries it in the config, an update
// beside it.
func gitopsArtifacts(d *schema.ResourceData, forUpdate bool) []map[string]interface{} {
	// The engine the artifacts are on before this apply decides the type an
	// update names them by: a reconciled deployment is a gitopsDeployment,
	// while during a migration they are still the Edge stacks being converted,
	// which Portainer matches by their Edge stack identifier.
	priorEngine, _ := d.GetChange("engine")
	reconciled := priorEngine == "reconcile"

	list, _ := d.Get("artifact").([]interface{})
	artifacts := make([]map[string]interface{}, 0, len(list))
	for i, item := range list {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		path := cty.GetAttrPath("artifact").IndexInt(i)

		files, _ := block["file"].([]interface{})
		encodedFiles := make([]map[string]interface{}, 0, len(files))
		for _, raw := range files {
			file, ok := raw.(map[string]interface{})
			if !ok {
				continue
			}
			encodedFiles = append(encodedFiles, map[string]interface{}{
				"sourceId": file["source_id"], "path": file["path"], "ref": file["ref"],
			})
		}

		// The three targets are alternatives, so only the configured ones are
		// sent: a Portainer 2.45 artifact only ever has Edge groups.
		targets := map[string]interface{}{}
		for field, key := range map[string]string{
			"edge_group_ids":        "edgeGroups",
			"environment_ids":       "environments",
			"environment_group_ids": "environmentGroups",
		} {
			if v, ok := block[field].([]interface{}); ok && len(v) > 0 {
				targets[key] = toIntSlice(v)
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
			artifact["id"] = block["artifact_id"]
			artifact["type"] = block["type"]
			if reconciled {
				artifact["type"] = "gitopsDeployment"
			}
		} else {
			artifact["name"] = block["name"]
		}
		if v, ok := block["deployment_type"].(string); ok && v != "" {
			artifact["deploymentType"] = v
		}
		if v, ok := block["dependencies"].([]interface{}); ok && len(v) > 0 {
			artifact["dependencies"] = expandStringList(v)
		}

		config, hasConfig := gitopsArtifactConfig(d, path.GetAttr("config"), block["config"])
		if helm, ok := gitopsHelm(d, path.GetAttr("helm"), block["helm"]); ok {
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

		artifacts = append(artifacts, artifact)
	}
	return artifacts
}

// plannedGitopsEngine is the engine the plan settles on, and whether it is
// known yet. An unset engine is unknown on create, being computed, but means
// `edgeStack` there; one taken from a value not known until apply cannot be
// checked.
func plannedGitopsEngine(d *schema.ResourceDiff) (string, bool) {
	if d.NewValueKnown("engine") {
		return d.Get("engine").(string), true
	}
	raw := d.GetRawConfig()
	if !raw.IsNull() && raw.IsKnown() && raw.Type().IsObjectType() && raw.Type().HasAttribute("engine") {
		if engine := raw.GetAttr("engine"); engine.IsKnown() && engine.IsNull() {
			return "", true
		}
	}
	return "", false
}

// resourceGitopsWorkflowCustomizeDiff turns what Portainer would only reject
// at apply time, or not at all, into plan errors, and replaces a workflow
// asked to leave the reconcile engine: Portainer can migrate a workflow onto
// it, but has no way back.
func resourceGitopsWorkflowCustomizeDiff(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	if d.Id() != "" && d.HasChange("engine") && d.NewValueKnown("engine") {
		if old, updated := d.GetChange("engine"); old == "reconcile" && updated != "reconcile" {
			if err := d.ForceNew("engine"); err != nil {
				return err
			}
		}
	}

	if engine, known := plannedGitopsEngine(d); known && d.NewValueKnown("reconcile_interval") {
		interval := d.Get("reconcile_interval").(string)
		switch {
		case interval != "" && engine != "reconcile":
			return fmt.Errorf("reconcile_interval only applies to engine = \"reconcile\"")
		case interval == "" && engine == "reconcile" && d.HasChange("engine"):
			return fmt.Errorf("reconcile_interval is required to create or migrate a workflow on engine = \"reconcile\"")
		}
	}

	artifacts, _ := d.Get("artifact").([]interface{})
	for i, item := range artifacts {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		targeted, known := false, true
		for _, field := range []string{"edge_group_ids", "environment_ids", "environment_group_ids"} {
			if !d.NewValueKnown(fmt.Sprintf("artifact.%d.%s", i, field)) {
				known = false
			}
			if v, ok := block[field].([]interface{}); ok && len(v) > 0 {
				targeted = true
			}
		}
		if known && !targeted {
			return fmt.Errorf("artifact %q needs a target: set edge_group_ids, environment_ids or environment_group_ids", block["name"])
		}
	}
	return nil
}

// gitopsEngineReported derives the engine from the artifact types, as
// Portainer reports no engine of its own: a reconciled deployment is a
// gitopsDeployment artifact. Empty when there is no artifact to tell by.
func gitopsEngineReported(types []string) string {
	engine := ""
	for _, t := range types {
		switch t {
		case "gitopsDeployment":
			return "reconcile"
		case "stack", "edgeStack":
			engine = "edgeStack"
		}
	}
	return engine
}

// gitopsCheckEngine fails an apply that asked for the reconcile engine when
// the read shows Portainer did not take it up: a server before 3.0 ignores the
// engine field it does not know and deploys Edge stacks instead.
func gitopsCheckEngine(d *schema.ResourceData, wanted string) diag.Diagnostics {
	if wanted == "reconcile" && d.Get("engine").(string) != "reconcile" {
		return diag.Errorf("the reconcile engine was not applied to GitOps workflow %s: Portainer still reports its artifacts as Edge stacks; the reconcile engine requires %s", d.Id(), requiresBE30)
	}
	return nil
}

func resourceGitopsWorkflowCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := map[string]interface{}{
		"name":      d.Get("name").(string),
		"artifacts": gitopsArtifacts(d, false),
	}
	// Both only when configured, so a Portainer 2.45 workflow is created
	// exactly as before.
	wanted, _ := d.Get("engine").(string)
	if wanted != "" {
		payload["engine"] = wanted
	}
	if v, ok := d.GetOk("reconcile_interval"); ok {
		payload["reconcile"] = map[string]interface{}{"interval": v.(string)}
	}

	var workflow struct {
		ID int `json:"id"`
	}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/gitops/workflows", payload, &workflow); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create GitOps workflow %s: %w", d.Get("name").(string), err))
	}

	d.SetId(strconv.Itoa(workflow.ID))
	if diags := resourceGitopsWorkflowRead(ctx, d, meta); diags.HasError() {
		return diags
	}
	return gitopsCheckEngine(d, wanted)
}

func resourceGitopsWorkflowRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var workflow struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		Artifacts []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
			Type string `json:"type"`
		} `json:"artifacts"`
	}
	workflowURL := fmt.Sprintf("%s/gitops/workflows/%s", client.Endpoint, d.Id())
	if err := doJSON(ctx, client, http.MethodGet, workflowURL, nil, &workflow); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read GitOps workflow %s: %w", d.Id(), err))
	}

	// Only the identifiers are read back into the artifact blocks. The update
	// payload needs them, but the rest of an artifact is driven by the
	// configuration: Portainer normalises the file and config fields, and
	// reading them back would turn a stable configuration into a churning plan.
	identifiers := make(map[string]int, len(workflow.Artifacts))
	types := make([]string, 0, len(workflow.Artifacts))
	for _, artifact := range workflow.Artifacts {
		identifiers[artifact.Name] = artifact.ID
		types = append(types, artifact.Type)
	}

	existing, _ := d.Get("artifact").([]interface{})
	artifacts := make([]interface{}, 0, len(existing))
	for _, item := range existing {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		updated := make(map[string]interface{}, len(block))
		for k, v := range block {
			updated[k] = v
		}
		if name, ok := block["name"].(string); ok {
			if id, found := identifiers[name]; found {
				updated["artifact_id"] = id
			}
		}
		artifacts = append(artifacts, updated)
	}

	fields := map[string]interface{}{
		"workflow_id": workflow.ID,
		"name":        workflow.Name,
	}
	if len(artifacts) > 0 {
		fields["artifact"] = artifacts
	}
	// The engine is only recorded once it is known to matter: a reconciled
	// workflow always, an Edge stack one only when an engine is already in
	// state. An unset engine on Portainer 2.45 so stays unset, with no drift.
	if reported := gitopsEngineReported(types); reported == "reconcile" || (reported != "" && d.Get("engine").(string) != "") {
		fields["engine"] = reported
	}

	if err := setFields(d, fields); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceGitopsWorkflowUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := map[string]interface{}{
		"name":      d.Get("name").(string),
		"artifacts": gitopsArtifacts(d, true),
	}
	// engine is a migration rather than a setting: only "reconcile" is
	// accepted, and only sent when the workflow is moving onto it. The
	// interval is sent whenever configured; left out, Portainer keeps the
	// recorded one.
	wanted, _ := d.Get("engine").(string)
	if d.HasChange("engine") && wanted == "reconcile" {
		payload["engine"] = wanted
	}
	if v, ok := d.GetOk("reconcile_interval"); ok {
		payload["reconcile"] = map[string]interface{}{"interval": v.(string)}
	}

	workflowURL := fmt.Sprintf("%s/gitops/workflows/%s", client.Endpoint, d.Id())
	if err := doJSON(ctx, client, http.MethodPut, workflowURL, payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update GitOps workflow %s: %w", d.Id(), err))
	}

	if diags := resourceGitopsWorkflowRead(ctx, d, meta); diags.HasError() {
		return diags
	}
	return gitopsCheckEngine(d, wanted)
}

func resourceGitopsWorkflowDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	mode := d.Get("on_destroy").(string)
	deleteURL := fmt.Sprintf("%s/gitops/workflows/%s/%s", client.Endpoint, d.Id(), mode)
	if err := doJSON(ctx, client, http.MethodDelete, deleteURL, nil, nil); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to %s GitOps workflow %s: %w", mode, d.Id(), err))
	}

	d.SetId("")
	return nil
}
