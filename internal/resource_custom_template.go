package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
	"github.com/portainer/client-api-go/v2/pkg/client/custom_templates"
	"github.com/portainer/client-api-go/v2/pkg/models"
)

func resourceCustomTemplate() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceCustomTemplateCreate,
		ReadContext:   resourceCustomTemplateRead,
		DeleteContext: resourceCustomTemplateDelete,
		UpdateContext: resourceCustomTemplateUpdate,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"title":                          {Type: schema.TypeString, Required: true, Description: "Title of the Portainer custom template."},
			"description":                    {Type: schema.TypeString, Required: true, Description: "Short description of the custom template displayed in the Portainer UI."},
			"note":                           {Type: schema.TypeString, Required: true, Description: "Additional note or instructions associated with the custom template."},
			"platform":                       {Type: schema.TypeInt, Required: true, ValidateFunc: validation.IntBetween(1, 2), Description: "Target platform for the template: 1 = Linux, 2 = Windows."},
			"type":                           {Type: schema.TypeInt, Required: true, ValidateFunc: validation.IntBetween(1, 3), Description: "Template type: 1 = Swarm stack, 2 = Compose stack, 3 = Kubernetes manifest."},
			"logo":                           {Type: schema.TypeString, Optional: true, Description: "URL of the logo image displayed next to the template in the Portainer UI."},
			"edge_template":                  {Type: schema.TypeBool, Optional: true, Default: false, Description: "Whether this template is exposed as an Edge template."},
			"is_compose_format":              {Type: schema.TypeBool, Optional: true, Default: false, Description: "Whether the Kubernetes manifest is provided in Compose format (only relevant for `type = 3`)."},
			"variables":                      {Type: schema.TypeList, Optional: true, Elem: &schema.Schema{Type: schema.TypeMap}, Description: "List of template variable definitions (name, label, description, defaultValue) used for parameterized templates."},
			"file_content":                   {Type: schema.TypeString, Optional: true, Description: "Inline template body (Compose YAML, Kubernetes manifest, or Swarm stack file). Mutually exclusive with `file_path` and the repository fields."},
			"file_path":                      {Type: schema.TypeString, Optional: true, ForceNew: true, Description: "Path on the local filesystem to a file containing the template body. Mutually exclusive with `file_content` and the repository fields."},
			"repository_url":                 {Type: schema.TypeString, Optional: true, ForceNew: true, Description: "URL of the Git repository hosting the template manifest."},
			"repository_username":            {Type: schema.TypeString, Optional: true, ForceNew: true, Description: "Username used to authenticate against the Git repository."},
			"repository_password":            {Type: schema.TypeString, Optional: true, Sensitive: true, ForceNew: true, ConflictsWith: []string{"repository_password_wo"}, Description: "Sensitive password or personal access token used to authenticate against the Git repository. Stored in state; use `repository_password_wo` to keep it out."},
			"repository_password_wo":         writeOnlySecretSchema("repository_password", "repository_password_wo_version", "Git password or personal access token"),
			"repository_password_wo_version": writeOnlyVersionSchema("repository_password_wo", true),
			"repository_reference":           {Type: schema.TypeString, Optional: true, Default: "refs/heads/main", ForceNew: true, Description: "Git reference (branch, tag, or commit) to check out (defaults to `refs/heads/main`)."},
			"compose_file_path":              {Type: schema.TypeString, Optional: true, Default: "docker-compose.yml", ForceNew: true, Description: "Path within the Git repository to the Compose or manifest file (defaults to `docker-compose.yml`)."},
			"source_id":                      {Type: schema.TypeInt, Optional: true, ForceNew: true, Description: "ID of a Portainer Source (Git source) providing the repository URL and credentials. Portainer 2.43 and later. When set, `repository_url` and the repository credentials can be left out; `repository_reference` and `compose_file_path` still apply."},
			"tlsskip_verify":                 {Type: schema.TypeBool, Optional: true, Default: false, ForceNew: true, Description: "Whether to skip TLS verification when cloning the Git repository."},
			"repository_authentication": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				ForceNew:    true,
				Description: "Enable authentication for the Git repository (default: false).",
			},
		},
	}
}

