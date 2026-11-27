package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/int64validator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/booldefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/boolplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/int64planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringdefault"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ resource.Resource                = (*customTemplateResource)(nil)
	_ resource.ResourceWithConfigure   = (*customTemplateResource)(nil)
	_ resource.ResourceWithImportState = (*customTemplateResource)(nil)
	_ resource.ResourceWithIdentity    = (*customTemplateResource)(nil)
)

func newCustomTemplateResource() resource.Resource { return &customTemplateResource{} }

type customTemplateResource struct {
	fwIDIdentity
	client *APIClient
}

type customTemplateModel struct {
	ID                          types.String `tfsdk:"id"`
	Title                       types.String `tfsdk:"title"`
	Description                 types.String `tfsdk:"description"`
	Note                        types.String `tfsdk:"note"`
	Platform                    types.Int64  `tfsdk:"platform"`
	Type                        types.Int64  `tfsdk:"type"`
	Logo                        types.String `tfsdk:"logo"`
	EdgeTemplate                types.Bool   `tfsdk:"edge_template"`
	IsComposeFormat             types.Bool   `tfsdk:"is_compose_format"`
	Variables                   types.List   `tfsdk:"variables"`
	FileContent                 types.String `tfsdk:"file_content"`
	FilePath                    types.String `tfsdk:"file_path"`
	RepositoryURL               types.String `tfsdk:"repository_url"`
	RepositoryUsername          types.String `tfsdk:"repository_username"`
	RepositoryPassword          types.String `tfsdk:"repository_password"`
	RepositoryPasswordWO        types.String `tfsdk:"repository_password_wo"`
	RepositoryPasswordWOVersion types.Int64  `tfsdk:"repository_password_wo_version"`
	RepositoryReference         types.String `tfsdk:"repository_reference"`
	ComposeFilePath             types.String `tfsdk:"compose_file_path"`
	SourceID                    types.Int64  `tfsdk:"source_id"`
	TLSSkipVerify               types.Bool   `tfsdk:"tlsskip_verify"`
	RepositoryAuthentication    types.Bool   `tfsdk:"repository_authentication"`
	EdgeSettings                types.List   `tfsdk:"edge_settings"`
}

func (r *customTemplateResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_custom_template"
}

