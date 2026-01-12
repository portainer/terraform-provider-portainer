package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceEdgeStack() *schema.Resource {
	return &schema.Resource{
		Create: resourceEdgeStackCreate,
		Read:   resourceEdgeStackRead,
		Delete: resourceEdgeStackDelete,
		Update: resourceEdgeStackUpdate,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"stack_file_content": {
				Type:     schema.TypeString,
				Optional: true,
			},
			"stack_file_path": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"pre_pull_image": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"retry_deploy": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"dryrun": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"repository_url": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"git_repository_authentication": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"repository_username": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"repository_password": {
				Type:      schema.TypeString,
				Optional:  true,
				ForceNew:  true,
				Sensitive: true,
			},
			"repository_reference_name": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "refs/heads/main",
			},
			"file_path_in_repository": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "docker-compose.yml",
			},
			"deployment_type": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "0 = Docker Compose, 1 = Kubernetes",
			},
			"edge_groups": {
				Type:     schema.TypeList,
				Required: true,
				Elem:     &schema.Schema{Type: schema.TypeInt},
			},
			"registries": {
				Type:     schema.TypeList,
				Optional: true,
				Elem:     &schema.Schema{Type: schema.TypeInt},
			},
			"use_manifest_namespaces": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
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
				Type:     schema.TypeString,
				Optional: true,
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

