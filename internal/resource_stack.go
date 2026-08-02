package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/hashicorp/go-cty/cty"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourcePortainerStack() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePortainerStackCreate,
		ReadContext:   resourcePortainerStackRead,
		DeleteContext: resourcePortainerStackDelete,
		UpdateContext: resourcePortainerStackUpdate,
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(30 * time.Minute),
			Update: schema.DefaultTimeout(30 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				// "<endpoint_id>-<stack_id>-<deployment_type>"
				// "<endpoint_id>-<stack_id>-<deployment_type>-<method>"

				parts := strings.Split(d.Id(), "-")
				if len(parts) < 3 {
					return nil, fmt.Errorf("invalid ID format. Use '<endpoint_id>-<stack_id>-<deployment_type>[-<method>]'")
				}

				endpointID, err := strconv.Atoi(parts[0])
				if err != nil {
					return nil, fmt.Errorf("invalid endpoint_id in import ID: %s", parts[0])
				}

				stackID, err := strconv.Atoi(parts[1])
				if err != nil {
					return nil, fmt.Errorf("invalid stack_id in import ID: %s", parts[1])
				}

				deploymentType := parts[2]

				if len(parts) > 3 {
					if err := d.Set("method", parts[3]); err != nil {
						return nil, err
					}
				}
				if err := d.Set("endpoint_id", endpointID); err != nil {
					return nil, err
				}
				if err := d.Set("deployment_type", deploymentType); err != nil {
					return nil, err
				}
				d.SetId(fmt.Sprintf("%d", stackID))
				return []*schema.ResourceData{d}, nil
			},
		},
		Schema: map[string]*schema.Schema{
			"deployment_type": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Deployment mode: 'standalone', 'swarm', or 'kubernetes'",
				ForceNew:    true,
				ValidateFunc: validation.StringInSlice([]string{
					"standalone", "swarm", "kubernetes",
				}, false),
			},
			"method": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Creation method: 'string', 'file', 'repository', or 'url'",
				ForceNew:    true,
				ValidateFunc: validation.StringInSlice([]string{
					"string", "file", "repository", "url",
				}, false),
			},
			"name":        {Type: schema.TypeString, Required: true, ForceNew: true, Description: "Name of the Portainer stack. Must be unique within the target endpoint. Changing this value forces resource recreation."},
			"endpoint_id": {Type: schema.TypeInt, Required: true, ForceNew: true, Description: "Identifier of the Portainer environment (endpoint) where the stack will be deployed. Changing this value forces resource recreation."},
			"swarm_id":    {Type: schema.TypeString, Optional: true, ForceNew: true, Computed: true, Description: "Identifier of the Docker Swarm cluster used when deployment_type is 'swarm'. Automatically fetched from Portainer when not provided. Changing this value forces resource recreation."},
			"namespace":   {Type: schema.TypeString, Optional: true, ForceNew: true, Description: "Kubernetes namespace used when deployment_type is 'kubernetes'. Changing this value forces resource recreation."},
			"stack_file_content": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Inline Compose or Kubernetes manifest content used to deploy the stack. Required when method is 'string'; populated from stack_file_path when method is 'file'.",
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					if d.Get("method").(string) != "file" {
						return false
					}
					path, ok := d.GetOk("stack_file_path")
					if !ok {
						return false
					}
					content, err := os.ReadFile(path.(string))
					if err != nil {
						return false
					}
					current := string(content)
					return strings.TrimSpace(old) == strings.TrimSpace(current)
				},
			},
			"stack_file_path": {Type: schema.TypeString, Optional: true, Description: "Local filesystem path to a Compose or manifest file. Contents are read and uploaded to Portainer when method is 'file'."},
			"additional_files": {
				Type:        schema.TypeList,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "List of additional Compose file paths to use when deploying from Git repository.",
			},
			"git_repository_authentication": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether the Git repository requires authentication. When true, repository_username and repository_password (or their write-only equivalents) are sent to Portainer.",
			},
			"force_update": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to prune unused services/networks during stack update (default: true)",
			},
			"update_interval": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "GitOps auto-update polling interval (e.g. '5m', '1h'). When set, Portainer periodically checks the Git repository for changes and redeploys the stack.",
			},
			"pull_image": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to force pull latest images during stack update (default: true)",
			},
			"stack_webhook": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Enable autoUpdate webhook (GitOps).",
			},
			"webhook_id": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "UUID of the GitOps webhook (read-only).",
			},
			"webhook_url": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Full URL of the webhook trigger",
			},
			"repository_url": {
				Type:          schema.TypeString,
				Optional:      true,
				ForceNew:      true,
				ConflictsWith: []string{"repository_url_wo"},
				Description:   "URL of the Git repository used to deploy the stack when method is 'repository'. Changing this value forces resource recreation.",
			},
			"repository_username": {
				Type:          schema.TypeString,
				Optional:      true,
				ConflictsWith: []string{"repository_username_wo"},
				Description:   "Username used to authenticate against the Git repository.",
			},
			"repository_password": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				ConflictsWith: []string{"repository_password_wo"},
				Description:   "Password or personal access token used to authenticate against the Git repository. Stored in state as a sensitive value.",
			},
			"repository_url_wo": {
				Type:          schema.TypeString,
				Optional:      true,
				WriteOnly:     true,
				Description:   "Write-only repository URL (supports ephemeral values; not stored in Terraform state).",
				ConflictsWith: []string{"repository_url"},
				RequiredWith:  []string{"repository_wo_version"},
			},
			"repository_username_wo": {
				Type:          schema.TypeString,
				Optional:      true,
				WriteOnly:     true,
				Sensitive:     true,
				Description:   "Write-only repository username (supports ephemeral values).",
				ConflictsWith: []string{"repository_username"},
				RequiredWith:  []string{"repository_wo_version"},
			},
			"repository_password_wo": {
				Type:          schema.TypeString,
				Optional:      true,
				WriteOnly:     true,
				Sensitive:     true,
				Description:   "Write-only repository password (supports ephemeral values; not stored in Terraform state).",
				ConflictsWith: []string{"repository_password"},
				RequiredWith:  []string{"repository_wo_version"},
			},
			"repository_wo_version": {
				Type:        schema.TypeInt,
				Optional:    true,
				ForceNew:    true,
				Description: "Version flag for write-only repository credentials; increment to trigger recreation.",
			},
			"repository_reference_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "refs/heads/main",
				Description: "Git reference (branch or tag) used by Portainer when deploying from the repository. Defaults to refs/heads/main.",
			},
			"file_path_in_repository": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "",
				Description: "Path to Compose/manifest file in the repository. Defaults to docker-compose.yml for Docker/Swarm stacks. Not required when helm_chart_path is set.",
			},
			"manifest_url":   {Type: schema.TypeString, Optional: true, ForceNew: true, Description: "URL to a remote Kubernetes manifest used when deployment_type is 'kubernetes' and method is 'url'. Changing this value forces resource recreation."},
			"compose_format": {Type: schema.TypeBool, Optional: true, Default: false, ForceNew: true, Description: "Whether the supplied content is in Docker Compose format (true) instead of native Kubernetes manifest (false). Only applies to Kubernetes stacks. Changing this value forces resource recreation."},
			"helm_chart_path": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Path to a Helm chart folder in the Git repository (must contain Chart.yaml). Only used when deployment_type is 'kubernetes' and method is 'repository'.",
			},
			"additional_helm_values_files": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "List of additional Helm values files (e.g. values-prod.yaml). Only used with helm_chart_path.",
			},
			"support_relative_path": {Type: schema.TypeBool, Optional: true, Default: false, ForceNew: true, Description: "Whether Portainer should support relative paths inside the Compose file for bind mounts referencing repository contents. Changing this value forces resource recreation."},
			"filesystem_path":       {Type: schema.TypeString, Optional: true, Description: "Local filesystem path on the host used when support_relative_path is true. Maps to the repository working directory."},
			"env": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "List of environment variables injected into the stack at deploy time.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name":  {Type: schema.TypeString, Required: true, Description: "Name of the environment variable."},
						"value": {Type: schema.TypeString, Required: true, Description: "Value of the environment variable."},
					},
				},
			},
			"tlsskip_verify": {Type: schema.TypeBool, Optional: true, Computed: true, ForceNew: true, Description: "Whether to skip TLS verification when Portainer connects to the Git repository. Changing this value forces resource recreation."},
			"prune": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to prune unused services/networks during stack update (default: false)",
			},
			"repository_git_credential_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "ID of the shared Git credentials to use for authentication (Portainer < 2.43). Replaced by 'source_id' in Portainer 2.43 STS, which switched to the Sources model; on 2.43+ this field is ignored. Both are sent, so a single configuration stays compatible with old and new Portainer.",
			},
			"source_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "ID of a Portainer Source (Git source) providing the repository URL and credentials. Introduced in Portainer 2.43 STS, which replaced 'repository_git_credential_id'. When set (non-zero), Portainer resolves the repository URL/credentials from the referenced Source and ignores the inline repository_username/repository_password and repository_git_credential_id fields.",
			},
			"resource_control_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Identifier of the Portainer resource control entry associated with the stack. Computed by Portainer.",
			},
			"registries": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "List of registry IDs allowed for this stack.",
				Elem:        &schema.Schema{Type: schema.TypeInt},
			},
			"ownership": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Ownership level: 'public', 'administrators' or 'restricted'.",
				ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
					v := val.(string)
					switch v {
					case "public", "administrators", "restricted", "private":
						return
					}
					errs = append(errs, fmt.Errorf("%q must be one of 'public', 'private', 'administrators', or 'restricted'", key))
					return
				},
			},
			"authorized_teams": {
				Type:        schema.TypeSet,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "List of team IDs authorized to access this stack (only if ownership is restricted).",
			},
			"authorized_users": {
				Type:        schema.TypeSet,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "List of user IDs authorized to access this stack (only if ownership is restricted).",
			},
			"active": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether the stack should be running. Set to false to stop the stack.",
			},
		},
	}
}

