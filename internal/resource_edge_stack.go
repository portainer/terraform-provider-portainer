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
	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceEdgeStack() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceEdgeStackCreate,
		ReadContext:   resourceEdgeStackRead,
		DeleteContext: resourceEdgeStackDelete,
		UpdateContext: resourceEdgeStackUpdate,
		Timeouts: &schema.ResourceTimeout{
			Create: schema.DefaultTimeout(15 * time.Minute),
			Update: schema.DefaultTimeout(15 * time.Minute),
			Delete: schema.DefaultTimeout(10 * time.Minute),
		},
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Name of the Portainer Edge stack.",
			},
			"stack_file_content": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Inline content of the Docker Compose or Kubernetes manifest used to deploy the Edge stack.",
			},
			"stack_file_path": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Local filesystem path to a Compose or manifest file used as the Edge stack definition. Changing this value forces resource recreation.",
			},
			"pre_pull_image": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether Portainer should pre-pull images on Edge agents before deploying the stack.",
			},
			"retry_deploy": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether the Edge agent should retry deployment on failure.",
			},
			"dryrun": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "If true, perform a dry-run of the Edge stack deployment without applying changes.",
			},
			"repository_url": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Git repository URL containing the Edge stack definition. Changing this value forces resource recreation.",
			},
			"git_repository_authentication": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether authentication is required to clone the Git repository.",
			},
			"repository_username": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Description: "Username used to authenticate against the Git repository. Changing this value forces resource recreation.",
			},
			"repository_password": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Sensitive:   true,
				Description: "Password or personal access token used to authenticate against the Git repository. Stored in state as sensitive value. Changing this value forces resource recreation.",
			},
			"repository_reference_name": {
				Type:        schema.TypeString,
				Optional:    true,
				Default:     "refs/heads/main",
				Description: "Git reference (branch or tag) to check out from the repository.",
			},
			"file_path_in_repository": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Default:     "docker-compose.yml",
				Description: "Path to the Compose or manifest file within the Git repository. Changing this value forces resource recreation.",
			},
			"deployment_type": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "0 = Docker Compose, 1 = Kubernetes",
			},
			"edge_groups": {
				Type:        schema.TypeList,
				Required:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "List of Portainer Edge group IDs to which this Edge stack is deployed.",
			},
			"registries": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "List of Portainer registry IDs used to pull images for this Edge stack.",
			},
			"use_manifest_namespaces": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "For Kubernetes deployments, whether to use namespaces declared inside the manifest instead of the default Portainer namespace.",
			},
			"stack_webhook": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Enable autoUpdate webhook (GitOps).",
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
				Description: "GitOps update interval (e.g. \"5m\") at which Portainer polls the Git repository for Edge stack changes.",
			},
			"pull_image": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to force pull latest images during stack update (default: true)",
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
			"environment": {
				Type:        schema.TypeMap,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Environment variables for the Edge Stack",
			},
			"relative_path": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Enable relative path volumes – also used as value for 'filesystemPath'.",
				Default:     "",
				ForceNew:    true,
			},
			"repository_git_credential_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "ID of the Git credentials to use for authentication.",
			},
			"always_clone": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether the agent must always clone the git repository for relative path. Only valid when relative_path is set.",
			},
		},
	}
}

func setAuthHeaders(client *APIClient, req *http.Request) {
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	}
}

func buildEnvVars(d *schema.ResourceData) []map[string]string {
	envVars := []map[string]string{}
	if envMap, ok := d.GetOk("environment"); ok {
		for k, v := range envMap.(map[string]interface{}) {
			envVars = append(envVars, map[string]string{
				"name":  k,
				"value": v.(string),
			})
		}
	}
	return envVars
}

