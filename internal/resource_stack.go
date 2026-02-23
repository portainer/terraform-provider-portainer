package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"

	"github.com/hashicorp/go-cty/cty"

	"github.com/google/uuid"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourcePortainerStack() *schema.Resource {
	return &schema.Resource{
		Create: resourcePortainerStackCreate,
		Read:   resourcePortainerStackRead,
		Delete: resourcePortainerStackDelete,
		Update: resourcePortainerStackUpdate,
		Importer: &schema.ResourceImporter{
			State: func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
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
					d.Set("method", parts[3])
				}
				d.Set("endpoint_id", endpointID)
				d.Set("deployment_type", deploymentType)
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
			},
			"method": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Creation method: 'string', 'file', 'repository', or 'url'",
				ForceNew:    true,
			},
			"name":        {Type: schema.TypeString, Required: true, ForceNew: true},
			"endpoint_id": {Type: schema.TypeInt, Required: true, ForceNew: true},
			"swarm_id":    {Type: schema.TypeString, Optional: true, ForceNew: true, Computed: true},
			"namespace":   {Type: schema.TypeString, Optional: true, ForceNew: true},
			"stack_file_content": {
				Type:     schema.TypeString,
				Optional: true,
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
			"stack_file_path": {Type: schema.TypeString, Optional: true},
			"additional_files": {
				Type:        schema.TypeList,
				Optional:    true,
				ForceNew:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "List of additional Compose file paths to use when deploying from Git repository.",
			},
			"git_repository_authentication": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
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
			},
			"repository_username": {
				Type:          schema.TypeString,
				Optional:      true,
				ConflictsWith: []string{"repository_username_wo"},
			},
			"repository_password": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				ConflictsWith: []string{"repository_password_wo"},
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
				Type:     schema.TypeString,
				Optional: true,
				Default:  "refs/heads/main",
			},
			"file_path_in_repository": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "docker-compose.yml",
			},
			"manifest_url":          {Type: schema.TypeString, Optional: true, ForceNew: true},
			"compose_format":        {Type: schema.TypeBool, Optional: true, Default: false, ForceNew: true},
			"support_relative_path": {Type: schema.TypeBool, Optional: true, Default: false, ForceNew: true},
			"filesystem_path":       {Type: schema.TypeString, Optional: true},
			"env": {
				Type:     schema.TypeList,
				Optional: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name":  {Type: schema.TypeString, Required: true},
						"value": {Type: schema.TypeString, Required: true},
					},
				},
			},
			"tlsskip_verify": {Type: schema.TypeBool, Optional: true, Computed: true, ForceNew: true},
			"prune": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to prune unused services/networks during stack update (default: false)",
			},
			"repository_git_credential_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "ID of the Git credentials to use for authentication.",
			},
			"resource_control_id": {
				Type:     schema.TypeInt,
				Computed: true,
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

func findExistingStackByName(client *APIClient, name string, endpointID int) (int, error) {
	url := fmt.Sprintf("%s/stacks", client.Endpoint)
	req, _ := http.NewRequest("GET", url, nil)
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

func resourcePortainerStackCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	deployment := d.Get("deployment_type").(string)
	method := d.Get("method").(string)
	name := d.Get("name").(string)
	endpointID := d.Get("endpoint_id").(int)

	if deployment == "swarm" && d.Get("swarm_id") == "" {
		swarmID, err := fetchSwarmID(client, endpointID)
		if err != nil {
			return fmt.Errorf("failed to fetch swarm_id: %w", err)
		}
		_ = d.Set("swarm_id", swarmID)
	}

	if existingID, err := findExistingStackByName(client, name, endpointID); err != nil {
		return fmt.Errorf("error checking for existing stack: %w", err)
	} else if existingID != 0 {
		d.SetId(strconv.Itoa(existingID))
		return resourcePortainerStackUpdate(d, meta)
	}

	var err error

	switch deployment {
	case "standalone":
		switch method {
		case "string":
			err = createStackStandaloneString(d, client)
		case "file":
			path := d.Get("stack_file_path").(string)
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("failed to read stack file from path: %w", readErr)
			}
			_ = d.Set("stack_file_content", string(content))
			err = createStackStandaloneString(d, client)
		case "repository":
			err = createStackStandaloneRepo(d, client)
		default:
			return fmt.Errorf("invalid method %q for standalone deployment", method)
		}

	case "swarm":
		switch method {
		case "string":
			err = createStackSwarmString(d, client)
		case "file":
			path := d.Get("stack_file_path").(string)
			content, readErr := os.ReadFile(path)
			if readErr != nil {
				return fmt.Errorf("failed to read stack file from path: %w", readErr)
			}
			_ = d.Set("stack_file_content", string(content))
			err = createStackSwarmString(d, client)
		case "repository":
			err = createStackSwarmRepo(d, client)
		default:
			return fmt.Errorf("invalid method %q for swarm deployment", method)
		}

	case "kubernetes":
		switch method {
		case "string":
			err = createStackK8sString(d, client)
		case "repository":
			err = createStackK8sRepo(d, client)
		case "url":
			err = createStackK8sURL(d, client)
		default:
			return fmt.Errorf("invalid method %q for kubernetes deployment", method)
		}

	default:
		return fmt.Errorf("invalid deployment_type %q", deployment)
	}

	if err != nil {
		return err
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
			return fmt.Errorf("failed to marshal stack update (create) payload: %w", err)
		}

		url := fmt.Sprintf("%s/stacks/%s?endpointId=%d", client.Endpoint, d.Id(), endpointID)
		req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonBody))
		if err != nil {
			return fmt.Errorf("failed to build stack update (create) request: %w", err)
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
			return fmt.Errorf("failed to perform stack update (create) request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			data, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to finalize stack creation (prune/webhook), status %d: %s", resp.StatusCode, string(data))
		}

		if webhookToken != "" {
			_ = d.Set("webhook_id", webhookToken)
			baseURL := strings.TrimSuffix(client.Endpoint, "/api")
			webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookToken)
			_ = d.Set("webhook_url", webhookURL)
		}
	}

	// ACCESS CONTROL UPDATE
	if err := updateStackAccessControl(d, client, d.Id()); err != nil {
		return fmt.Errorf("failed to update stack access control: %w", err)
	}

	return resourcePortainerStackRead(d, meta)
}

func resourcePortainerStackRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	stackID := d.Id()

	url := fmt.Sprintf("%s/stacks/%s", client.Endpoint, stackID)
	req, _ := http.NewRequest("GET", url, nil)
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch stack: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read stack, status: %d, body: %s", resp.StatusCode, string(body))
	}

	var stack struct {
		Name                string `json:"Name"`
		Type                int    `json:"Type"`
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
			TLSSkipVerify  bool `json:"tlsskipVerify"`
			Authentication struct {
				GitCredentialID int `json:"GitCredentialID"`
			} `json:"Authentication"`
		} `json:"gitConfig,omitempty"`

		Portainer struct {
			ResourceControl struct {
				Id int `json:"Id"`
			} `json:"ResourceControl"`
		} `json:"Portainer"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&stack); err != nil {
		return fmt.Errorf("failed to decode stack response: %w", err)
	}

	d.Set("name", stack.Name)
	d.Set("swarm_id", stack.SwarmID)
	d.Set("namespace", stack.Namespace)
	d.Set("compose_format", stack.ComposeFmt)

	var webhookToken string
	if stack.AutoUpdate != nil && stack.AutoUpdate.Webhook != "" {
		webhookToken = stack.AutoUpdate.Webhook
	} else if stack.Webhook != "" {
		webhookToken = stack.Webhook
	}

	if webhookToken != "" {
		_ = d.Set("stack_webhook", true)
		_ = d.Set("webhook_id", webhookToken)

		baseURL := strings.TrimSuffix(client.Endpoint, "/api")
		webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookToken)
		_ = d.Set("webhook_url", webhookURL)
	} else {
		_ = d.Set("stack_webhook", false)
		_ = d.Set("webhook_id", "")
		_ = d.Set("webhook_url", "")
	}

	method := d.Get("method").(string)
	if method != "repository" {
		fileURL := fmt.Sprintf("%s/stacks/%s/file", client.Endpoint, stackID)
		fileReq, _ := http.NewRequest("GET", fileURL, nil)
		if client.APIKey != "" {
			fileReq.Header.Set("X-API-Key", client.APIKey)
		} else if client.JWTToken != "" {
			fileReq.Header.Set("Authorization", "Bearer "+client.JWTToken)
		} else {
			return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
		}

		fileResp, err := client.HTTPClient.Do(fileReq)
		if err != nil {
			return fmt.Errorf("failed to fetch stack file: %w", err)
		}
		defer fileResp.Body.Close()

		if fileResp.StatusCode >= 400 {
			body, _ := io.ReadAll(fileResp.Body)
			return fmt.Errorf("failed to fetch stack file, status: %d, body: %s", fileResp.StatusCode, string(body))
		}

		var fileContent struct {
			StackFileContent string `json:"StackFileContent"`
		}
		if err := json.NewDecoder(fileResp.Body).Decode(&fileContent); err != nil {
			return fmt.Errorf("failed to decode stack file content: %w", err)
		}

		d.Set("stack_file_content", fileContent.StackFileContent)
	}

	// Env → Terraform
	var tfEnvs []map[string]interface{}
	for _, env := range stack.Env {
		tfEnvs = append(tfEnvs, map[string]interface{}{
			"name":  env.Name,
			"value": env.Value,
		})
	}
	d.Set("env", tfEnvs)
	_ = d.Set("method", method)
	_ = d.Set("endpoint_id", stack.EndpointID)
	_ = d.Set("support_relative_path", stack.SupportRelativePath)
	if method == "repository" && stack.GitConfig != nil {
		_ = d.Set("tlsskip_verify", stack.GitConfig.TLSSkipVerify)
		_ = d.Set("repository_git_credential_id", stack.GitConfig.Authentication.GitCredentialID)
	}
	if stack.AutoUpdate != nil {
		_ = d.Set("pull_image", stack.AutoUpdate.ForcePullImage)
		_ = d.Set("update_interval", stack.AutoUpdate.Interval)
	}

	if stack.Portainer.ResourceControl.Id != 0 {
		_ = d.Set("resource_control_id", stack.Portainer.ResourceControl.Id)

		// Read Access Control
		rcID := strconv.Itoa(stack.Portainer.ResourceControl.Id)
		if err := readStackAccessControl(d, client, rcID); err != nil {
			return fmt.Errorf("failed to read stack access control: %w", err)
		}
	}

	return nil
}

func fetchSwarmID(client *APIClient, endpointID int) (string, error) {
	url := fmt.Sprintf("%s/endpoints/%d/docker/swarm", client.Endpoint, endpointID)
	req, _ := http.NewRequest("GET", url, nil)
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return "", fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
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

func resourcePortainerStackDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Id()
	endpointID := d.Get("endpoint_id").(int)

	url := fmt.Sprintf("%s/stacks/%s?endpointId=%d", client.Endpoint, id, endpointID)
	req, err := http.NewRequest("DELETE", url, nil)
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

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 204 || resp.StatusCode == 404 {
		return nil
	}

	data, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("failed to delete stack: %s", string(data))
}

func resourcePortainerStackUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	stackID := d.Id()
	endpointID := d.Get("endpoint_id").(int)
	method := d.Get("method").(string)

	if method == "file" {
		path := d.Get("stack_file_path").(string)
		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("failed to read stack file for update: %w", err)
		}
		_ = d.Set("stack_file_content", string(content))
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
		_ = d.Set("webhook_id", webhookID)
		if webhookID != "" {
			baseURL := strings.TrimSuffix(client.Endpoint, "/api")
			webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookID)
			_ = d.Set("webhook_url", webhookURL)
		}

		jsonBody, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal git update payload: %w", err)
		}

		url := fmt.Sprintf("%s/stacks/%s/git?endpointId=%d", client.Endpoint, stackID, endpointID)
		req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
		if err != nil {
			return fmt.Errorf("failed to build git update request: %w", err)
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
			return fmt.Errorf("failed to perform git update request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			body, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to update git stack settings: %s", string(body))
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
			"stackName":                 d.Get("name").(string),
			"additionalFiles":           expandStringList(d.Get("additional_files").([]interface{})),
			"registries":                expandIntList(d.Get("registries").([]interface{})),
		}

		redeployBody, err := json.Marshal(redeployPayload)
		if err != nil {
			return fmt.Errorf("failed to marshal git redeploy payload: %w", err)
		}

		redeployURL := fmt.Sprintf("%s/stacks/%s/git/redeploy?endpointId=%d", client.Endpoint, stackID, endpointID)
		reqRedeploy, err := http.NewRequest("PUT", redeployURL, bytes.NewBuffer(redeployBody))
		if err != nil {
			return fmt.Errorf("failed to build git redeploy request: %w", err)
		}
		if client.APIKey != "" {
			reqRedeploy.Header.Set("X-API-Key", client.APIKey)
		} else if client.JWTToken != "" {
			reqRedeploy.Header.Set("Authorization", "Bearer "+client.JWTToken)
		} else {
			return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
		}
		reqRedeploy.Header.Set("Content-Type", "application/json")

		respRedeploy, err := client.HTTPClient.Do(reqRedeploy)
		if err != nil {
			return fmt.Errorf("failed to perform git redeploy request: %w", err)
		}
		defer respRedeploy.Body.Close()

		if respRedeploy.StatusCode != 200 {
			data, _ := io.ReadAll(respRedeploy.Body)
			return fmt.Errorf("failed to redeploy git stack: %s", string(data))
		}

		return resourcePortainerStackRead(d, meta)
	}

	if err := updateStackAccessControl(d, client, stackID); err != nil {
		return fmt.Errorf("failed to update stack access control: %w", err)
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
			return fmt.Errorf("failed to marshal standard update payload: %w", err)
		}

		url := fmt.Sprintf("%s/stacks/%s?endpointId=%d", client.Endpoint, stackID, endpointID)
		req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonBody))
		if err != nil {
			return fmt.Errorf("failed to build standard update request: %w", err)
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
			return fmt.Errorf("failed to perform standard update request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			data, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to update stack: %s", string(data))
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
			return fmt.Errorf("failed to marshal webhook update payload: %w", err)
		}

		url := fmt.Sprintf("%s/stacks/%s?endpointId=%d", client.Endpoint, d.Id(), endpointID)
		req, err := http.NewRequest("PUT", url, bytes.NewBuffer(jsonBody))
		if err != nil {
			return fmt.Errorf("failed to build webhook update request: %w", err)
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
			return fmt.Errorf("failed to perform webhook update request: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			data, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to update stack webhook, status %d: %s", resp.StatusCode, string(data))
		}

		_ = d.Set("webhook_id", webhookToken)
		baseURL := strings.TrimSuffix(client.Endpoint, "/api")
		webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookToken)
		_ = d.Set("webhook_url", webhookURL)
	}

	return resourcePortainerStackRead(d, meta)
}

func flattenEnvList(envList []interface{}) []map[string]string {
	var out []map[string]string
	for _, v := range envList {
		item := v.(map[string]interface{})
		out = append(out, map[string]string{
			"name":  item["name"].(string),
			"value": item["value"].(string),
		})
	}
	return out
}

func firstNonEmpty(values ...interface{}) string {
	for _, v := range values {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return s
		}
	}
	return ""
}

func mustJSON(data interface{}) []byte {
	out, _ := json.Marshal(data)
	return out
}

// --------------------- STANDALONE ----------------------

func createStackStandaloneString(d *schema.ResourceData, client *APIClient) error {
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

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
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

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create standalone stack: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	d.SetId(strconv.Itoa(result.ID))
	return nil
}

func createStackStandaloneRepo(d *schema.ResourceData, client *APIClient) error {
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

	payload := map[string]interface{}{
		"name":                      d.Get("name").(string),
		"composeFile":               d.Get("file_path_in_repository").(string),
		"repositoryURL":             repoURL,
		"repositoryUsername":        repoUser,
		"repositoryPassword":        repoPass,
		"repositoryReferenceName":   d.Get("repository_reference_name").(string),
		"repositoryAuthentication":  d.Get("git_repository_authentication").(bool),
		"repositoryGitCredentialID": d.Get("repository_git_credential_id").(int),
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
			d.Set("webhook_id", webhookID)
			baseURL := strings.TrimSuffix(client.Endpoint, "/api")
			webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookID)
			d.Set("webhook_url", webhookURL)
		}
	}

	payload["registries"] = expandIntList(d.Get("registries").([]interface{}))
	endpointID := d.Get("endpoint_id").(int)
	url := fmt.Sprintf("%s/stacks/create/standalone/repository?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
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

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create standalone stack from repository: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	d.SetId(strconv.Itoa(result.ID))
	return nil
}

// --------------------- SWARM ----------------------

func createStackSwarmString(d *schema.ResourceData, client *APIClient) error {
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

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
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

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create swarm stack: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	d.SetId(strconv.Itoa(result.ID))
	return nil
}

func createStackSwarmRepo(d *schema.ResourceData, client *APIClient) error {
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
	payload := map[string]interface{}{
		"name":                      d.Get("name").(string),
		"composeFile":               d.Get("file_path_in_repository").(string),
		"repositoryURL":             repoURL,
		"repositoryUsername":        repoUser,
		"repositoryPassword":        repoPass,
		"repositoryReferenceName":   d.Get("repository_reference_name").(string),
		"repositoryAuthentication":  d.Get("git_repository_authentication").(bool),
		"repositoryGitCredentialID": d.Get("repository_git_credential_id").(int),
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
			d.Set("webhook_id", webhookID)
			baseURL := strings.TrimSuffix(client.Endpoint, "/api")
			webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookID)
			d.Set("webhook_url", webhookURL)
		}
	}

	payload["registries"] = expandIntList(d.Get("registries").([]interface{}))
	endpointID := d.Get("endpoint_id").(int)
	url := fmt.Sprintf("%s/stacks/create/swarm/repository?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
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

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create swarm stack from repository: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	d.SetId(strconv.Itoa(result.ID))

	if d.Get("prune").(bool) {
		fmt.Println("[INFO] Performing immediate redeploy with prune=true after stack creation")
		if err := resourcePortainerStackUpdate(d, client); err != nil {
			fmt.Printf("[WARN] prune redeploy failed: %v\n", err)
		} else {
			fmt.Println("[INFO] prune redeploy succeeded")
		}
	}

	return nil
}

// --------------------- KUBERNETES ----------------------

func createStackK8sString(d *schema.ResourceData, client *APIClient) error {
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

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
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

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create kubernetes stack from string: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	d.SetId(strconv.Itoa(result.ID))
	return nil
}

func createStackK8sRepo(d *schema.ResourceData, client *APIClient) error {
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
	payload := map[string]interface{}{
		"stackName":                 d.Get("name").(string),
		"manifestFile":              d.Get("file_path_in_repository").(string),
		"namespace":                 d.Get("namespace").(string),
		"composeFormat":             d.Get("compose_format").(bool),
		"repositoryURL":             repoURL,
		"repositoryUsername":        repoUser,
		"repositoryPassword":        repoPass,
		"repositoryReferenceName":   d.Get("repository_reference_name").(string),
		"repositoryAuthentication":  d.Get("git_repository_authentication").(bool),
		"repositoryGitCredentialID": d.Get("repository_git_credential_id").(int),
		"tlsskipVerify":             d.Get("tlsskip_verify").(bool),
		"fromAppTemplate":           false,
		"additionalFiles":           expandStringList(d.Get("additional_files").([]interface{})),
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
			d.Set("webhook_id", webhookID)
			baseURL := strings.TrimSuffix(client.Endpoint, "/api")
			webhookURL := fmt.Sprintf("%s/api/stacks/webhooks/%s", baseURL, webhookID)
			d.Set("webhook_url", webhookURL)
		}
	}

	payload["registries"] = expandIntList(d.Get("registries").([]interface{}))
	endpointID := d.Get("endpoint_id").(int)
	url := fmt.Sprintf("%s/stacks/create/kubernetes/repository?endpointId=%d", client.Endpoint, endpointID)
	jsonBody, _ := json.Marshal(payload)

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
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

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create kubernetes stack from repository: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
	d.SetId(strconv.Itoa(result.ID))
	return nil
}

func createStackK8sURL(d *schema.ResourceData, client *APIClient) error {
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

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(jsonBody))
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

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create kubernetes stack from URL: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	json.NewDecoder(resp.Body).Decode(&result)
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

	resp, err := client.DoRequest("PUT", fmt.Sprintf("/resource_controls/%s", rcID), nil, payload)
	if err != nil {
		return fmt.Errorf("failed to update resource control: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update resource control %s: %s", rcID, string(data))
	}

	return nil
}

func readStackAccessControl(d *schema.ResourceData, client *APIClient, rcID string) error {
	resp, err := client.DoRequest("GET", fmt.Sprintf("/resource_controls/%s", rcID), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to fetch resource control %s", rcID)
	}

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

	if err := json.NewDecoder(resp.Body).Decode(&rc); err != nil {
		return err
	}

	if rc.Public {
		d.Set("ownership", "public")
	} else if rc.AdministratorsOnly {
		d.Set("ownership", "administrators")
	} else {
		d.Set("ownership", "restricted")
	}

	users := []int{}
	for _, u := range rc.UserAccesses {
		users = append(users, u.UserID)
	}
	d.Set("authorized_users", users)

	teams := []int{}
	for _, t := range rc.TeamAccesses {
		teams = append(teams, t.TeamID)
	}
	d.Set("authorized_teams", teams)

	return nil
}

func expandIntSet(set *schema.Set) []int {
	result := []int{}
	for _, v := range set.List() {
		result = append(result, v.(int))
	}
	return result
}
