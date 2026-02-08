package internal

import (
	"fmt"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/custom_templates"
	"github.com/portainer/client-api-go/v2/pkg/models"
)

func resourceCustomTemplate() *schema.Resource {
	return &schema.Resource{
		Create: resourceCustomTemplateCreate,
		Read:   resourceCustomTemplateRead,
		Delete: resourceCustomTemplateDelete,
		Update: resourceCustomTemplateUpdate,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"title":                {Type: schema.TypeString, Required: true},
			"description":          {Type: schema.TypeString, Required: true},
			"note":                 {Type: schema.TypeString, Required: true},
			"platform":             {Type: schema.TypeInt, Required: true},
			"type":                 {Type: schema.TypeInt, Required: true},
			"logo":                 {Type: schema.TypeString, Optional: true},
			"edge_template":        {Type: schema.TypeBool, Optional: true, Default: false},
			"is_compose_format":    {Type: schema.TypeBool, Optional: true, Default: false},
			"variables":            {Type: schema.TypeList, Optional: true, Elem: &schema.Schema{Type: schema.TypeMap}},
			"file_content":         {Type: schema.TypeString, Optional: true},
			"file_path":            {Type: schema.TypeString, Optional: true, ForceNew: true},
			"repository_url":       {Type: schema.TypeString, Optional: true, ForceNew: true},
			"repository_username":  {Type: schema.TypeString, Optional: true, ForceNew: true},
			"repository_password":  {Type: schema.TypeString, Optional: true, Sensitive: true, ForceNew: true},
			"repository_reference": {Type: schema.TypeString, Optional: true, Default: "refs/heads/main", ForceNew: true},
			"compose_file_path":    {Type: schema.TypeString, Optional: true, Default: "docker-compose.yml", ForceNew: true},
			"tlsskip_verify":       {Type: schema.TypeBool, Optional: true, Default: false, ForceNew: true},
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

func findExistingCustomTemplateByTitle(client *APIClient, title string) (int, error) {
	params := custom_templates.NewCustomTemplateListParams()
	resp, err := client.Client.CustomTemplates.CustomTemplateList(params, client.AuthInfo)
	if err != nil {
		return 0, fmt.Errorf("failed to list custom templates: %w", err)
	}

	for _, tmpl := range resp.Payload {
		if tmpl.Title == title {
			return int(tmpl.ID), nil
		}
	}

	return 0, nil
}

func resourceCustomTemplateCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	title := d.Get("title").(string)

	existingID, err := findExistingCustomTemplateByTitle(client, title)
	if err != nil {
		return fmt.Errorf("failed to check for existing custom template: %w", err)
	} else if existingID != 0 {
		d.SetId(strconv.Itoa(existingID))
		return resourceCustomTemplateUpdate(d, meta)
	}

	if v, ok := d.GetOk("file_content"); ok {
		return createTemplateFromString(d, client, v.(string))
	}

	if v, ok := d.GetOk("file_path"); ok {
		content, err := os.ReadFile(v.(string))
		if err != nil {
			return fmt.Errorf("failed to read template file from path: %w", err)
		}
		d.Set("file_content", string(content))
		return createTemplateFromString(d, client, string(content))
	}

	if v, ok := d.GetOk("repository_url"); ok {
		return createTemplateFromRepository(d, client, v.(string))
	}

	return fmt.Errorf("one of file_content, file_path, or repository_url must be provided")
}

func createTemplateFromString(d *schema.ResourceData, client *APIClient, content string) error {
	title := d.Get("title").(string)
	description := d.Get("description").(string)
	templateType := int64(d.Get("type").(int))

	params := custom_templates.NewCustomTemplateCreateStringParams()
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
		return fmt.Errorf("failed to create custom template from string: %w", err)
	}

	d.SetId(strconv.FormatInt(resp.Payload.ID, 10))
	return nil
}

func createTemplateFromRepository(d *schema.ResourceData, client *APIClient, repoURL string) error {
	title := d.Get("title").(string)
	description := d.Get("description").(string)
	templateType := int64(d.Get("type").(int))
	useAuth := d.Get("repository_authentication").(bool)

	params := custom_templates.NewCustomTemplateCreateRepositoryParams()
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
		params.Body.RepositoryPassword = d.Get("repository_password").(string)
	}

	resp, err := client.Client.CustomTemplates.CustomTemplateCreateRepository(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to create custom template from repository: %w", err)
	}

	d.SetId(strconv.FormatInt(resp.Payload.ID, 10))
	return nil
}

func getVariables(d *schema.ResourceData) []interface{} {
	if v, ok := d.GetOk("variables"); ok {
		return v.([]interface{})
	}
	return []interface{}{}
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

func resourceCustomTemplateRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	params := custom_templates.NewCustomTemplateInspectParams()
	params.ID = id

	resp, err := client.Client.CustomTemplates.CustomTemplateInspect(params, client.AuthInfo)
	if err != nil {
		if _, ok := err.(*custom_templates.CustomTemplateInspectNotFound); ok {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("failed to read custom template: %w", err)
	}

	d.Set("title", resp.Payload.Title)
	d.Set("description", resp.Payload.Description)
	d.Set("note", resp.Payload.Note)
	d.Set("platform", int(resp.Payload.Platform))
	d.Set("type", int(resp.Payload.Type))
	d.Set("logo", resp.Payload.Logo)
	d.Set("edge_template", resp.Payload.EdgeTemplate)
	d.Set("is_compose_format", resp.Payload.IsComposeFormat)

	return nil
}

func resourceCustomTemplateUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	title := d.Get("title").(string)
	description := d.Get("description").(string)
	templateType := int64(d.Get("type").(int))

	var fileContent string
	if v, ok := d.GetOk("file_path"); ok {
		content, err := os.ReadFile(v.(string))
		if err != nil {
			return fmt.Errorf("failed to read template file from path: %w", err)
		}
		fileContent = string(content)
		d.Set("file_content", fileContent)
	} else if v, ok := d.GetOk("file_content"); ok {
		fileContent = v.(string)
	}

	composePath := d.Get("compose_file_path").(string)
	useAuth := d.Get("repository_authentication").(bool)

	params := custom_templates.NewCustomTemplateUpdateParams()
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
	if v, ok := d.GetOk("repository_url"); ok {
		isGitBased = true
		repoURL := v.(string)
		params.Body.RepositoryURL = &repoURL
		params.Body.RepositoryReferenceName = d.Get("repository_reference").(string)
		params.Body.RepositoryAuthentication = useAuth
		if useAuth {
			params.Body.RepositoryUsername = d.Get("repository_username").(string)
			params.Body.RepositoryPassword = d.Get("repository_password").(string)
		}
	}

	_, err := client.Client.CustomTemplates.CustomTemplateUpdate(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to update custom template: %w", err)
	}

	if isGitBased {
		gitParams := custom_templates.NewCustomTemplateGitFetchParams()
		gitParams.ID = id
		_, err := client.Client.CustomTemplates.CustomTemplateGitFetch(gitParams, client.AuthInfo)
		if err != nil {
			return fmt.Errorf("failed to git_fetch template: %w", err)
		}
	}

	return resourceCustomTemplateRead(d, meta)
}

func resourceCustomTemplateDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	params := custom_templates.NewCustomTemplateDeleteParams()
	params.ID = id

	_, err := client.Client.CustomTemplates.CustomTemplateDelete(params, client.AuthInfo)
	if err != nil {
		if _, ok := err.(*custom_templates.CustomTemplateDeleteNotFound); ok {
			return nil
		}
		return fmt.Errorf("failed to delete custom template: %w", err)
	}
	return nil
}