func (r *customTemplateResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	replace := []planmodifier.String{stringplanmodifier.RequiresReplace()}
	boolReplace := []planmodifier.Bool{boolplanmodifier.RequiresReplace()}
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"title":       schema.StringAttribute{Required: true, Description: "Title of the Portainer custom template."},
			"description": schema.StringAttribute{Required: true, Description: "Short description of the custom template displayed in the Portainer UI."},
			"note":        schema.StringAttribute{Required: true, Description: "Additional note or instructions associated with the custom template."},
			"platform": schema.Int64Attribute{
				Required:    true,
				Validators:  []validator.Int64{int64validator.Between(1, 2)},
				Description: "Target platform for the template: 1 = Linux, 2 = Windows.",
			},
			"type": schema.Int64Attribute{
				Required:    true,
				Validators:  []validator.Int64{int64validator.Between(1, 3)},
				Description: "Template type: 1 = Swarm stack, 2 = Compose stack, 3 = Kubernetes manifest.",
			},
			"logo":              schema.StringAttribute{Optional: true, Description: "URL of the logo image displayed next to the template in the Portainer UI."},
			"edge_template":     schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether this template is exposed as an Edge template."},
			"is_compose_format": schema.BoolAttribute{Optional: true, Computed: true, Default: booldefault.StaticBool(false), Description: "Whether the Kubernetes manifest is provided in Compose format (only relevant for `type = 3`)."},
			"variables": schema.ListAttribute{
				Optional:    true,
				ElementType: types.MapType{ElemType: types.StringType},
				Description: "List of template variable definitions (name, label, description, defaultValue) used for parameterized templates.",
			},
			"file_content":                   schema.StringAttribute{Optional: true, Description: "Inline template body (Compose YAML, Kubernetes manifest, or Swarm stack file). Mutually exclusive with `file_path` and the repository fields."},
			"file_path":                      schema.StringAttribute{Optional: true, PlanModifiers: replace, Description: "Path on the local filesystem to a file containing the template body. Mutually exclusive with `file_content` and the repository fields."},
			"repository_url":                 schema.StringAttribute{Optional: true, PlanModifiers: replace, Description: "URL of the Git repository hosting the template manifest."},
			"repository_username":            schema.StringAttribute{Optional: true, PlanModifiers: replace, Description: "Username used to authenticate against the Git repository."},
			"repository_password":            schema.StringAttribute{Optional: true, Sensitive: true, PlanModifiers: replace, Description: "Sensitive password or personal access token used to authenticate against the Git repository. Stored in state; use `repository_password_wo` to keep it out."},
			"repository_password_wo":         gsbWriteOnlySecretAttribute("repository_password", "repository_password_wo_version", "Git password or personal access token"),
			"repository_password_wo_version": gsbWriteOnlyVersionAttribute("repository_password_wo", true),
			"repository_reference": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("refs/heads/main"), PlanModifiers: replace,
				Description: "Git reference (branch, tag, or commit) to check out (defaults to `refs/heads/main`).",
			},
			"compose_file_path": schema.StringAttribute{
				Optional: true, Computed: true, Default: stringdefault.StaticString("docker-compose.yml"), PlanModifiers: replace,
				Description: "Path within the Git repository to the Compose or manifest file (defaults to `docker-compose.yml`).",
			},
			"source_id": schema.Int64Attribute{
				Optional:      true,
				PlanModifiers: []planmodifier.Int64{int64planmodifier.RequiresReplace()},
				Description:   "ID of a Portainer Source (Git source) providing the repository URL and credentials. Portainer 2.43 and later. When set, `repository_url` and the repository credentials can be left out; `repository_reference` and `compose_file_path` still apply.",
			},
			"tlsskip_verify": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false), PlanModifiers: boolReplace,
				Description: "Whether to skip TLS verification when cloning the Git repository.",
			},
			"repository_authentication": schema.BoolAttribute{
				Optional: true, Computed: true, Default: booldefault.StaticBool(false), PlanModifiers: boolReplace,
				Description: "Enable authentication for the Git repository (default: false).",
			},
		},
		Blocks: map[string]schema.Block{
			"edge_settings": customTemplateEdgeSettingsBlock(),
		},
	}
}

func (r *customTemplateResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
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

// customTemplateVariables reads the variables list (nil when unset).
func customTemplateVariables(ctx context.Context, m *customTemplateModel, diags *diag.Diagnostics) []*customTemplateVariable {
	if m.Variables.IsNull() || m.Variables.IsUnknown() || len(m.Variables.Elements()) == 0 {
		return nil
	}
	var vars []map[string]string
	diags.Append(m.Variables.ElementsAs(ctx, &vars, false)...)
	result := make([]*customTemplateVariable, 0, len(vars))
	for _, v := range vars {
		result = append(result, &customTemplateVariable{
			Name:         v["name"],
			Label:        v["label"],
			DefaultValue: v["default_value"],
			Description:  v["description"],
		})
	}
	return result
}

func (r *customTemplateResource) repositoryPassword(ctx context.Context, config tfsdk.Config, m *customTemplateModel, diags *diag.Diagnostics) string {
	password, _ := gsbTwinSecret(ctx, config, m.RepositoryPassword, path.Root("repository_password_wo"), m.RepositoryPasswordWOVersion, diags)
	return password
}

func (r *customTemplateResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan customTemplateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}

	existingID, err := findExistingCustomTemplateByTitle(ctx, r.client, plan.Title.ValueString())
	if err != nil {
		resp.Diagnostics.AddError("Failed to create custom template", fmt.Sprintf("failed to check for existing custom template: %s", err))
		return
	}
	switch {
	case existingID != 0:
		// A template with this title is adopted and brought in line.
		plan.ID = types.StringValue(strconv.Itoa(existingID))
		r.update(ctx, req.Config, &plan, &resp.Diagnostics)
	case plan.FileContent.ValueString() != "":
		r.createFromString(ctx, &plan, plan.FileContent.ValueString(), &resp.Diagnostics)
	case plan.FilePath.ValueString() != "":
		// SDKv2 also copied the file into file_content, which then planned its
		// removal on every run; the framework keeps file_content as configured.
		content, err := os.ReadFile(plan.FilePath.ValueString())
		if err != nil {
			resp.Diagnostics.AddError("Failed to create custom template", fmt.Sprintf("failed to read template file from path: %s", err))
			return
		}
		r.createFromString(ctx, &plan, string(content), &resp.Diagnostics)
	case plan.RepositoryURL.ValueString() != "" || plan.SourceID.ValueInt64() != 0:
		r.createFromRepository(ctx, req.Config, &plan, &resp.Diagnostics)
	default:
		resp.Diagnostics.AddError("Failed to create custom template", "one of file_content, file_path, repository_url, or source_id must be provided")
	}
	if resp.Diagnostics.HasError() {
		return
	}
	plan.RepositoryPasswordWO = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	gsbMarkFrameworkState(ctx, resp.Private, &resp.Diagnostics)
}