func findExistingCustomTemplateByTitle(ctx context.Context, client *APIClient, title string) (int, error) {
	ctx, errBody := withErrorCapture(ctx)
	params := custom_templates.NewCustomTemplateListParams()
	params.SetContext(ctx)
	resp, err := client.Client.CustomTemplates.CustomTemplateList(params, client.AuthInfo)
	if err != nil {
		return 0, fmt.Errorf("failed to list custom templates: %w", decorateSDKError(err, errBody))
	}

	for _, tmpl := range resp.Payload {
		if tmpl.Title == title {
			return int(tmpl.ID), nil
		}
	}

	return 0, nil
}

func resourceCustomTemplateCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	title := d.Get("title").(string)

	existingID, err := findExistingCustomTemplateByTitle(ctx, client, title)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to check for existing custom template: %w", err))
	} else if existingID != 0 {
		d.SetId(strconv.Itoa(existingID))
		return resourceCustomTemplateUpdate(ctx, d, meta)
	}

	if v, ok := d.GetOk("file_content"); ok {
		return diag.FromErr(createTemplateFromString(ctx, d, client, v.(string)))
	}

	if v, ok := d.GetOk("file_path"); ok {
		content, err := os.ReadFile(v.(string))
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to read template file from path: %w", err))
		}
		if err := d.Set("file_content", string(content)); err != nil {
			return diag.FromErr(err)
		}
		return diag.FromErr(createTemplateFromString(ctx, d, client, string(content)))
	}

	repoURL := d.Get("repository_url").(string)
	if repoURL != "" || d.Get("source_id").(int) != 0 {
		return diag.FromErr(createTemplateFromRepository(ctx, d, client, repoURL))
	}

	return diag.FromErr(fmt.Errorf("one of file_content, file_path, repository_url, or source_id must be provided"))
}

func createTemplateFromString(ctx context.Context, d *schema.ResourceData, client *APIClient, content string) error {
	title := d.Get("title").(string)
	description := d.Get("description").(string)
	templateType := int64(d.Get("type").(int))

	ctx, errBody := withErrorCapture(ctx)
	params := custom_templates.NewCustomTemplateCreateStringParams()
	params.SetContext(ctx)
	params.Body = &models.CustomtemplatesCustomTemplateFromFileContentPayload{
		Title:        &title,
		Description:  &description,
		Note:         d.Get("note").(string),
		Platform:     int64(d.Get("platform").(int)),
		Type:         &templateType,
		Logo:         d.Get("logo").(string),
		EdgeTemplate: d.Get("edge_template").(bool),
		FileContent:  &content,
		Variables:    getVariablesSDK(d),
	}

	resp, err := client.Client.CustomTemplates.CustomTemplateCreateString(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to create custom template from string: %w", decorateSDKError(err, errBody))
	}

	d.SetId(strconv.FormatInt(resp.Payload.ID, 10))
	return nil
}