func expandStringList(rawList []interface{}) []string {
	result := make([]string, len(rawList))
	for i, v := range rawList {
		result[i] = v.(string)
	}
	return result
}

func expandIntList(rawList []interface{}) []int {
	result := make([]int, len(rawList))
	for i, v := range rawList {
		result[i] = v.(int)
	}
	return result
}

func findExistingStackByName(ctx context.Context, client *APIClient, name string, endpointID int) (int, error) {
	url := fmt.Sprintf("%s/stacks", client.Endpoint)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err := setAuthHeader(req, client); err != nil {
		return 0, err
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("failed to list stacks: %s", string(data))
	}

	var stacks []struct {
		ID         int    `json:"Id"`
		Name       string `json:"Name"`
		EndpointID int    `json:"EndpointId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&stacks); err != nil {
		return 0, err
	}

	for _, s := range stacks {
		if s.Name == name && s.EndpointID == endpointID {
			return s.ID, nil
		}
	}
	return 0, nil // not found
}

func resourcePortainerStackCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	deployment := d.Get("deployment_type").(string)
	method := d.Get("method").(string)
	name := d.Get("name").(string)
	endpointID := d.Get("endpoint_id").(int)

	if deployment == "swarm" && d.Get("swarm_id") == "" {
		swarmID, err := fetchSwarmID(ctx, client, endpointID)
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to fetch swarm_id: %w", err))
		}
		if err := d.Set("swarm_id", swarmID); err != nil {
			return diag.FromErr(err)
		}
	}

	if existingID, err := findExistingStackByName(ctx, client, name, endpointID); err != nil {
		return diag.FromErr(fmt.Errorf("error checking for existing stack: %w", err))
	} else if existingID != 0 {
		d.SetId(strconv.Itoa(existingID))
		return resourcePortainerStackUpdate(ctx, d, meta)
	}

	var err error

	switch deployment {
	case "standalone":
		switch method {
		case "string":
			err = createStackStandaloneString(ctx, d, client)
		case "file":
			path := d.Get("stack_file_path").(string)
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return diag.FromErr(fmt.Errorf("failed to read stack file from path: %w", readErr))
			}
			if err := d.Set("stack_file_content", string(content)); err != nil {
				return diag.FromErr(err)
			}
			err = createStackStandaloneString(ctx, d, client)
		case "repository":
			err = createStackStandaloneRepo(ctx, d, client)
		default:
			return diag.FromErr(fmt.Errorf("invalid method %q for standalone deployment", method))
		}

	case "swarm":
		switch method {
		case "string":
			err = createStackSwarmString(ctx, d, client)
		case "file":
			path := d.Get("stack_file_path").(string)
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return diag.FromErr(fmt.Errorf("failed to read stack file from path: %w", readErr))
			}
			if err := d.Set("stack_file_content", string(content)); err != nil {
				return diag.FromErr(err)
			}
			err = createStackSwarmString(ctx, d, client)
		case "repository":
			err = createStackSwarmRepo(ctx, d, client)
		default:
			return diag.FromErr(fmt.Errorf("invalid method %q for swarm deployment", method))
		}

	case "kubernetes":
		switch method {
		case "string":
			err = createStackK8sString(ctx, d, client)
		case "repository":
			err = createStackK8sRepo(ctx, d, client)
		case "url":
			err = createStackK8sURL(ctx, d, client)
		default:
			return diag.FromErr(fmt.Errorf("invalid method %q for kubernetes deployment", method))
		}

	default:
		return diag.FromErr(fmt.Errorf("invalid deployment_type %q", deployment))
	}

	if err != nil {
		return diag.FromErr(err)
	}

	if id := d.Id(); id == "" || id == "0" {
		resolvedID, lookupErr := findExistingStackByName(ctx, client, name, endpointID)
		if lookupErr != nil {
			return diag.FromErr(fmt.Errorf("stack %q was created but its ID could not be resolved by name on endpoint %d: %w", name, endpointID, lookupErr))
		}
		if resolvedID == 0 {
			return diag.FromErr(fmt.Errorf("stack %q was created but could not be found by name on endpoint %d to resolve its ID", name, endpointID))
		}
		d.SetId(strconv.Itoa(resolvedID))
	}

	if method != "repository" {
		var webhookToken string
		if d.Get("stack_webhook").(bool) {
			webhookToken = d.Get("webhook_id").(string)
			if webhookToken == "" {
				webhookToken = uuid.New().String()
			}
		}

		payload := map[string]interface{}{
			"env":              flattenEnvList(d.Get("env").([]interface{})),
			"stackFileContent": d.Get("stack_file_content").(string),
			"prune":            d.Get("prune").(bool),
			"pullImage":        d.Get("pull_image").(bool),
			"registries":       expandIntList(d.Get("registries").([]interface{})),
		}
		if webhookToken != "" {
			payload["webhook"] = webhookToken
		}

		jsonBody, err := json.Marshal(payload)
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to marshal stack update (create) payload: %w", err))
		}

		url := fmt.Sprintf("%s/stacks/%s?endpointId=%d", client.Endpoint, d.Id(), endpointID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(jsonBody))
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to build stack update (create) request: %w", err))
		}

		if err := setAuthHeader(req, client); err != nil {
			return diag.FromErr(err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to perform stack update (create) request: %w", err))
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			data, _ := io.ReadAll(resp.Body)
			return diag.FromErr(fmt.Errorf("failed to finalize stack creation (prune/webhook), status %d: %s", resp.StatusCode, string(data)))
		}

		if webhookToken != "" {
			baseURL := strings.TrimSuffix(client.Endpoint, "/api")
			webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookToken)
			if err := setFields(d, map[string]interface{}{
				"webhook_id":  webhookToken,
				"webhook_url": webhookURL,
			}); err != nil {
				return diag.FromErr(err)
			}
		}
	}

	// ACCESS CONTROL UPDATE
	if err := updateStackAccessControl(d, client, d.Id()); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update stack access control: %w", err))
	}

	// The Portainer API always deploys a new stack in the running state, so if
	// the user requested active = false we have to stop it right after creation.
	if !d.Get("active").(bool) {
		if err := setStackActive(ctx, client, d.Id(), endpointID, false); err != nil {
			return diag.FromErr(err)
		}
	}

	return resourcePortainerStackRead(ctx, d, meta)
}

func resourcePortainerStackRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	stackID := d.Id()

	url := fmt.Sprintf("%s/stacks/%s", client.Endpoint, stackID)

	var stack struct {
		Name                string `json:"Name"`
		Status              int    `json:"Status"`
		Type                int    `json:"Type"`
		SourceID            int    `json:"SourceID"`
		SwarmID             string `json:"SwarmId"`
		Namespace           string `json:"namespace"`
		ComposeFmt          bool   `json:"composeFormat"`
		Webhook             string `json:"webhook"`
		EndpointID          int    `json:"EndpointId"`
		SupportRelativePath bool   `json:"supportRelativePath"`
		AutoUpdate          *struct {
			Interval       string `json:"Interval"`
			Webhook        string `json:"Webhook"`
			ForcePullImage bool   `json:"ForcePullImage"`
		} `json:"AutoUpdate,omitempty"`
		Env []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"Env"`
		Registries []int `json:"Registries"`

		Option struct {
			Prune bool `json:"prune"`
		} `json:"Option"`

		GitConfig *struct {
			URL             string   `json:"URL"`
			ReferenceName   string   `json:"ReferenceName"`
			ConfigFilePath  string   `json:"ConfigFilePath"`
			AdditionalFiles []string `json:"AdditionalFiles"`
			TLSSkipVerify   bool     `json:"tlsskipVerify"`
			Authentication  struct {
				GitCredentialID int `json:"GitCredentialID"`
			} `json:"Authentication"`
		} `json:"gitConfig,omitempty"`

		HelmConfig *struct {
			ChartPath   string   `json:"chartPath"`
			ValuesFiles []string `json:"valuesFiles"`
		} `json:"helmConfig,omitempty"`

		Portainer struct {
			ResourceControl struct {
				Id int `json:"Id"`
			} `json:"ResourceControl"`
		} `json:"Portainer"`
	}
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &stack); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read stack: %w", err))
	}

	if err := d.Set("name", stack.Name); err != nil {
		return diag.FromErr(err)
	}
	// Portainer API: Status 1 = active, 2 = inactive
	if err := d.Set("active", stack.Status == 1); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("swarm_id", stack.SwarmID); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("namespace", stack.Namespace); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("compose_format", stack.ComposeFmt); err != nil {
		return diag.FromErr(err)
	}

	var webhookToken string
	if stack.AutoUpdate != nil && stack.AutoUpdate.Webhook != "" {
		webhookToken = stack.AutoUpdate.Webhook
	} else if stack.Webhook != "" {
		webhookToken = stack.Webhook
	}

	if webhookToken != "" {
		baseURL := strings.TrimSuffix(client.Endpoint, "/api")
		webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookToken)
		if err := setFields(d, map[string]interface{}{
			"stack_webhook": true,
			"webhook_id":    webhookToken,
			"webhook_url":   webhookURL,
		}); err != nil {
			return diag.FromErr(err)
		}
	} else {
		if err := setFields(d, map[string]interface{}{
			"stack_webhook": false,
			"webhook_id":    "",
			"webhook_url":   "",
		}); err != nil {
			return diag.FromErr(err)
		}
	}

	method := d.Get("method").(string)
	if method != "repository" {
		fileURL := fmt.Sprintf("%s/stacks/%s/file", client.Endpoint, stackID)
		fileReq, _ := http.NewRequestWithContext(ctx, http.MethodGet, fileURL, nil)
		if err := setAuthHeader(fileReq, client); err != nil {
			return diag.FromErr(err)
		}

		fileResp, err := client.HTTPClient.Do(fileReq)
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to fetch stack file: %w", err))
		}
		defer fileResp.Body.Close()

		if fileResp.StatusCode >= 400 {
			body, _ := io.ReadAll(fileResp.Body)
			return diag.FromErr(fmt.Errorf("failed to fetch stack file, status: %d, body: %s", fileResp.StatusCode, string(body)))
		}

		var fileContent struct {
			StackFileContent string `json:"StackFileContent"`
		}
		if err := json.NewDecoder(fileResp.Body).Decode(&fileContent); err != nil {
			return diag.FromErr(fmt.Errorf("failed to decode stack file content: %w", err))
		}

		if err := d.Set("stack_file_content", fileContent.StackFileContent); err != nil {
			return diag.FromErr(err)
		}
	}

	// Env → Terraform
	tfEnvs := make([]map[string]interface{}, 0, len(stack.Env))
	for _, env := range stack.Env {
		tfEnvs = append(tfEnvs, map[string]interface{}{
			"name":  env.Name,
			"value": env.Value,
		})
	}
	if err := d.Set("env", tfEnvs); err != nil {
		return diag.FromErr(err)
	}
	fields := map[string]interface{}{
		"method":                method,
		"endpoint_id":           stack.EndpointID,
		"support_relative_path": stack.SupportRelativePath,
	}
	if stack.SourceID != 0 {
		fields["source_id"] = stack.SourceID
	}
	if method == "repository" && stack.GitConfig != nil {
		fields["tlsskip_verify"] = stack.GitConfig.TLSSkipVerify
		fields["repository_git_credential_id"] = stack.GitConfig.Authentication.GitCredentialID
		if stack.GitConfig.URL != "" {
			fields["repository_url"] = stack.GitConfig.URL
		}
		if stack.GitConfig.ReferenceName != "" {
			fields["repository_reference_name"] = stack.GitConfig.ReferenceName
		}
		if stack.GitConfig.ConfigFilePath != "" {
			fields["file_path_in_repository"] = stack.GitConfig.ConfigFilePath
		}
		if len(stack.GitConfig.AdditionalFiles) > 0 {
			fields["additional_files"] = stack.GitConfig.AdditionalFiles
		}
	}
	if stack.HelmConfig != nil && stack.HelmConfig.ChartPath != "" {
		fields["helm_chart_path"] = stack.HelmConfig.ChartPath
		if len(stack.HelmConfig.ValuesFiles) > 0 {
			fields["additional_helm_values_files"] = stack.HelmConfig.ValuesFiles
		}
	}
	if stack.AutoUpdate != nil {
		fields["pull_image"] = stack.AutoUpdate.ForcePullImage
		fields["update_interval"] = stack.AutoUpdate.Interval
	}
	if stack.Portainer.ResourceControl.Id != 0 {
		fields["resource_control_id"] = stack.Portainer.ResourceControl.Id
	}
	if err := setFields(d, fields); err != nil {
		return diag.FromErr(err)
	}

	if stack.Portainer.ResourceControl.Id != 0 {
		// Read Access Control
		rcID := strconv.Itoa(stack.Portainer.ResourceControl.Id)
		if err := readStackAccessControl(d, client, rcID); err != nil {
			return diag.FromErr(fmt.Errorf("failed to read stack access control: %w", err))
		}
	}

	return nil
}