func (r *customTemplateResource) createFromString(ctx context.Context, m *customTemplateModel, content string, diags *diag.Diagnostics) {
	body := customTemplateFromStringPayload{
		Title:        m.Title.ValueString(),
		Description:  m.Description.ValueString(),
		Note:         m.Note.ValueString(),
		Platform:     m.Platform.ValueInt64(),
		Type:         m.Type.ValueInt64(),
		Logo:         m.Logo.ValueString(),
		EdgeTemplate: m.EdgeTemplate.ValueBool(),
		FileContent:  content,
		Variables:    customTemplateVariables(ctx, m, diags),
		EdgeSettings: customTemplateEdgeSettingsPayload(ctx, m.EdgeSettings, diags),
	}
	var created portainerCustomTemplate
	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/custom_templates/create/string", body, &created); err != nil {
		diags.AddError("Failed to create custom template", fmt.Sprintf("failed to create custom template from string: %s", err))
		return
	}
	m.ID = types.StringValue(strconv.FormatInt(created.ID, 10))
}

func (r *customTemplateResource) createFromRepository(ctx context.Context, config tfsdk.Config, m *customTemplateModel, diags *diag.Diagnostics) {
	repoURL := m.RepositoryURL.ValueString()
	useAuth := m.RepositoryAuthentication.ValueBool()
	composePath := m.ComposeFilePath.ValueString()
	body := &customTemplateFromRepositoryPayload{
		Title:                       m.Title.ValueString(),
		Description:                 m.Description.ValueString(),
		Note:                        m.Note.ValueString(),
		Platform:                    m.Platform.ValueInt64(),
		Type:                        m.Type.ValueInt64(),
		Logo:                        m.Logo.ValueString(),
		EdgeTemplate:                m.EdgeTemplate.ValueBool(),
		IsComposeFormat:             m.IsComposeFormat.ValueBool(),
		RepositoryURL:               &repoURL,
		RepositoryAuthentication:    useAuth,
		RepositoryReferenceName:     m.RepositoryReference.ValueString(),
		ComposeFilePathInRepository: &composePath,
		TLSSkipVerify:               m.TLSSkipVerify.ValueBool(),
		Variables:                   customTemplateVariables(ctx, m, diags),
		EdgeSettings:                customTemplateEdgeSettingsPayload(ctx, m.EdgeSettings, diags),
	}
	if useAuth {
		body.RepositoryUsername = m.RepositoryUsername.ValueString()
		body.RepositoryPassword = r.repositoryPassword(ctx, config, m, diags)
	}
	if diags.HasError() {
		return
	}

	// A template built from a Source goes out as the same body plus SourceID.
	if sourceID := int(m.SourceID.ValueInt64()); sourceID != 0 {
		withSource, err := withSourceID(body, sourceID)
		if err != nil {
			diags.AddError("Failed to create custom template", err.Error())
			return
		}
		var created struct {
			ID int `json:"Id"`
		}
		if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/custom_templates/create/repository", withSource, &created); err != nil {
			diags.AddError("Failed to create custom template", fmt.Sprintf("failed to create custom template from source %d: %s", sourceID, err))
			return
		}
		m.ID = types.StringValue(strconv.Itoa(created.ID))
		return
	}

	var created portainerCustomTemplate
	if err := doJSON(ctx, r.client, http.MethodPost, r.client.Endpoint+"/custom_templates/create/repository", body, &created); err != nil {
		diags.AddError("Failed to create custom template", fmt.Sprintf("failed to create custom template from repository: %s", err))
		return
	}
	m.ID = types.StringValue(strconv.FormatInt(created.ID, 10))
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

