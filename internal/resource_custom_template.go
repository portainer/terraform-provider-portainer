package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
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
	templates, err := listCustomTemplates(ctx, client)
	if err != nil {
		return 0, err
	}

	for _, tmpl := range templates {
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

	body := customTemplateFromStringPayload{
		Title:        title,
		Description:  description,
		Note:         d.Get("note").(string),
		Platform:     int64(d.Get("platform").(int)),
		Type:         templateType,
		Logo:         d.Get("logo").(string),
		EdgeTemplate: d.Get("edge_template").(bool),
		FileContent:  content,
		Variables:    customTemplateVariables(d),
	}

	var created portainerCustomTemplate
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/custom_templates/create/string", body, &created); err != nil {
		return fmt.Errorf("failed to create custom template from string: %w", err)
	}

	d.SetId(strconv.FormatInt(created.ID, 10))
	return nil
}

func createTemplateFromRepository(ctx context.Context, d *schema.ResourceData, client *APIClient, repoURL string) error {
	title := d.Get("title").(string)
	description := d.Get("description").(string)
	templateType := int64(d.Get("type").(int))
	useAuth := d.Get("repository_authentication").(bool)

	composePath := d.Get("compose_file_path").(string)
	body := &customTemplateFromRepositoryPayload{
		Title:                       title,
		Description:                 description,
		Note:                        d.Get("note").(string),
		Platform:                    int64(d.Get("platform").(int)),
		Type:                        templateType,
		Logo:                        d.Get("logo").(string),
		EdgeTemplate:                d.Get("edge_template").(bool),
		IsComposeFormat:             d.Get("is_compose_format").(bool),
		RepositoryURL:               &repoURL,
		RepositoryAuthentication:    useAuth,
		RepositoryReferenceName:     d.Get("repository_reference").(string),
		ComposeFilePathInRepository: &composePath,
		TLSSkipVerify:               d.Get("tlsskip_verify").(bool),
		Variables:                   customTemplateVariables(d),
	}

	if useAuth {
		body.RepositoryUsername = d.Get("repository_username").(string)
		password, err := secretValue(d, "repository_password", "repository_password_wo", "repository_password_wo_version")
		if err != nil {
			return err
		}
		body.RepositoryPassword = password
	}

	// A template built from a Source goes out as the same body plus SourceID.
	if sourceID := d.Get("source_id").(int); sourceID != 0 {
		return createTemplateFromSource(ctx, client, d, body, sourceID)
	}

	var created portainerCustomTemplate
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/custom_templates/create/repository", body, &created); err != nil {
		return fmt.Errorf("failed to create custom template from repository: %w", err)
	}

	d.SetId(strconv.FormatInt(created.ID, 10))
	return nil
}

// withSourceID returns a template payload as a JSON object with SourceID
// added. An empty repository URL is dropped, since the Source provides it.
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

func createTemplateFromSource(ctx context.Context, client *APIClient, d *schema.ResourceData, payload *customTemplateFromRepositoryPayload, sourceID int) error {
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

func customTemplateVariables(d *schema.ResourceData) []*customTemplateVariable {
	if v, ok := d.GetOk("variables"); ok {
		vars := v.([]interface{})
		result := make([]*customTemplateVariable, 0, len(vars))
		for _, varItem := range vars {
			if varMap, ok := varItem.(map[string]interface{}); ok {
				varDef := &customTemplateVariable{}
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

	var tmpl portainerCustomTemplate
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/custom_templates/%d", client.Endpoint, id), nil, &tmpl); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read custom template: %w", err))
	}

	if err := d.Set("title", tmpl.Title); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("description", tmpl.Description); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("note", tmpl.Note); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("platform", int(tmpl.Platform)); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("type", int(tmpl.Type)); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("logo", tmpl.Logo); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("edge_template", tmpl.EdgeTemplate); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("is_compose_format", tmpl.IsComposeFormat); err != nil {
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

	payload := &customTemplateUpdatePayload{
		Title:                       title,
		Description:                 description,
		Note:                        d.Get("note").(string),
		Platform:                    int64(d.Get("platform").(int)),
		Type:                        templateType,
		Logo:                        d.Get("logo").(string),
		EdgeTemplate:                d.Get("edge_template").(bool),
		IsComposeFormat:             d.Get("is_compose_format").(bool),
		ComposeFilePathInRepository: &composePath,
		TLSSkipVerify:               d.Get("tlsskip_verify").(bool),
		FileContent:                 fileContent,
		Variables:                   customTemplateVariables(d),
	}

	isGitBased := false
	if repoURL := d.Get("repository_url").(string); repoURL != "" || d.Get("source_id").(int) != 0 {
		isGitBased = true
		payload.RepositoryURL = &repoURL
		payload.RepositoryReferenceName = d.Get("repository_reference").(string)
		payload.RepositoryAuthentication = useAuth
		if useAuth {
			payload.RepositoryUsername = d.Get("repository_username").(string)
			password, err := secretValue(d, "repository_password", "repository_password_wo", "repository_password_wo_version")
			if err != nil {
				return diag.FromErr(err)
			}
			payload.RepositoryPassword = password
		}
	}

	var body interface{} = payload
	if sourceID := d.Get("source_id").(int); sourceID != 0 {
		// A Source-based template has to name its Source on every update, or
		// Portainer finds neither content nor a repository and rejects it.
		withSource, err := withSourceID(payload, sourceID)
		if err != nil {
			return diag.FromErr(err)
		}
		body = withSource
	}
	if err := doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/custom_templates/%d", client.Endpoint, id), body, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update custom template: %w", err))
	}

	if isGitBased {
		if err := doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/custom_templates/%d/git_fetch", client.Endpoint, id), nil, nil); err != nil {
			return diag.FromErr(fmt.Errorf("failed to git_fetch template: %w", err))
		}
	}

	return resourceCustomTemplateRead(ctx, d, meta)
}

func resourceCustomTemplateDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	if err := doJSON(ctx, client, http.MethodDelete, fmt.Sprintf("%s/custom_templates/%d", client.Endpoint, id), nil, nil); err != nil {
		if isAPINotFound(err) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete custom template: %w", err))
	}
	return nil
}

type customTemplateVariable struct {
	Name         string `json:"name,omitempty"`
	Label        string `json:"label,omitempty"`
	DefaultValue string `json:"defaultValue,omitempty"`
	Description  string `json:"description,omitempty"`
}

// customTemplateFromStringPayload is the POST /custom_templates/create/string
// body.
type customTemplateFromStringPayload struct {
	Title        string                    `json:"title"`
	Description  string                    `json:"description"`
	Note         string                    `json:"note,omitempty"`
	Platform     int64                     `json:"platform,omitempty"`
	Type         int64                     `json:"type"`
	Logo         string                    `json:"logo,omitempty"`
	EdgeTemplate bool                      `json:"edgeTemplate,omitempty"`
	FileContent  string                    `json:"fileContent"`
	Variables    []*customTemplateVariable `json:"variables"`
}

// customTemplateFromRepositoryPayload is the
// POST /custom_templates/create/repository body.
type customTemplateFromRepositoryPayload struct {
	Title                       string                    `json:"title"`
	Description                 string                    `json:"description"`
	Note                        string                    `json:"note,omitempty"`
	Platform                    int64                     `json:"platform,omitempty"`
	Type                        int64                     `json:"type"`
	Logo                        string                    `json:"logo,omitempty"`
	EdgeTemplate                bool                      `json:"edgeTemplate,omitempty"`
	IsComposeFormat             bool                      `json:"isComposeFormat,omitempty"`
	RepositoryURL               *string                   `json:"repositoryURL"`
	RepositoryReferenceName     string                    `json:"repositoryReferenceName,omitempty"`
	RepositoryAuthentication    bool                      `json:"repositoryAuthentication,omitempty"`
	RepositoryUsername          string                    `json:"repositoryUsername,omitempty"`
	RepositoryPassword          string                    `json:"repositoryPassword,omitempty"`
	ComposeFilePathInRepository *string                   `json:"composeFilePathInRepository,omitempty"`
	TLSSkipVerify               bool                      `json:"tlsskipVerify,omitempty"`
	Variables                   []*customTemplateVariable `json:"variables"`
}

// customTemplateUpdatePayload is the PUT /custom_templates/{id} body.
type customTemplateUpdatePayload struct {
	Title                       string                    `json:"title"`
	Description                 string                    `json:"description"`
	Note                        string                    `json:"note,omitempty"`
	Platform                    int64                     `json:"platform,omitempty"`
	Type                        int64                     `json:"type"`
	Logo                        string                    `json:"logo,omitempty"`
	EdgeTemplate                bool                      `json:"edgeTemplate,omitempty"`
	IsComposeFormat             bool                      `json:"isComposeFormat,omitempty"`
	FileContent                 string                    `json:"fileContent"`
	RepositoryURL               *string                   `json:"repositoryURL"`
	RepositoryReferenceName     string                    `json:"repositoryReferenceName,omitempty"`
	RepositoryAuthentication    bool                      `json:"repositoryAuthentication,omitempty"`
	RepositoryUsername          string                    `json:"repositoryUsername,omitempty"`
	RepositoryPassword          string                    `json:"repositoryPassword,omitempty"`
	ComposeFilePathInRepository *string                   `json:"composeFilePathInRepository,omitempty"`
	TLSSkipVerify               bool                      `json:"tlsskipVerify,omitempty"`
	Variables                   []*customTemplateVariable `json:"variables"`
}

type portainerCustomTemplate struct {
	ID              int64  `json:"Id"`
	Title           string `json:"Title"`
	Description     string `json:"Description"`
	Note            string `json:"Note"`
	Platform        int64  `json:"Platform"`
	Type            int64  `json:"Type"`
	Logo            string `json:"Logo"`
	EdgeTemplate    bool   `json:"edgeTemplate"`
	IsComposeFormat bool   `json:"isComposeFormat"`
}

// listCustomTemplates returns every custom template (GET /custom_templates).
func listCustomTemplates(ctx context.Context, client *APIClient) ([]portainerCustomTemplate, error) {
	var templates []portainerCustomTemplate
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/custom_templates", nil, &templates); err != nil {
		return nil, fmt.Errorf("failed to list custom templates: %w", err)
	}
	return templates, nil
}