func fetchSwarmID(ctx context.Context, client *APIClient, endpointID int) (string, error) {
	url := fmt.Sprintf("%s/endpoints/%d/docker/swarm", client.Endpoint, endpointID)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err := setAuthHeader(req, client); err != nil {
		return "", err
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("failed to fetch swarm info: %s", string(data))
	}

	var swarm struct {
		ID string `json:"ID"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&swarm); err != nil {
		return "", err
	}
	return swarm.ID, nil
}

func resourcePortainerStackDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Id()
	endpointID := d.Get("endpoint_id").(int)

	timeout := d.Timeout(schema.TimeoutDelete)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	retryInterval := 15 * time.Second

	for {
		deleteURL := fmt.Sprintf("%s/stacks/%s?endpointId=%d", client.Endpoint, id, endpointID)
		req, err := http.NewRequestWithContext(ctx, http.MethodDelete, deleteURL, nil)
		if err != nil {
			return diag.FromErr(err)
		}
		if err := setAuthHeader(req, client); err != nil {
			return diag.FromErr(err)
		}

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return diag.FromErr(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()

		if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
			return nil
		}

		if resp.StatusCode == http.StatusOK {
			return nil
		}

		// Retry on server errors (5xx)
		if resp.StatusCode >= 500 {
			select {
			case <-ctx.Done():
				return diag.FromErr(fmt.Errorf("timeout deleting stack %s after %s: last error: %s", id, timeout, string(body)))
			case <-time.After(retryInterval):
				continue
			}
		}

		return diag.FromErr(fmt.Errorf("failed to delete stack: %s", string(body)))
	}
}

// setStackActive starts or stops a stack via the Portainer stack start/stop
// endpoints. The Portainer API has no way to create a stack in the stopped
// state, so callers that want active = false must deploy the stack first and
// then call this with active = false to stop it.
func setStackActive(ctx context.Context, client *APIClient, stackID string, endpointID int, active bool) error {
	action := "stop"
	if active {
		action = "start"
	}
	actionURL := fmt.Sprintf("%s/stacks/%s/%s?endpointId=%d", client.Endpoint, stackID, action, endpointID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, actionURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create %s request: %w", action, err)
	}
	if err := setAuthHeader(req, client); err != nil {
		return err
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to %s stack: %w", action, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to %s stack: %s", action, string(body))
	}
	return nil
}

// enforceStackActive applies the desired running state as the final step of an
// update, after any (re)deploy has run. Portainer's update/redeploy endpoints
// always bring the stack back up, so a stack configured active = false must be
// stopped here — regardless of whether "active" itself changed, since a redeploy
// triggered by any other attribute change would otherwise silently restart a
// stack meant to stay stopped (issue #139). active = true needs no action: the
// (re)deploy already left the stack running.
func enforceStackActive(ctx context.Context, client *APIClient, d *schema.ResourceData, endpointID int) error {
	if d.Get("active").(bool) {
		return nil
	}
	return setStackActive(ctx, client, d.Id(), endpointID, false)
}

func resourcePortainerStackUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	stackID := d.Id()
	endpointID := d.Get("endpoint_id").(int)
	method := d.Get("method").(string)

	// NOTE: the desired running state is enforced at the END of this function
	// (see enforceStackActive), not here. Portainer's stack update/redeploy
	// endpoints always bring the stack back up, so stopping before the redeploy
	// would be undone by it — that was the root cause of active=false never
	// persisting on repository stacks (issue #139).

	if method == "file" {
		path := d.Get("stack_file_path").(string)
		content, err := os.ReadFile(path)
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to read stack file for update: %w", err))
		}
		if err := d.Set("stack_file_content", string(content)); err != nil {
			return diag.FromErr(err)
		}
	}

	// ---------------- REPOSITORY STACK ----------------
	if method == "repository" {
		payload := map[string]interface{}{
			"supportRelativePath":       d.Get("support_relative_path").(bool),
			"env":                       flattenEnvList(d.Get("env").([]interface{})),
			"prune":                     d.Get("prune").(bool),
			"pullImage":                 d.Get("pull_image").(bool),
			"repositoryAuthentication":  d.Get("git_repository_authentication").(bool),
			"repositoryUsername":        d.Get("repository_username").(string),
			"repositoryPassword":        d.Get("repository_password").(string),
			"repositoryReferenceName":   d.Get("repository_reference_name").(string),
			"repositoryGitCredentialID": d.Get("repository_git_credential_id").(int),
			"sourceID":                  d.Get("source_id").(int),
			"tlsskipVerify":             d.Get("tlsskip_verify").(bool),
			"additionalFiles":           expandStringList(d.Get("additional_files").([]interface{})),
			"registries":                expandIntList(d.Get("registries").([]interface{})),
		}

		if v, ok := d.GetOk("filesystem_path"); ok {
			payload["filesystemPath"] = v.(string)
		}

		webhookID := ""
		if d.Get("stack_webhook").(bool) {
			webhookID = d.Get("webhook_id").(string)
			if webhookID == "" {
				webhookID = uuid.New().String()
			}

			autoUpdate := map[string]interface{}{
				"ForcePullImage": d.Get("pull_image").(bool),
				"ForceUpdate":    d.Get("force_update").(bool),
				"Interval":       d.Get("update_interval").(string),
				"Webhook":        webhookID,
			}
			payload["AutoUpdate"] = autoUpdate
		} else if v, ok := d.GetOk("update_interval"); ok && v.(string) != "" {
			payload["AutoUpdate"] = map[string]interface{}{
				"ForcePullImage": d.Get("pull_image").(bool),
				"ForceUpdate":    d.Get("force_update").(bool),
				"Interval":       v.(string),
			}
		}

		// Always update git settings via POST /stacks/{id}/git
		// This ensures autoUpdate interval changes are applied
		if err := d.Set("webhook_id", webhookID); err != nil {
			return diag.FromErr(err)
		}
		if webhookID != "" {
			baseURL := strings.TrimSuffix(client.Endpoint, "/api")
			webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookID)
			if err := d.Set("webhook_url", webhookURL); err != nil {
				return diag.FromErr(err)
			}
		}

		jsonBody, err := json.Marshal(payload)
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to marshal git update payload: %w", err))
		}

		url := fmt.Sprintf("%s/stacks/%s/git?endpointId=%d", client.Endpoint, stackID, endpointID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to build git update request: %w", err))
		}
		if err := setAuthHeader(req, client); err != nil {
			return diag.FromErr(err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to perform git update request: %w", err))
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			return diag.FromErr(fmt.Errorf("failed to update git stack settings: %s", string(body)))
		}

		redeployPayload := map[string]interface{}{
			"env":                       flattenEnvList(d.Get("env").([]interface{})),
			"prune":                     d.Get("prune").(bool),
			"pullImage":                 d.Get("pull_image").(bool),
			"repositoryAuthentication":  d.Get("git_repository_authentication").(bool),
			"repositoryUsername":        d.Get("repository_username").(string),
			"repositoryPassword":        d.Get("repository_password").(string),
			"repositoryReferenceName":   d.Get("repository_reference_name").(string),
			"repositoryGitCredentialID": d.Get("repository_git_credential_id").(int),
			"sourceID":                  d.Get("source_id").(int),
			"stackName":                 d.Get("name").(string),
			"additionalFiles":           expandStringList(d.Get("additional_files").([]interface{})),
			"registries":                expandIntList(d.Get("registries").([]interface{})),
		}

		redeployBody, err := json.Marshal(redeployPayload)
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to marshal git redeploy payload: %w", err))
		}

		redeployURL := fmt.Sprintf("%s/stacks/%s/git/redeploy?endpointId=%d", client.Endpoint, stackID, endpointID)
		reqRedeploy, err := http.NewRequestWithContext(ctx, http.MethodPut, redeployURL, bytes.NewBuffer(redeployBody))
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to build git redeploy request: %w", err))
		}
		if err := setAuthHeader(reqRedeploy, client); err != nil {
			return diag.FromErr(err)
		}
		reqRedeploy.Header.Set("Content-Type", "application/json")

		respRedeploy, err := client.HTTPClient.Do(reqRedeploy)
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to perform git redeploy request: %w", err))
		}
		defer respRedeploy.Body.Close()

		if respRedeploy.StatusCode != http.StatusOK {
			data, _ := io.ReadAll(respRedeploy.Body)
			return diag.FromErr(fmt.Errorf("failed to redeploy git stack: %s", string(data)))
		}

		// Apply the desired running state AFTER the git redeploy (which restarts
		// the stack), so active = false actually persists for repository stacks.
		if err := enforceStackActive(ctx, client, d, endpointID); err != nil {
			return diag.FromErr(err)
		}

		return resourcePortainerStackRead(ctx, d, meta)
	}

	if err := updateStackAccessControl(d, client, stackID); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update stack access control: %w", err))
	}

	// ---------------- NON-REPOSITORY STACKS ----------------
	if method != "repository" {
		payload := map[string]interface{}{
			"env":              flattenEnvList(d.Get("env").([]interface{})),
			"stackFileContent": d.Get("stack_file_content").(string),
			"prune":            d.Get("prune").(bool),
			"pullImage":        d.Get("pull_image").(bool),
		}

		jsonBody, err := json.Marshal(payload)
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to marshal standard update payload: %w", err))
		}

		url := fmt.Sprintf("%s/stacks/%s?endpointId=%d", client.Endpoint, stackID, endpointID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(jsonBody))
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to build standard update request: %w", err))
		}
		if err := setAuthHeader(req, client); err != nil {
			return diag.FromErr(err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to perform standard update request: %w", err))
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			data, _ := io.ReadAll(resp.Body)
			return diag.FromErr(fmt.Errorf("failed to update stack: %s", string(data)))
		}
	}

	if d.Get("stack_webhook").(bool) && method != "repository" {
		webhookToken := d.Get("webhook_id").(string)
		if webhookToken == "" {
			webhookToken = uuid.New().String()
		}

		payload := map[string]interface{}{
			"env":              flattenEnvList(d.Get("env").([]interface{})),
			"stackFileContent": d.Get("stack_file_content").(string),
			"prune":            d.Get("prune").(bool),
			"pullImage":        d.Get("pull_image").(bool),
			"webhook":          webhookToken,
			"registries":       expandIntList(d.Get("registries").([]interface{})),
		}

		jsonBody, err := json.Marshal(payload)
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to marshal webhook update payload: %w", err))
		}

		url := fmt.Sprintf("%s/stacks/%s?endpointId=%d", client.Endpoint, d.Id(), endpointID)
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(jsonBody))
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to build webhook update request: %w", err))
		}
		if err := setAuthHeader(req, client); err != nil {
			return diag.FromErr(err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to perform webhook update request: %w", err))
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			data, _ := io.ReadAll(resp.Body)
			return diag.FromErr(fmt.Errorf("failed to update stack webhook, status %d: %s", resp.StatusCode, string(data)))
		}

		baseURL := strings.TrimSuffix(client.Endpoint, "/api")
		webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookToken)
		if err := setFields(d, map[string]interface{}{
			"webhook_id":  webhookToken,
			"webhook_url": webhookURL,
		}); err != nil {
			return diag.FromErr(err)
		}
	}

	// Apply the desired running state AFTER the stack (re)deploy above, so
	// active = false persists instead of being undone by the update.
	if err := enforceStackActive(ctx, client, d, endpointID); err != nil {
		return diag.FromErr(err)
	}

	return resourcePortainerStackRead(ctx, d, meta)
}

func flattenEnvList(envList []interface{}) []map[string]string {
	out := make([]map[string]string, 0, len(envList))
	for _, v := range envList {
		item := v.(map[string]interface{})
		out = append(out, map[string]string{
			"name":  item["name"].(string),
			"value": item["value"].(string),
		})
	}
	return out
}

// --------------------- STANDALONE ----------------------

func createStackStandaloneString(ctx context.Context, d *schema.ResourceData, client *APIClient) error {
	payload := map[string]interface{}{
		"name":             d.Get("name").(string),
		"stackFileContent": d.Get("stack_file_content").(string),
		"env":              flattenEnvList(d.Get("env").([]interface{})),
		"fromAppTemplate":  false,
		"registries":       expandIntList(d.Get("registries").([]interface{})),
	}
	endpointID := d.Get("endpoint_id").(int)
	url := fmt.Sprintf("%s/stacks/create/standalone/string?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err := setAuthHeader(req, client); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create standalone stack: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode stack response: %w", err)
	}
	d.SetId(strconv.Itoa(result.ID))
	return nil
}

func createStackStandaloneRepo(ctx context.Context, d *schema.ResourceData, client *APIClient) error {
	repoURL := d.Get("repository_url").(string)
	repoUser := d.Get("repository_username").(string)
	repoPass := d.Get("repository_password").(string)

	if d.Get("repository_wo_version").(int) != 0 {
		if raw, diag := d.GetRawConfigAt(cty.GetAttrPath("repository_url_wo")); diag == nil && raw.IsKnown() && !raw.IsNull() {
			repoURL = raw.AsString()
		}
		if raw, diag := d.GetRawConfigAt(cty.GetAttrPath("repository_username_wo")); diag == nil && raw.IsKnown() && !raw.IsNull() {
			repoUser = raw.AsString()
		}
		if raw, diag := d.GetRawConfigAt(cty.GetAttrPath("repository_password_wo")); diag == nil && raw.IsKnown() && !raw.IsNull() {
			repoPass = raw.AsString()
		}
	}

	composeFile := d.Get("file_path_in_repository").(string)
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}

	payload := map[string]interface{}{
		"name":                      d.Get("name").(string),
		"composeFile":               composeFile,
		"repositoryURL":             repoURL,
		"repositoryUsername":        repoUser,
		"repositoryPassword":        repoPass,
		"repositoryReferenceName":   d.Get("repository_reference_name").(string),
		"repositoryAuthentication":  d.Get("git_repository_authentication").(bool),
		"repositoryGitCredentialID": d.Get("repository_git_credential_id").(int),
		"sourceID":                  d.Get("source_id").(int),
		"supportRelativePath":       d.Get("support_relative_path").(bool),
		"env":                       flattenEnvList(d.Get("env").([]interface{})),
		"fromAppTemplate":           false,
		"tlsskipVerify":             d.Get("tlsskip_verify").(bool),
		"additionalFiles":           expandStringList(d.Get("additional_files").([]interface{})),
	}

	if v, ok := d.GetOk("filesystem_path"); ok {
		payload["filesystemPath"] = v.(string)
	}

	stackWebhook := d.Get("stack_webhook").(bool)
	if stackWebhook || d.Get("update_interval").(string) != "" {
		webhookID := ""
		if stackWebhook {
			webhookID = uuid.New().String()
		}
		autoUpdate := map[string]interface{}{
			"forcePullImage": d.Get("pull_image").(bool),
			"forceUpdate":    d.Get("force_update").(bool),
			"interval":       d.Get("update_interval").(string),
			"webhook":        webhookID,
		}
		payload["autoUpdate"] = autoUpdate
		if webhookID != "" {
			if err := d.Set("webhook_id", webhookID); err != nil {
				return err
			}
			baseURL := strings.TrimSuffix(client.Endpoint, "/api")
			webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookID)
			if err := d.Set("webhook_url", webhookURL); err != nil {
				return err
			}
		}
	}

	payload["registries"] = expandIntList(d.Get("registries").([]interface{}))
	endpointID := d.Get("endpoint_id").(int)
	url := fmt.Sprintf("%s/stacks/create/standalone/repository?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err := setAuthHeader(req, client); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create standalone stack from repository: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode stack response: %w", err)
	}
	d.SetId(strconv.Itoa(result.ID))
	return nil
}

// --------------------- SWARM ----------------------

func createStackSwarmString(ctx context.Context, d *schema.ResourceData, client *APIClient) error {
	payload := map[string]interface{}{
		"name":             d.Get("name").(string),
		"stackFileContent": d.Get("stack_file_content").(string),
		"env":              flattenEnvList(d.Get("env").([]interface{})),
		"fromAppTemplate":  false,
		"swarmID":          d.Get("swarm_id").(string),
		"registries":       expandIntList(d.Get("registries").([]interface{})),
	}
	endpointID := d.Get("endpoint_id").(int)
	url := fmt.Sprintf("%s/stacks/create/swarm/string?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err := setAuthHeader(req, client); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create swarm stack: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode stack response: %w", err)
	}
	d.SetId(strconv.Itoa(result.ID))
	return nil
}

func createStackSwarmRepo(ctx context.Context, d *schema.ResourceData, client *APIClient) error {
	repoURL := d.Get("repository_url").(string)
	repoUser := d.Get("repository_username").(string)
	repoPass := d.Get("repository_password").(string)

	if d.Get("repository_wo_version").(int) != 0 {
		if raw, diag := d.GetRawConfigAt(cty.GetAttrPath("repository_url_wo")); diag == nil && raw.IsKnown() && !raw.IsNull() {
			repoURL = raw.AsString()
		}
		if raw, diag := d.GetRawConfigAt(cty.GetAttrPath("repository_username_wo")); diag == nil && raw.IsKnown() && !raw.IsNull() {
			repoUser = raw.AsString()
		}
		if raw, diag := d.GetRawConfigAt(cty.GetAttrPath("repository_password_wo")); diag == nil && raw.IsKnown() && !raw.IsNull() {
			repoPass = raw.AsString()
		}
	}

	composeFile := d.Get("file_path_in_repository").(string)
	if composeFile == "" {
		composeFile = "docker-compose.yml"
	}

	payload := map[string]interface{}{
		"name":                      d.Get("name").(string),
		"composeFile":               composeFile,
		"repositoryURL":             repoURL,
		"repositoryUsername":        repoUser,
		"repositoryPassword":        repoPass,
		"repositoryReferenceName":   d.Get("repository_reference_name").(string),
		"repositoryAuthentication":  d.Get("git_repository_authentication").(bool),
		"repositoryGitCredentialID": d.Get("repository_git_credential_id").(int),
		"sourceID":                  d.Get("source_id").(int),
		"supportRelativePath":       d.Get("support_relative_path").(bool),
		"env":                       flattenEnvList(d.Get("env").([]interface{})),
		"fromAppTemplate":           false,
		"tlsskipVerify":             d.Get("tlsskip_verify").(bool),
		"swarmID":                   d.Get("swarm_id").(string),
		"additionalFiles":           expandStringList(d.Get("additional_files").([]interface{})),
	}

	if v, ok := d.GetOk("filesystem_path"); ok {
		payload["filesystemPath"] = v.(string)
	}

	stackWebhook := d.Get("stack_webhook").(bool)
	if stackWebhook || d.Get("update_interval").(string) != "" {
		webhookID := ""
		if stackWebhook {
			webhookID = uuid.New().String()
		}
		autoUpdate := map[string]interface{}{
			"forcePullImage": d.Get("pull_image").(bool),
			"forceUpdate":    d.Get("force_update").(bool),
			"interval":       d.Get("update_interval").(string),
			"webhook":        webhookID,
		}
		payload["autoUpdate"] = autoUpdate
		if webhookID != "" {
			if err := d.Set("webhook_id", webhookID); err != nil {
				return err
			}
			baseURL := strings.TrimSuffix(client.Endpoint, "/api")
			webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookID)
			if err := d.Set("webhook_url", webhookURL); err != nil {
				return err
			}
		}
	}

	payload["registries"] = expandIntList(d.Get("registries").([]interface{}))
	endpointID := d.Get("endpoint_id").(int)
	url := fmt.Sprintf("%s/stacks/create/swarm/repository?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err := setAuthHeader(req, client); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create swarm stack from repository: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode stack response: %w", err)
	}
	d.SetId(strconv.Itoa(result.ID))

	if d.Get("prune").(bool) {
		fmt.Println("[INFO] Performing immediate redeploy with prune=true after stack creation")
		if diags := resourcePortainerStackUpdate(context.Background(), d, client); diags.HasError() {
			fmt.Printf("[WARN] prune redeploy failed: %v\n", diags)
		} else {
			fmt.Println("[INFO] prune redeploy succeeded")
		}
	}

	return nil
}

// --------------------- KUBERNETES ----------------------

func createStackK8sString(ctx context.Context, d *schema.ResourceData, client *APIClient) error {
	payload := map[string]interface{}{
		"stackName":        d.Get("name").(string),
		"stackFileContent": d.Get("stack_file_content").(string),
		"namespace":        d.Get("namespace").(string),
		"composeFormat":    d.Get("compose_format").(bool),
		"fromAppTemplate":  false,
		"registries":       expandIntList(d.Get("registries").([]interface{})),
	}
	endpointID := d.Get("endpoint_id").(int)
	url := fmt.Sprintf("%s/stacks/create/kubernetes/string?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err := setAuthHeader(req, client); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create kubernetes stack from string: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode stack response: %w", err)
	}
	d.SetId(strconv.Itoa(result.ID))
	return nil
}

func createStackK8sRepo(ctx context.Context, d *schema.ResourceData, client *APIClient) error {
	repoURL := d.Get("repository_url").(string)
	repoUser := d.Get("repository_username").(string)
	repoPass := d.Get("repository_password").(string)

	if d.Get("repository_wo_version").(int) != 0 {
		if raw, diag := d.GetRawConfigAt(cty.GetAttrPath("repository_url_wo")); diag == nil && raw.IsKnown() && !raw.IsNull() {
			repoURL = raw.AsString()
		}
		if raw, diag := d.GetRawConfigAt(cty.GetAttrPath("repository_username_wo")); diag == nil && raw.IsKnown() && !raw.IsNull() {
			repoUser = raw.AsString()
		}
		if raw, diag := d.GetRawConfigAt(cty.GetAttrPath("repository_password_wo")); diag == nil && raw.IsKnown() && !raw.IsNull() {
			repoPass = raw.AsString()
		}
	}
	helmChartPath := ""
	if v, ok := d.GetOk("helm_chart_path"); ok {
		helmChartPath = v.(string)
	}

	manifestFile := d.Get("file_path_in_repository").(string)
	if helmChartPath != "" {
		// When deploying via Helm chart, manifestFile is not needed
		manifestFile = ""
	}

	payload := map[string]interface{}{
		"stackName":                 d.Get("name").(string),
		"manifestFile":              manifestFile,
		"namespace":                 d.Get("namespace").(string),
		"composeFormat":             d.Get("compose_format").(bool),
		"repositoryURL":             repoURL,
		"repositoryUsername":        repoUser,
		"repositoryPassword":        repoPass,
		"repositoryReferenceName":   d.Get("repository_reference_name").(string),
		"repositoryAuthentication":  d.Get("git_repository_authentication").(bool),
		"repositoryGitCredentialID": d.Get("repository_git_credential_id").(int),
		"sourceID":                  d.Get("source_id").(int),
		"tlsskipVerify":             d.Get("tlsskip_verify").(bool),
		"fromAppTemplate":           false,
		"additionalFiles":           expandStringList(d.Get("additional_files").([]interface{})),
	}

	if helmChartPath != "" {
		payload["helmChartPath"] = helmChartPath
		if valuesFiles, ok := d.GetOk("additional_helm_values_files"); ok {
			payload["helmValuesFiles"] = expandStringList(valuesFiles.([]interface{}))
		}
	}

	stackWebhook := d.Get("stack_webhook").(bool)
	if stackWebhook || d.Get("update_interval").(string) != "" {
		webhookID := ""
		if stackWebhook {
			webhookID = uuid.New().String()
		}
		autoUpdate := map[string]interface{}{
			"forcePullImage": d.Get("pull_image").(bool),
			"forceUpdate":    d.Get("force_update").(bool),
			"interval":       d.Get("update_interval").(string),
			"webhook":        webhookID,
		}
		payload["autoUpdate"] = autoUpdate
		if webhookID != "" {
			if err := d.Set("webhook_id", webhookID); err != nil {
				return err
			}
			baseURL := strings.TrimSuffix(client.Endpoint, "/api")
			webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookID)
			if err := d.Set("webhook_url", webhookURL); err != nil {
				return err
			}
		}
	}

	payload["registries"] = expandIntList(d.Get("registries").([]interface{}))
	endpointID := d.Get("endpoint_id").(int)
	url := fmt.Sprintf("%s/stacks/create/kubernetes/repository?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err := setAuthHeader(req, client); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create kubernetes stack from repository: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode stack response: %w", err)
	}
	d.SetId(strconv.Itoa(result.ID))
	return nil
}

func createStackK8sURL(ctx context.Context, d *schema.ResourceData, client *APIClient) error {
	payload := map[string]interface{}{
		"stackName":     d.Get("name").(string),
		"manifestURL":   d.Get("manifest_url").(string),
		"namespace":     d.Get("namespace").(string),
		"composeFormat": d.Get("compose_format").(bool),
		"registries":    expandIntList(d.Get("registries").([]interface{})),
	}
	endpointID := d.Get("endpoint_id").(int)
	url := fmt.Sprintf("%s/stacks/create/kubernetes/url?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(jsonBody))
	if err := setAuthHeader(req, client); err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create kubernetes stack from URL: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode stack response: %w", err)
	}
	d.SetId(strconv.Itoa(result.ID))
	return nil
}

func updateStackAccessControl(d *schema.ResourceData, client *APIClient, stackID string) error {
	// If ownership is not set, we might default to something or just return
	// But if d.Get("ownership") is set, it might be "", which is default.
	// But "ownership" is computed so default is likely not "".

	// Check if ownership is set in TF config
	ownership := d.Get("ownership").(string)
	if ownership == "" {
		// No ownership change requested or managed
		return nil
	}

	// Retrieve the ResourceControlID for this stack
	rcIDString, _, err := lookupResourceControlID(client, 6, stackID) // 6 = stack
	if err != nil {
		// If resource control doesn't exist, we can't update it.
		// But stacks usually have one created by default.
		return fmt.Errorf("failed to lookup resource control for stack %s: %w", stackID, err)
	}

	rcID := rcIDString

	// Prepare update payload
	payload := map[string]interface{}{}

	switch ownership {
	case "public":
		payload["public"] = true
		payload["administratorsOnly"] = false
		payload["users"] = []int{}
		payload["teams"] = []int{}
	case "administrators":
		payload["public"] = false
		payload["administratorsOnly"] = true
		payload["users"] = []int{}
		payload["teams"] = []int{}
	case "restricted", "private":
		payload["public"] = false
		payload["administratorsOnly"] = false
		// Only set users/teams if restricted
		if v, ok := d.GetOk("authorized_users"); ok {
			payload["users"] = expandIntSet(v.(*schema.Set))
		} else {
			payload["users"] = []int{}
		}

		if v, ok := d.GetOk("authorized_teams"); ok {
			payload["teams"] = expandIntSet(v.(*schema.Set))
		} else {
			payload["teams"] = []int{}
		}
	}

	if err := doJSON(context.Background(), client, http.MethodPut, fmt.Sprintf("%s/resource_controls/%s", client.Endpoint, rcID), payload, nil); err != nil {
		return fmt.Errorf("failed to update resource control %s: %w", rcID, err)
	}

	return nil
}

func readStackAccessControl(d *schema.ResourceData, client *APIClient, rcID string) error {
	var rc struct {
		AdministratorsOnly bool `json:"AdministratorsOnly"`
		Public             bool `json:"Public"`
		TeamAccesses       []struct {
			TeamID int `json:"TeamId"`
		} `json:"TeamAccesses"`
		UserAccesses []struct {
			UserID int `json:"UserId"`
		} `json:"UserAccesses"`
	}

	if err := doJSON(context.Background(), client, http.MethodGet, fmt.Sprintf("%s/resource_controls/%s", client.Endpoint, rcID), nil, &rc); err != nil {
		return fmt.Errorf("failed to fetch resource control %s: %w", rcID, err)
	}

	if rc.Public {
		if err := d.Set("ownership", "public"); err != nil {
			return err
		}
	} else if rc.AdministratorsOnly {
		if err := d.Set("ownership", "administrators"); err != nil {
			return err
		}
	} else {
		if err := d.Set("ownership", "restricted"); err != nil {
			return err
		}
	}

	users := []int{}
	for _, u := range rc.UserAccesses {
		users = append(users, u.UserID)
	}
	if err := d.Set("authorized_users", users); err != nil {
		return err
	}

	teams := []int{}
	for _, t := range rc.TeamAccesses {
		teams = append(teams, t.TeamID)
	}
	if err := d.Set("authorized_teams", teams); err != nil {
		return err
	}

	return nil
}

func expandIntSet(set *schema.Set) []int {
	result := []int{}
	for _, v := range set.List() {
		result = append(result, v.(int))
	}
	return result
}