func (r *customTemplateResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	fwSetIDIdentity(ctx, req.State, resp.Identity, &resp.Diagnostics)
	var state customTemplateModel
	resp.Diagnostics.Append(req.State.Get(ctx, &state)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, _ := strconv.ParseInt(state.ID.ValueString(), 10, 64)

	var tmpl portainerCustomTemplate
	if err := doJSON(ctx, r.client, http.MethodGet, fmt.Sprintf("%s/custom_templates/%d", r.client.Endpoint, id), nil, &tmpl); err != nil {
		if isAPINotFound(err) {
			resp.State.RemoveResource(ctx)
			return
		}
		resp.Diagnostics.AddError("Failed to read custom template", fmt.Sprintf("failed to read custom template: %s", err))
		return
	}

	state.Title = types.StringValue(tmpl.Title)
	state.Description = types.StringValue(tmpl.Description)
	state.Note = types.StringValue(tmpl.Note)
	state.Platform = types.Int64Value(tmpl.Platform)
	state.Type = types.Int64Value(tmpl.Type)
	state.Logo = gsbString(tmpl.Logo, state.Logo, gsbFrameworkState(ctx, req.Private))
	state.EdgeTemplate = types.BoolValue(tmpl.EdgeTemplate)
	state.IsComposeFormat = types.BoolValue(tmpl.IsComposeFormat)
	state.EdgeSettings = customTemplateEdgeSettingsFromAPI(ctx, state.EdgeSettings, tmpl.EdgeSettings, &resp.Diagnostics)
	resp.Diagnostics.Append(resp.State.Set(ctx, &state)...)
	// A list resource reads the template with no private state to mark.
	if resp.Private != nil {
		gsbMarkFrameworkState(ctx, resp.Private, &resp.Diagnostics)
	}
}

func (r *customTemplateResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	defer func() { fwSetIDIdentity(ctx, resp.State, resp.Identity, &resp.Diagnostics) }()
	var plan customTemplateModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	if resp.Diagnostics.HasError() {
		return
	}
	r.update(ctx, req.Config, &plan, &resp.Diagnostics)
	if resp.Diagnostics.HasError() {
		return
	}
	plan.RepositoryPasswordWO = types.StringNull()
	resp.Diagnostics.Append(resp.State.Set(ctx, &plan)...)
	gsbMarkFrameworkState(ctx, resp.Private, &resp.Diagnostics)
}

// update sends the template (PUT), fetches a Git-based one again and reads
// it back; the configured values are kept in state, as the framework rejects
// a configured value changing during apply (a difference shows on the next
// refresh).
func (r *customTemplateResource) update(ctx context.Context, config tfsdk.Config, m *customTemplateModel, diags *diag.Diagnostics) {
	id, _ := strconv.ParseInt(m.ID.ValueString(), 10, 64)

	var fileContent string
	if p := m.FilePath.ValueString(); p != "" {
		content, err := os.ReadFile(p)
		if err != nil {
			diags.AddError("Failed to update custom template", fmt.Sprintf("failed to read template file from path: %s", err))
			return
		}
		fileContent = string(content)
	} else {
		fileContent = m.FileContent.ValueString()
	}

	composePath := m.ComposeFilePath.ValueString()
	useAuth := m.RepositoryAuthentication.ValueBool()
	payload := &customTemplateUpdatePayload{
		Title:                       m.Title.ValueString(),
		Description:                 m.Description.ValueString(),
		Note:                        m.Note.ValueString(),
		Platform:                    m.Platform.ValueInt64(),
		Type:                        m.Type.ValueInt64(),
		Logo:                        m.Logo.ValueString(),
		EdgeTemplate:                m.EdgeTemplate.ValueBool(),
		IsComposeFormat:             m.IsComposeFormat.ValueBool(),
		ComposeFilePathInRepository: &composePath,
		TLSSkipVerify:               m.TLSSkipVerify.ValueBool(),
		FileContent:                 fileContent,
		Variables:                   customTemplateVariables(ctx, m, diags),
		EdgeSettings:                customTemplateEdgeSettingsPayload(ctx, m.EdgeSettings, diags),
	}

	isGitBased := false
	if repoURL := m.RepositoryURL.ValueString(); repoURL != "" || m.SourceID.ValueInt64() != 0 {
		isGitBased = true
		payload.RepositoryURL = &repoURL
		payload.RepositoryReferenceName = m.RepositoryReference.ValueString()
		payload.RepositoryAuthentication = useAuth
		if useAuth {
			payload.RepositoryUsername = m.RepositoryUsername.ValueString()
			payload.RepositoryPassword = r.repositoryPassword(ctx, config, m, diags)
		}
	}
	if diags.HasError() {
		return
	}

	var body interface{} = payload
	if sourceID := int(m.SourceID.ValueInt64()); sourceID != 0 {
		// A Source-based template has to name its Source on every update, or
		// Portainer finds neither content nor a repository and rejects it.
		withSource, err := withSourceID(payload, sourceID)
		if err != nil {
			diags.AddError("Failed to update custom template", err.Error())
			return
		}
		body = withSource
	}
	if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/custom_templates/%d", r.client.Endpoint, id), body, nil); err != nil {
		diags.AddError("Failed to update custom template", fmt.Sprintf("failed to update custom template: %s", err))
		return
	}

	if isGitBased {
		if err := doJSON(ctx, r.client, http.MethodPut, fmt.Sprintf("%s/custom_templates/%d/git_fetch", r.client.Endpoint, id), nil, nil); err != nil {
			diags.AddError("Failed to update custom template", fmt.Sprintf("failed to git_fetch template: %s", err))
			return
		}
	}

	var tmpl portainerCustomTemplate
	if err := doJSON(ctx, r.client, http.MethodGet, fmt.Sprintf("%s/custom_templates/%d", r.client.Endpoint, id), nil, &tmpl); err != nil && !isAPINotFound(err) {
		diags.AddError("Failed to read custom template", fmt.Sprintf("failed to read custom template: %s", err))
	}
}