func findExistingEdgeStackByName(ctx context.Context, client *APIClient, name string) (int, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/edge_stacks", client.Endpoint), nil)
	if err != nil {
		return 0, err
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return 0, fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("failed to list edge stacks: %s", string(data))
	}

	var stacks []map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&stacks); err != nil {
		return 0, err
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

func resourceEdgeStackCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	timeout := d.Timeout(schema.TimeoutCreate)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := meta.(*APIClient)
	edgeGroups := toIntSlice(d.Get("edge_groups").([]interface{}))
	registries := toIntSlice(d.Get("registries").([]interface{}))
	name := d.Get("name").(string)
	deployType := d.Get("deployment_type").(int)
	useManifest := d.Get("use_manifest_namespaces").(bool)

	if existingID, err := findExistingEdgeStackByName(ctx, client, name); err != nil {
		return diag.FromErr(fmt.Errorf("failed to check for existing edge stack: %w", err))
	} else if existingID != 0 {
		d.SetId(strconv.Itoa(existingID))
		return resourceEdgeStackUpdate(ctx, d, meta)
	}

	// Method: stackFileContent (string)
	if content, ok := d.GetOk("stack_file_content"); ok {
		payload := map[string]interface{}{
			"name":                  name,
			"deploymentType":        deployType,
			"edgeGroups":            edgeGroups,
			"stackFileContent":      content.(string),
			"useManifestNamespaces": useManifest,
			"registries":            registries,
		}
		if envMap, ok := d.GetOk("environment"); ok {
			envVars := []map[string]string{}
			for k, v := range envMap.(map[string]interface{}) {
				envVars = append(envVars, map[string]string{
					"name":  k,
					"value": v.(string),
				})
			}
			payload["envVars"] = envVars
		}
		return diag.FromErr(createEdgeStackFromJSON(ctx, client, d, payload, "/edge_stacks/create/string"))
	}

	// Method: stackFilePath (file)
	if filePathRaw, ok := d.GetOk("stack_file_path"); ok {
		filePath := filePathRaw.(string)
		file, err := os.Open(filePath)
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to open stack file: %w", err))
		}
		defer file.Close()

		body := &bytes.Buffer{}
		writer := multipart.NewWriter(body)

		_ = writer.WriteField("Name", name)
		_ = writer.WriteField("DeploymentType", strconv.Itoa(deployType))
		_ = writer.WriteField("EdgeGroups", toJSONString(edgeGroups))
		_ = writer.WriteField("UseManifestNamespaces", strconv.FormatBool(useManifest))
		_ = writer.WriteField("Registries", toJSONString(registries))
		_ = writer.WriteField("PrePullImage", strconv.FormatBool(d.Get("pre_pull_image").(bool)))
		_ = writer.WriteField("RetryDeploy", strconv.FormatBool(d.Get("retry_deploy").(bool)))

		part, err := writer.CreateFormFile("file", filepath.Base(filePath))
		if err != nil {
			return diag.FromErr(err)
		}
		_, _ = io.Copy(part, file)
		writer.Close()

		// Build query string for dryrun
		endpoint := fmt.Sprintf("%s/edge_stacks/create/file", client.Endpoint)
		if d.Get("dryrun").(bool) {
			endpoint += "?dryrun=true"
		}
		req, _ := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, body)
		if client.APIKey != "" {
			req.Header.Set("X-API-Key", client.APIKey)
		} else if client.JWTToken != "" {
			req.Header.Set("Authorization", "Bearer "+client.JWTToken)
		} else {
			return diag.FromErr(fmt.Errorf("no valid authentication method provided (api_key or jwt token)"))
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return diag.FromErr(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			data, _ := io.ReadAll(resp.Body)
			return diag.FromErr(fmt.Errorf("failed to create edge stack from file: %s", string(data)))
		}

		var result struct {
			ID int `json:"Id"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&result)

		if !d.Get("dryrun").(bool) {
			d.SetId(strconv.Itoa(result.ID))
			return resourceEdgeStackRead(ctx, d, meta)
		}

		return nil
	}

	// Method: repository
	if repoURLRaw, ok := d.GetOk("repository_url"); ok {
		repoURL := repoURLRaw.(string)
		payload := map[string]interface{}{
			"name":                      name,
			"deploymentType":            deployType,
			"edgeGroups":                edgeGroups,
			"repositoryURL":             repoURL,
			"repositoryAuthentication":  d.Get("git_repository_authentication").(bool),
			"repositoryUsername":        d.Get("repository_username").(string),
			"repositoryPassword":        d.Get("repository_password").(string),
			"repositoryReferenceName":   d.Get("repository_reference_name").(string),
			"repositoryGitCredentialID": d.Get("repository_git_credential_id").(int),
			"filePathInRepository":      d.Get("file_path_in_repository").(string),
			"useManifestNamespaces":     useManifest,
			"registries":                registries,
		}

		if relPath, ok := d.GetOk("relative_path"); ok && relPath.(string) != "" {
			payload["supportRelativePath"] = true
			payload["filesystemPath"] = relPath.(string)
			payload["AlwaysCloneGitRepoForRelativePath"] = d.Get("always_clone").(bool)
		}

		if envMap, ok := d.GetOk("environment"); ok {
			envVars := []map[string]string{}
			for k, v := range envMap.(map[string]interface{}) {
				envVars = append(envVars, map[string]string{
					"name":  k,
					"value": v.(string),
				})
			}
			payload["envVars"] = envVars
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
					return diag.FromErr(err)
				}
				baseURL := strings.TrimSuffix(client.Endpoint, "/api")
				webhookURL := fmt.Sprintf("%s/api/edge_stacks/webhooks/%s", baseURL, webhookID)
				if err := d.Set("webhook_url", webhookURL); err != nil {
					return diag.FromErr(err)
				}
			}
		}
		return diag.FromErr(createEdgeStackFromJSON(ctx, client, d, payload, "/edge_stacks/create/repository"))
	}

	return diag.FromErr(fmt.Errorf("one of 'stack_file_content', 'stack_file_path', or 'repository_url' must be provided"))
}

func resourceEdgeStackUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	timeout := d.Timeout(schema.TimeoutUpdate)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := meta.(*APIClient)
	deploymentType := d.Get("deployment_type").(int)

	if _, hasFile := d.GetOk("stack_file_content"); hasFile || d.Get("stack_file_path").(string) != "" {
		payload := map[string]interface{}{
			"name":                  d.Get("name").(string),
			"deploymentType":        deploymentType,
			"edgeGroups":            toIntSlice(d.Get("edge_groups").([]interface{})),
			"updateVersion":         true,
			"useManifestNamespaces": d.Get("use_manifest_namespaces").(bool),
			"envVars":               buildEnvVars(d),
			"prePullImage":          d.Get("pre_pull_image").(bool),
			"rePullImage":           d.Get("pull_image").(bool),
			"registries":            toIntSlice(d.Get("registries").([]interface{})),
		}

		if v, ok := d.GetOk("stack_file_content"); ok {
			payload["stackFileContent"] = v.(string)
		}

		jsonBody, err := json.Marshal(payload)
		if err != nil {
			return diag.FromErr(err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s/edge_stacks/%s", client.Endpoint, d.Id()), bytes.NewBuffer(jsonBody))
		if err != nil {
			return diag.FromErr(err)
		}
		setAuthHeaders(client, req)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return diag.FromErr(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			data, _ := io.ReadAll(resp.Body)
			return diag.FromErr(fmt.Errorf("failed to update edge stack: %s", string(data)))
		}

		return resourceEdgeStackRead(ctx, d, meta)
	}

	// Repository-based update via /git
	// Portainer's PUT /edge_stacks/{id}/git endpoint silently ignores any
	// `repositoryURL` / `filePathInRepository` fields in the payload — only
	// `refName` and the authentication block are honored. Changing the source
	// URL or in-repo file path requires recreating the stack (enforced via
	// ForceNew on the matching schema attributes).
	if repoURL, ok := d.GetOk("repository_url"); ok && repoURL.(string) != "" {
		payload := map[string]interface{}{
			"deploymentType": deploymentType,
			"groupIds":       toIntSlice(d.Get("edge_groups").([]interface{})),
			"refName":        d.Get("repository_reference_name").(string),
			"envVars":        buildEnvVars(d),
			"updateVersion":  true,
			"prePullImage":   d.Get("pre_pull_image").(bool),
			"rePullImage":    d.Get("pull_image").(bool),
			"registries":     toIntSlice(d.Get("registries").([]interface{})),
			"retryDeploy":    d.Get("retry_deploy").(bool),
		}

		if relPath, ok := d.GetOk("relative_path"); ok && relPath.(string) != "" {
			payload["supportRelativePath"] = true
			payload["filesystemPath"] = relPath.(string)
			payload["AlwaysCloneGitRepoForRelativePath"] = d.Get("always_clone").(bool)
		}

		if d.Get("git_repository_authentication").(bool) {
			payload["authentication"] = map[string]interface{}{
				"username":        d.Get("repository_username").(string),
				"password":        d.Get("repository_password").(string),
				"gitCredentialID": d.Get("repository_git_credential_id").(int),
			}
		}

		if d.Get("stack_webhook").(bool) || d.Get("update_interval").(string) != "" {
			webhookID := ""
			if d.Get("stack_webhook").(bool) {
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
					return diag.FromErr(err)
				}
				baseURL := strings.TrimSuffix(client.Endpoint, "/api")
				webhookURL := fmt.Sprintf("%s/api/edge_stacks/webhooks/%s", baseURL, webhookID)
				if err := d.Set("webhook_url", webhookURL); err != nil {
					return diag.FromErr(err)
				}
			}
		}

		jsonBody, err := json.Marshal(payload)
		if err != nil {
			return diag.FromErr(err)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodPut, fmt.Sprintf("%s/edge_stacks/%s/git", client.Endpoint, d.Id()), bytes.NewBuffer(jsonBody))
		if err != nil {
			return diag.FromErr(err)
		}
		setAuthHeaders(client, req)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return diag.FromErr(err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			data, _ := io.ReadAll(resp.Body)
			return diag.FromErr(fmt.Errorf("failed to update repository-based edge stack: %s", string(data)))
		}

		return resourceEdgeStackRead(ctx, d, meta)
	}

	return diag.FromErr(fmt.Errorf("one of 'stack_file_content', 'stack_file_path', or 'repository_url' must be provided for update"))
}

func createEdgeStackFromJSON(ctx context.Context, client *APIClient, d *schema.ResourceData, payload map[string]interface{}, endpoint string) error {
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, client.Endpoint+endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create edge stack: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	d.SetId(strconv.Itoa(result.ID))
	if diags := resourceEdgeStackRead(ctx, d, client); diags.HasError() {
		return fmt.Errorf("%s", diags[0].Summary)
	}
	return nil
}

func toJSONString(input interface{}) string {
	data, _ := json.Marshal(input)
	return string(data)
}

func resourceEdgeStackRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, fmt.Sprintf("%s/edge_stacks/%s", client.Endpoint, d.Id()), nil)
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return diag.FromErr(fmt.Errorf("no valid authentication method provided (api_key or jwt token)"))
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		d.SetId("")
		return nil
	} else if resp.StatusCode != http.StatusOK {
		data, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to read edge stack: %s", string(data)))
	}

	var stack struct {
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
		EnvVars                           []struct {
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
	}

	if err := json.NewDecoder(resp.Body).Decode(&stack); err != nil {
		return diag.FromErr(err)
	}

	if err := d.Set("name", stack.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("deployment_type", stack.DeploymentType); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("dryrun", false); err != nil { // write-only creation flag, API never returns it
		return diag.FromErr(err)
	}
	if err := d.Set("edge_groups", stack.EdgeGroups); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("registries", stack.Registries); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("use_manifest_namespaces", stack.UseManifestNamespaces); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("pre_pull_image", stack.PrePullImage); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("retry_deploy", stack.RetryDeploy); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("always_clone", stack.AlwaysCloneGitRepoForRelativePath); err != nil {
		return diag.FromErr(err)
	}

	envMap := make(map[string]string, len(stack.EnvVars))
	for _, env := range stack.EnvVars {
		envMap[env.Name] = env.Value
	}

	if len(envMap) > 0 {
		if err := d.Set("environment", envMap); err != nil {
			return diag.FromErr(err)
		}
	}

	if stack.SupportRelativePath {
		if err := d.Set("relative_path", stack.FilesystemPath); err != nil {
			return diag.FromErr(err)
		}
	}

	if stack.GitConfig != nil {
		if err := d.Set("repository_url", stack.GitConfig.URL); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("repository_reference_name", stack.GitConfig.ReferenceName); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("file_path_in_repository", stack.GitConfig.ConfigFilePath); err != nil {
			return diag.FromErr(err)
		}
		if stack.GitConfig.Authentication != nil {
			if err := d.Set("git_repository_authentication", true); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set("repository_username", stack.GitConfig.Authentication.Username); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set("repository_git_credential_id", stack.GitConfig.Authentication.GitCredentialID); err != nil {
				return diag.FromErr(err)
			}
		} else {
			if err := d.Set("git_repository_authentication", false); err != nil {
				return diag.FromErr(err)
			}
		}
	}

	// pull_image is sent to two different places in Create/Update payloads:
	// top-level rePullImage (always) and autoUpdate.forcePullImage (only when
	// GitOps is configured). The API persists both. Use top-level RePullImage
	// as the source of truth, and let AutoUpdate.ForcePullImage override when
	// AutoUpdate exists since that's the value the user-configured GitOps block
	// carries.
	if err := d.Set("pull_image", stack.RePullImage); err != nil {
		return diag.FromErr(err)
	}

	if stack.AutoUpdate != nil {
		if err := d.Set("update_interval", stack.AutoUpdate.Interval); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("pull_image", stack.AutoUpdate.ForcePullImage); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("force_update", stack.AutoUpdate.ForceUpdate); err != nil {
			return diag.FromErr(err)
		}
		if stack.AutoUpdate.Webhook != "" {
			if err := d.Set("stack_webhook", true); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set("webhook_id", stack.AutoUpdate.Webhook); err != nil {
				return diag.FromErr(err)
			}
			baseURL := strings.TrimSuffix(client.Endpoint, "/api")
			if err := d.Set("webhook_url", fmt.Sprintf("%s/api/edge_stacks/webhooks/%s", baseURL, stack.AutoUpdate.Webhook)); err != nil {
				return diag.FromErr(err)
			}
		} else {
			// AutoUpdate exists but webhook was cleared — drop any stale
			// computed outputs so state reflects reality.
			if err := d.Set("stack_webhook", false); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set("webhook_id", ""); err != nil {
				return diag.FromErr(err)
			}
			if err := d.Set("webhook_url", ""); err != nil {
				return diag.FromErr(err)
			}
		}
	} else {
		// No GitOps configured — clear all AutoUpdate-derived fields,
		// including webhook outputs that may have been set previously.
		if err := d.Set("force_update", false); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("stack_webhook", false); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("update_interval", ""); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("webhook_id", ""); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("webhook_url", ""); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourceEdgeStackDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	timeout := d.Timeout(schema.TimeoutDelete)
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	client := meta.(*APIClient)

	req, _ := http.NewRequestWithContext(ctx, http.MethodDelete, fmt.Sprintf("%s/edge_stacks/%s", client.Endpoint, d.Id()), nil)
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return diag.FromErr(fmt.Errorf("no valid authentication method provided (api_key or jwt token)"))
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNoContent || resp.StatusCode == http.StatusNotFound {
		return nil
	}

	data, _ := io.ReadAll(resp.Body)
	return diag.FromErr(fmt.Errorf("failed to delete edge stack: %s", string(data)))
}