func findExistingEdgeStackByName(client *APIClient, name string) (int, error) {
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/edge_stacks", client.Endpoint), nil)
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

	if resp.StatusCode != 200 {
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

func resourceEdgeStackCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	edgeGroups := toIntSlice(d.Get("edge_groups").([]interface{}))
	registries := toIntSlice(d.Get("registries").([]interface{}))
	name := d.Get("name").(string)
	deployType := d.Get("deployment_type").(int)
	useManifest := d.Get("use_manifest_namespaces").(bool)

	if existingID, err := findExistingEdgeStackByName(client, name); err != nil {
		return fmt.Errorf("failed to check for existing edge stack: %w", err)
	} else if existingID != 0 {
		d.SetId(strconv.Itoa(existingID))
		return resourceEdgeStackUpdate(d, meta)
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
		return createEdgeStackFromJSON(client, d, payload, "/edge_stacks/create/string")
	}

	// Method: stackFilePath (file)
	if filePathRaw, ok := d.GetOk("stack_file_path"); ok {
		filePath := filePathRaw.(string)
		file, err := os.Open(filePath)
		if err != nil {
			return fmt.Errorf("failed to open stack file: %w", err)
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
			return err
		}
		_, _ = io.Copy(part, file)
		writer.Close()

		// Build query string for dryrun
		endpoint := fmt.Sprintf("%s/edge_stacks/create/file", client.Endpoint)
		if d.Get("dryrun").(bool) {
			endpoint += "?dryrun=true"
		}
		req, _ := http.NewRequest("POST", endpoint, body)
		if client.APIKey != "" {
			req.Header.Set("X-API-Key", client.APIKey)
		} else if client.JWTToken != "" {
			req.Header.Set("Authorization", "Bearer "+client.JWTToken)
		} else {
			return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
		}
		req.Header.Set("Content-Type", writer.FormDataContentType())

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			data, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to create edge stack from file: %s", string(data))
		}

		var result struct {
			ID int `json:"Id"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&result)

		if !d.Get("dryrun").(bool) {
			d.SetId(strconv.Itoa(result.ID))
			return resourceEdgeStackRead(d, meta)
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
		if stackWebhook {
			webhookID := uuid.New().String()
			autoUpdate := map[string]interface{}{
				"forcePullImage": d.Get("pull_image").(bool),
				"forceUpdate":    d.Get("force_update").(bool),
				"interval":       d.Get("update_interval").(string),
				"webhook":        webhookID,
			}
			payload["autoUpdate"] = autoUpdate
			d.Set("webhook_id", webhookID)
			baseURL := strings.TrimSuffix(client.Endpoint, "/api")
			webhookURL := fmt.Sprintf("%s/api/edge_stacks/webhooks/%s", baseURL, webhookID)
			d.Set("webhook_url", webhookURL)
		}
		return createEdgeStackFromJSON(client, d, payload, "/edge_stacks/create/repository")
	}

	return fmt.Errorf("one of 'stack_file_content', 'stack_file_path', or 'repository_url' must be provided")
}

func resourceEdgeStackUpdate(d *schema.ResourceData, meta interface{}) error {
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
			return err
		}

		req, err := http.NewRequest("PUT", fmt.Sprintf("%s/edge_stacks/%s", client.Endpoint, d.Id()), bytes.NewBuffer(jsonBody))
		if err != nil {
			return err
		}
		setAuthHeaders(client, req)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			data, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to update edge stack: %s", string(data))
		}

		return resourceEdgeStackRead(d, meta)
	}

	// Repository-based update via /git
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
		}

		if d.Get("git_repository_authentication").(bool) {
			payload["authentication"] = map[string]interface{}{
				"username":        d.Get("repository_username").(string),
				"password":        d.Get("repository_password").(string),
				"gitCredentialID": d.Get("repository_git_credential_id").(int),
			}
		}

		if d.Get("stack_webhook").(bool) {
			webhookID := uuid.New().String()
			autoUpdate := map[string]interface{}{
				"forcePullImage": d.Get("pull_image").(bool),
				"forceUpdate":    d.Get("force_update").(bool),
				"interval":       d.Get("update_interval").(string),
				"webhook":        webhookID,
			}
			payload["autoUpdate"] = autoUpdate

			d.Set("webhook_id", webhookID)
			baseURL := strings.TrimSuffix(client.Endpoint, "/api")
			webhookURL := fmt.Sprintf("%s/api/edge_stacks/webhooks/%s", baseURL, webhookID)
			d.Set("webhook_url", webhookURL)
		}

		jsonBody, err := json.Marshal(payload)
		if err != nil {
			return err
		}

		req, err := http.NewRequest("PUT", fmt.Sprintf("%s/edge_stacks/%s/git", client.Endpoint, d.Id()), bytes.NewBuffer(jsonBody))
		if err != nil {
			return err
		}
		setAuthHeaders(client, req)
		req.Header.Set("Content-Type", "application/json")

		resp, err := client.HTTPClient.Do(req)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			data, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to update repository-based edge stack: %s", string(data))
		}

		return resourceEdgeStackRead(d, meta)
	}

	return fmt.Errorf("one of 'stack_file_content', 'stack_file_path', or 'repository_url' must be provided for update")
}

func createEdgeStackFromJSON(client *APIClient, d *schema.ResourceData, payload map[string]interface{}, endpoint string) error {
	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("POST", client.Endpoint+endpoint, bytes.NewBuffer(jsonBody))
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
	return resourceEdgeStackRead(d, client)
}

func toJSONString(input interface{}) string {
	data, _ := json.Marshal(input)
	return string(data)
}

func resourceEdgeStackRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	req, _ := http.NewRequest("GET", fmt.Sprintf("%s/edge_stacks/%s", client.Endpoint, d.Id()), nil)
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	} else if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read edge stack: %s", string(data))
	}

	var stack struct {
		Name    string `json:"Name"`
		EnvVars []struct {
			Name  string `json:"name"`
			Value string `json:"value"`
		} `json:"envVars"`
		GitConfig *struct {
			Authentication struct {
				GitCredentialID int `json:"GitCredentialID"`
			} `json:"Authentication"`
		} `json:"GitConfig"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&stack); err != nil {
		return err
	}

	d.Set("name", stack.Name)
	envMap := make(map[string]string)
	for _, env := range stack.EnvVars {
		envMap[env.Name] = env.Value
	}
	if len(envMap) > 0 {
		d.Set("environment", envMap)
	}

	if stack.GitConfig != nil {
		d.Set("repository_git_credential_id", stack.GitConfig.Authentication.GitCredentialID)
	}

	return nil
}

func resourceEdgeStackDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	req, _ := http.NewRequest("DELETE", fmt.Sprintf("%s/edge_stacks/%s", client.Endpoint, d.Id()), nil)
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 || resp.StatusCode == 404 {
		return nil
	}

	data, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("failed to delete edge stack: %s", string(data))
}