func (r *customTemplateResource) Delete(ctx context.Context, req resource.DeleteRequest, resp *resource.DeleteResponse) {
	var idValue types.String
	resp.Diagnostics.Append(req.State.GetAttribute(ctx, path.Root("id"), &idValue)...)
	if resp.Diagnostics.HasError() {
		return
	}
	id, _ := strconv.ParseInt(idValue.ValueString(), 10, 64)
	if err := doJSON(ctx, r.client, http.MethodDelete, fmt.Sprintf("%s/custom_templates/%d", r.client.Endpoint, id), nil, nil); err != nil && !isAPINotFound(err) {
		resp.Diagnostics.AddError("Failed to delete custom template", fmt.Sprintf("failed to delete custom template: %s", err))
	}
}

func (r *customTemplateResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	fwImportIDOrIdentity(ctx, req, resp)
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
	EdgeSettings map[string]interface{}    `json:"edgeSettings,omitempty"`
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
	EdgeSettings                map[string]interface{}    `json:"edgeSettings,omitempty"`
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
	EdgeSettings                map[string]interface{}    `json:"edgeSettings,omitempty"`
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
	// Business Edition only.
	EdgeSettings *customTemplateEdgeSettingsAPI `json:"EdgeSettings"`
}

// listCustomTemplates returns every custom template (GET /custom_templates).
func listCustomTemplates(ctx context.Context, client *APIClient) ([]portainerCustomTemplate, error) {
	var templates []portainerCustomTemplate
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/custom_templates", nil, &templates); err != nil {
		return nil, fmt.Errorf("failed to list custom templates: %w", err)
	}
	return templates, nil
}
