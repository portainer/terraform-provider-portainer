package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

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
func resourceGitopsWorkflow() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceGitopsWorkflowCreate,
		ReadContext:   resourceGitopsWorkflowRead,
		UpdateContext: resourceGitopsWorkflowUpdate,
		DeleteContext: resourceGitopsWorkflowDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the workflow.",
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
							Description:  "What the artifact deploys as: `stack` or `edgeStack`. Defaults to `edgeStack`.",
						},
						"deployment_type": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Deployment type of the artifact, for example `compose` or `kubernetes`.",
						},
						"edge_group_ids": {
							Type:        schema.TypeList,
							Required:    true,
							Elem:        &schema.Schema{Type: schema.TypeInt},
							Description: "Edge groups the artifact is deployed to.",
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
							Description: "Identifier Portainer assigned to the artifact, which the update call needs.",
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

// gitopsArtifactConfig builds the config half of an artifact. Each field is
// omitted when empty so a workflow never sends a zero value Portainer would
// read as a deliberate setting.
func gitopsArtifactConfig(raw interface{}) (map[string]interface{}, bool) {
	list, _ := raw.([]interface{})
	if len(list) == 0 {
		return nil, false
	}
	block, ok := list[0].(map[string]interface{})
	if !ok {
		return nil, false
	}

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
	} {
		if v, ok := block[field].(string); ok && v != "" {
			config[key] = v
		}
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

// gitopsArtifacts builds the artifact list. On update Portainer keys artifacts
// by their identifier and takes their type, where on create it takes the name
// and deployment type - so the two payloads differ by more than a field name.
// The update payload has no name field, which is why the name is ForceNew.
func gitopsArtifacts(d *schema.ResourceData, forUpdate bool) []map[string]interface{} {
	list, _ := d.Get("artifact").([]interface{})
	artifacts := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}

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

		artifact := map[string]interface{}{
			"files": encodedFiles,
			"targets": map[string]interface{}{
				"edgeGroups": toIntSlice(block["edge_group_ids"].([]interface{})),
			},
		}
		if forUpdate {
			artifact["id"] = block["artifact_id"]
			artifact["type"] = block["type"]
		} else {
			artifact["name"] = block["name"]
		}
		if v, ok := block["deployment_type"].(string); ok && v != "" {
			artifact["deploymentType"] = v
		}
		if config, ok := gitopsArtifactConfig(block["config"]); ok {
			artifact["config"] = config
		}

		artifacts = append(artifacts, artifact)
	}
	return artifacts
}

func resourceGitopsWorkflowCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := map[string]interface{}{
		"name":      d.Get("name").(string),
		"artifacts": gitopsArtifacts(d, false),
	}

	var workflow struct {
		ID int `json:"id"`
	}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/gitops/workflows", payload, &workflow); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create GitOps workflow %s: %w", d.Get("name").(string), err))
	}

	d.SetId(strconv.Itoa(workflow.ID))
	return resourceGitopsWorkflowRead(ctx, d, meta)
}

func resourceGitopsWorkflowRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var workflow struct {
		ID        int    `json:"id"`
		Name      string `json:"name"`
		Artifacts []struct {
			ID   int    `json:"id"`
			Name string `json:"name"`
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
	for _, artifact := range workflow.Artifacts {
		identifiers[artifact.Name] = artifact.ID
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

	workflowURL := fmt.Sprintf("%s/gitops/workflows/%s", client.Endpoint, d.Id())
	if err := doJSON(ctx, client, http.MethodPut, workflowURL, payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update GitOps workflow %s: %w", d.Id(), err))
	}

	return resourceGitopsWorkflowRead(ctx, d, meta)
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