func createTemplateFromRepository(ctx context.Context, d *schema.ResourceData, client *APIClient, repoURL string) error {
	title := d.Get("title").(string)
	description := d.Get("description").(string)
	templateType := int64(d.Get("type").(int))
	useAuth := d.Get("repository_authentication").(bool)

	ctx, errBody := withErrorCapture(ctx)
	params := custom_templates.NewCustomTemplateCreateRepositoryParams()
	params.SetContext(ctx)
	composePath := d.Get("compose_file_path").(string)
	params.Body = &models.CustomtemplatesCustomTemplateFromGitRepositoryPayload{
		Title:                       &title,
		Description:                 &description,
		Note:                        d.Get("note").(string),
		Platform:                    int64(d.Get("platform").(int)),
		Type:                        &templateType,
		Logo:                        d.Get("logo").(string),
		EdgeTemplate:                d.Get("edge_template").(bool),
		IsComposeFormat:             d.Get("is_compose_format").(bool),
		RepositoryURL:               &repoURL,
		RepositoryAuthentication:    useAuth,
		RepositoryReferenceName:     d.Get("repository_reference").(string),
		ComposeFilePathInRepository: &composePath,
		TlsskipVerify:               d.Get("tlsskip_verify").(bool),
		Variables:                   getVariablesSDK(d),
	}

	if useAuth {
		params.Body.RepositoryUsername = d.Get("repository_username").(string)
		password, err := secretValue(d, "repository_password", "repository_password_wo", "repository_password_wo_version")
		if err != nil {
			return err
		}
		params.Body.RepositoryPassword = password
	}

	// The SDK payload predates Sources and has no SourceID field, so a
	// template built from a Source goes out as the same body plus SourceID.
	if sourceID := d.Get("source_id").(int); sourceID != 0 {
		return createTemplateFromSource(ctx, client, d, params.Body, sourceID)
	}

	resp, err := client.Client.CustomTemplates.CustomTemplateCreateRepository(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to create custom template from repository: %w", decorateSDKError(err, errBody))
	}

	d.SetId(strconv.FormatInt(resp.Payload.ID, 10))
	return nil
}

// withSourceID returns an SDK payload as a JSON object with SourceID added,
// for the template endpoints whose SDK models predate Sources. An empty
// repository URL is dropped, since the Source provides it.
func withSourceID(payload interface{}, sourceID int) (map[string]interface{}, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode custom template payload: %w", err)
	}
	var body map[string]interface{}
	if err := json.Unmarshal(raw, &body); err != nil {
		return nil, fmt.Errorf("failed to encode custom template payload: %w", err)
	}
	body["SourceID"] = sourceID
	if url, ok := body["repositoryURL"]; ok && (url == nil || url == "") {
		delete(body, "repositoryURL")
	}
	return body, nil
}

func createTemplateFromSource(ctx context.Context, client *APIClient, d *schema.ResourceData, payload *models.CustomtemplatesCustomTemplateFromGitRepositoryPayload, sourceID int) error {
	body, err := withSourceID(payload, sourceID)
	if err != nil {
		return err
	}

	var created struct {
		ID int `json:"Id"`
	}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/custom_templates/create/repository", body, &created); err != nil {
		return fmt.Errorf("failed to create custom template from source %d: %w", sourceID, err)
	}
	d.SetId(strconv.Itoa(created.ID))
	return nil
}

func getVariablesSDK(d *schema.ResourceData) []*models.PortainerCustomTemplateVariableDefinition {
	if v, ok := d.GetOk("variables"); ok {
		vars := v.([]interface{})
		result := make([]*models.PortainerCustomTemplateVariableDefinition, 0, len(vars))
		for _, varItem := range vars {
			if varMap, ok := varItem.(map[string]interface{}); ok {
				varDef := &models.PortainerCustomTemplateVariableDefinition{}
				if name, exists := varMap["name"]; exists {
					varDef.Name = name.(string)
				}
				if label, exists := varMap["label"]; exists {
					varDef.Label = label.(string)
				}
				if defaultValue, exists := varMap["default_value"]; exists {
					varDef.DefaultValue = defaultValue.(string)
				}
				if desc, exists := varMap["description"]; exists {
					varDef.Description = desc.(string)
				}
				result = append(result, varDef)
			}
		}
		return result
	}
	return nil
}

func resourceCustomTemplateRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	ctx, errBody := withErrorCapture(ctx)
	params := custom_templates.NewCustomTemplateInspectParams()
	params.SetContext(ctx)
	params.ID = id

	resp, err := client.Client.CustomTemplates.CustomTemplateInspect(params, client.AuthInfo)
	if err != nil {
		var notFound *custom_templates.CustomTemplateInspectNotFound
		if errors.As(err, &notFound) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read custom template: %w", decorateSDKError(err, errBody)))
	}

	if err := d.Set("title", resp.Payload.Title); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("description", resp.Payload.Description); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("note", resp.Payload.Note); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("platform", int(resp.Payload.Platform)); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("type", int(resp.Payload.Type)); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("logo", resp.Payload.Logo); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("edge_template", resp.Payload.EdgeTemplate); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("is_compose_format", resp.Payload.IsComposeFormat); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceCustomTemplateUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	title := d.Get("title").(string)
	description := d.Get("description").(string)
	templateType := int64(d.Get("type").(int))

	var fileContent string
	if v, ok := d.GetOk("file_path"); ok {
		content, err := os.ReadFile(v.(string))
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to read template file from path: %w", err))
		}
		fileContent = string(content)
		if err := d.Set("file_content", fileContent); err != nil {
			return diag.FromErr(err)
		}
	} else if v, ok := d.GetOk("file_content"); ok {
		fileContent = v.(string)
	}

	composePath := d.Get("compose_file_path").(string)
	useAuth := d.Get("repository_authentication").(bool)

	ctx, errBody := withErrorCapture(ctx)
	params := custom_templates.NewCustomTemplateUpdateParams()
	params.SetContext(ctx)
	params.ID = id
	params.Body = &models.CustomtemplatesCustomTemplateUpdatePayload{
		Title:                       &title,
		Description:                 &description,
		Note:                        d.Get("note").(string),
		Platform:                    int64(d.Get("platform").(int)),
		Type:                        &templateType,
		Logo:                        d.Get("logo").(string),
		EdgeTemplate:                d.Get("edge_template").(bool),
		IsComposeFormat:             d.Get("is_compose_format").(bool),
		ComposeFilePathInRepository: &composePath,
		TlsskipVerify:               d.Get("tlsskip_verify").(bool),
		FileContent:                 &fileContent,
		Variables:                   getVariablesSDK(d),
	}

	isGitBased := false
	if repoURL := d.Get("repository_url").(string); repoURL != "" || d.Get("source_id").(int) != 0 {
		isGitBased = true
		params.Body.RepositoryURL = &repoURL
		params.Body.RepositoryReferenceName = d.Get("repository_reference").(string)
		params.Body.RepositoryAuthentication = useAuth
		if useAuth {
			params.Body.RepositoryUsername = d.Get("repository_username").(string)
			password, err := secretValue(d, "repository_password", "repository_password_wo", "repository_password_wo_version")
			if err != nil {
				return diag.FromErr(err)
			}
			params.Body.RepositoryPassword = password
		}
	}

	if sourceID := d.Get("source_id").(int); sourceID != 0 {
		// A Source-based template has to name its Source on every update, or
		// Portainer finds neither content nor a repository and rejects it.
		body, err := withSourceID(params.Body, sourceID)
		if err != nil {
			return diag.FromErr(err)
		}
		if err := doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/custom_templates/%d", client.Endpoint, id), body, nil); err != nil {
			return diag.FromErr(fmt.Errorf("failed to update custom template: %w", err))
		}
	} else if _, err := client.Client.CustomTemplates.CustomTemplateUpdate(params, client.AuthInfo); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update custom template: %w", decorateSDKError(err, errBody)))
	}

	if isGitBased {
		gitCtx, gitErrBody := withErrorCapture(ctx)
		gitParams := custom_templates.NewCustomTemplateGitFetchParams()
		gitParams.SetContext(gitCtx)
		gitParams.ID = id
		_, err := client.Client.CustomTemplates.CustomTemplateGitFetch(gitParams, client.AuthInfo)
		if err != nil {
			return diag.FromErr(fmt.Errorf("failed to git_fetch template: %w", decorateSDKError(err, gitErrBody)))
		}
	}

	return resourceCustomTemplateRead(ctx, d, meta)
}

func resourceCustomTemplateDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	ctx, errBody := withErrorCapture(ctx)
	params := custom_templates.NewCustomTemplateDeleteParams()
	params.SetContext(ctx)
	params.ID = id

	_, err := client.Client.CustomTemplates.CustomTemplateDelete(params, client.AuthInfo)
	if err != nil {
		var notFound *custom_templates.CustomTemplateDeleteNotFound
		if errors.As(err, &notFound) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete custom template: %w", decorateSDKError(err, errBody)))
	}
	return nil
}
