package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-framework-validators/datasourcevalidator"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

var (
	_ datasource.DataSourceWithConfigure        = (*policyDataSource)(nil)
	_ datasource.DataSourceWithConfigValidators = (*policyDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*policyTemplateDataSource)(nil)
	_ datasource.DataSourceWithConfigValidators = (*policyTemplateDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*policyMetadataDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*policyConflictsDataSource)(nil)
	_ datasource.DataSourceWithConfigure        = (*policyObservabilityTestDataSource)(nil)
)

// ---------------------------------------------------------------------------
// portainer_policy

func newPolicyDataSource() datasource.DataSource { return &policyDataSource{} }

type policyDataSource struct{ client *APIClient }

type policyDataSourceModel struct {
	ID                types.String `tfsdk:"id"`
	PolicyID          types.Int64  `tfsdk:"policy_id"`
	Name              types.String `tfsdk:"name"`
	EnvironmentType   types.String `tfsdk:"environment_type"`
	PolicyType        types.String `tfsdk:"policy_type"`
	EnvironmentGroups types.List   `tfsdk:"environment_groups"`
	Data              types.String `tfsdk:"data"`
	CreatedAt         types.String `tfsdk:"created_at"`
	UpdatedAt         types.String `tfsdk:"updated_at"`
}

func (d *policyDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy"
}

func (d *policyDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"policy_id": schema.Int64Attribute{
				Optional:    true,
				Description: "ID of the policy. If set, the policy is looked up by ID directly.",
			},
			// Computed too: the lookup by ID fills it in.
			"name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Name of the policy. If set, the policy is looked up by name from the list.",
			},
			"environment_type":   schema.StringAttribute{Computed: true, Description: "Environment type for the policy."},
			"policy_type":        schema.StringAttribute{Computed: true, Description: "Policy type."},
			"environment_groups": schema.ListAttribute{Computed: true, ElementType: types.Int64Type, Description: "List of environment group IDs."},
			"data":               schema.StringAttribute{Computed: true, Description: "Policy data as a JSON string."},
			"created_at":         schema.StringAttribute{Computed: true, Description: "Timestamp when the policy was created."},
			"updated_at":         schema.StringAttribute{Computed: true, Description: "Timestamp when the policy was last updated."},
		},
	}
}

func (d *policyDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(path.MatchRoot("policy_id"), path.MatchRoot("name")),
	}
}

func (d *policyDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *policyDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m policyDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	// If policy_id is provided, look up directly.
	policyID := 0
	if !m.PolicyID.IsNull() && m.PolicyID.ValueInt64() != 0 {
		policyID = int(m.PolicyID.ValueInt64())
	} else {
		// Otherwise, look up by name from the list.
		name := m.Name.ValueString()
		var listResp struct {
			Policies []map[string]interface{} `json:"policies"`
		}
		if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/policies", nil, &listResp); err != nil {
			resp.Diagnostics.AddError("Failed to list policies", fmt.Errorf("failed to list policies: %w", err).Error())
			return
		}
		found := false
		for _, p := range listResp.Policies {
			if pName, ok := p["Name"].(string); ok && pName == name {
				if id, ok := p["Id"].(float64); ok {
					policyID, found = int(id), true
					break
				}
			}
		}
		if !found {
			resp.Diagnostics.AddError("Policy not found", fmt.Sprintf("policy with name %q not found", name))
			return
		}
	}

	idStr := strconv.Itoa(policyID)
	got, err := readPolicyAPI(ctx, d.client, idStr)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read policy", fmt.Errorf("failed to read policy %d: %w", policyID, err).Error())
		return
	}
	m.ID = types.StringValue(idStr)
	m.Name = policyStringOr(got.Name, m.Name)
	m.EnvironmentType = policyStringOr(got.environmentType(), types.StringNull())
	m.PolicyType = policyStringOr(got.PolicyType, types.StringNull())
	m.CreatedAt = policyStringOr(got.CreatedAt, types.StringNull())
	m.UpdatedAt = policyStringOr(got.UpdatedAt, types.StringNull())
	m.EnvironmentGroups = types.ListNull(types.Int64Type)
	if got.HasGroups {
		m.EnvironmentGroups = int64ListValue(got.EnvironmentGroups)
	}
	m.Data = policyStringOr(got.Data, types.StringNull())
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// ---------------------------------------------------------------------------
// portainer_policy_template

func newPolicyTemplateDataSource() datasource.DataSource { return &policyTemplateDataSource{} }

type policyTemplateDataSource struct{ client *APIClient }

type policyTemplateDataSourceModel struct {
	ID          types.String `tfsdk:"id"`
	TemplateID  types.String `tfsdk:"template_id"`
	Name        types.String `tfsdk:"name"`
	Description types.String `tfsdk:"description"`
	Category    types.String `tfsdk:"category"`
	PolicyType  types.String `tfsdk:"policy_type"`
	Data        types.String `tfsdk:"data"`
}

func (d *policyTemplateDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy_template"
}

func (d *policyTemplateDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"template_id": schema.StringAttribute{
				Optional:    true,
				Description: "ID of the policy template. If set, the template is looked up by ID directly.",
			},
			// Computed too: the lookup by ID fills it in.
			"name": schema.StringAttribute{
				Optional:    true,
				Computed:    true,
				Description: "Name of the policy template. If set, the template is looked up by name from the list.",
			},
			"description": schema.StringAttribute{Computed: true, Description: "Description of the policy template."},
			"category":    schema.StringAttribute{Computed: true, Description: "Category of the policy template (rbac, security, setup, registry)."},
			"policy_type": schema.StringAttribute{Computed: true, Description: "Policy type of the template."},
			"data":        schema.StringAttribute{Computed: true, Description: "Template data as a JSON string."},
		},
	}
}

func (d *policyTemplateDataSource) ConfigValidators(context.Context) []datasource.ConfigValidator {
	return []datasource.ConfigValidator{
		datasourcevalidator.ExactlyOneOf(path.MatchRoot("template_id"), path.MatchRoot("name")),
	}
}

func (d *policyTemplateDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *policyTemplateDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m policyTemplateDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	templateID := m.TemplateID.ValueString()
	if templateID == "" {
		name := m.Name.ValueString()
		var listResp struct {
			Templates []map[string]interface{} `json:"templates"`
		}
		if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/policies/templates", nil, &listResp); err != nil {
			resp.Diagnostics.AddError("Failed to list policy templates", fmt.Errorf("failed to list policy templates: %w", err).Error())
			return
		}
		for _, t := range listResp.Templates {
			if tName, ok := t["name"].(string); ok && tName == name {
				if id, ok := t["id"].(string); ok {
					templateID = id
					break
				}
			}
		}
		if templateID == "" {
			resp.Diagnostics.AddError("Policy template not found", fmt.Sprintf("policy template with name %q not found", name))
			return
		}
	}

	var tmpl map[string]interface{}
	if err := doJSON(ctx, d.client, http.MethodGet, fmt.Sprintf("%s/policies/templates/%s", d.client.Endpoint, templateID), nil, &tmpl); err != nil {
		resp.Diagnostics.AddError("Failed to read policy template", fmt.Errorf("failed to read policy template %s: %w", templateID, err).Error())
		return
	}
	str := func(key string, current types.String) types.String {
		v, ok := tmpl[key]
		if !ok {
			return current
		}
		s, _ := v.(string)
		return types.StringValue(s)
	}
	m.ID = types.StringValue(templateID)
	m.Name = str("name", m.Name)
	m.Description = str("description", types.StringNull())
	m.Category = str("category", types.StringNull())
	m.PolicyType = str("type", types.StringNull())
	m.Data = types.StringNull()
	if data, ok := tmpl["data"]; ok && data != nil {
		if dataJSON, err := json.Marshal(data); err == nil {
			m.Data = types.StringValue(string(dataJSON))
		}
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// ---------------------------------------------------------------------------
// portainer_policy_metadata

func newPolicyMetadataDataSource() datasource.DataSource { return &policyMetadataDataSource{} }

type policyMetadataDataSource struct{ client *APIClient }

type policyMetadataDataSourceModel struct {
	ID                   types.String `tfsdk:"id"`
	MinimumAgentVersions types.Map    `tfsdk:"minimum_agent_versions"`
}

func (d *policyMetadataDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy_metadata"
}

func (d *policyMetadataDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"minimum_agent_versions": schema.MapAttribute{
				Computed:    true,
				ElementType: types.StringType,
				Description: "The lowest agent version each policy type needs, keyed by policy type. An environment on an older agent will not run that policy.",
			},
		},
	}
}

func (d *policyMetadataDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *policyMetadataDataSource) Read(ctx context.Context, _ datasource.ReadRequest, resp *datasource.ReadResponse) {
	var metadata struct {
		MinimumAgentVersions map[string]string `json:"minimumAgentVersions"`
	}
	if err := doJSON(ctx, d.client, http.MethodGet, d.client.Endpoint+"/policies/metadata", nil, &metadata); err != nil {
		resp.Diagnostics.AddError("Failed to read the policy metadata", fmt.Errorf("failed to read the policy metadata: %w", err).Error())
		return
	}
	m := policyMetadataDataSourceModel{
		ID:                   types.StringValue("portainer-policy-metadata"),
		MinimumAgentVersions: stringMapValue(metadata.MinimumAgentVersions),
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// ---------------------------------------------------------------------------
// portainer_policy_conflicts

// policyConflictsDataSource previews what a policy would collide with before
// it is created. It is a data source rather than a resource because the
// endpoint changes nothing: it answers a question about a candidate policy.
func newPolicyConflictsDataSource() datasource.DataSource { return &policyConflictsDataSource{} }

type policyConflictsDataSource struct{ client *APIClient }

type policyConflictsDataSourceModel struct {
	ID                      types.String `tfsdk:"id"`
	PolicyJSON              types.String `tfsdk:"policy_json"`
	TotalEnvironments       types.Int64  `tfsdk:"total_environments"`
	SupportedEnvironments   types.Int64  `tfsdk:"supported_environments"`
	UnsupportedEnvironments types.Int64  `tfsdk:"unsupported_environments"`
	Conflicts               types.List   `tfsdk:"conflicts"`
	NewGroups               types.List   `tfsdk:"new_groups"`
}

func (d *policyConflictsDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy_conflicts"
}

func (d *policyConflictsDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"policy_json": schema.StringAttribute{
				Required:   true,
				Validators: []validator.String{policyJSONObjectValidator{}},
				// Portainer does not give this payload a shape in its own
				// specification, so it is passed through as JSON rather than
				// flattened into arguments that could not be kept accurate.
				Description: "The candidate policy, as a JSON object. Use `jsonencode({...})`. Portainer does not describe this payload's shape in its API specification, so it is passed through as given.",
			},
			"total_environments":       schema.Int64Attribute{Computed: true, Description: "Environments the candidate policy would reach."},
			"supported_environments":   schema.Int64Attribute{Computed: true, Description: "Of those, the ones that can run it."},
			"unsupported_environments": schema.Int64Attribute{Computed: true, Description: "Of those, the ones that cannot - usually an agent too old for the policy type. Check `portainer_policy_metadata` for the versions each type needs."},
			"conflicts": schema.ListAttribute{
				Computed:    true,
				ElementType: policyConflictObjectType,
				Description: "Environment groups where an existing policy already applies.",
			},
			"new_groups": schema.ListAttribute{
				Computed:    true,
				ElementType: policyNewGroupObjectType,
				Description: "Environment groups the candidate policy would newly cover, with no existing policy in the way.",
			},
		},
	}
}

func (d *policyConflictsDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *policyConflictsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m policyConflictsDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(m.PolicyJSON.ValueString()), &payload); err != nil {
		resp.Diagnostics.AddError("Invalid candidate policy", fmt.Errorf("failed to parse the candidate policy as a JSON object: %w", err).Error())
		return
	}

	var response struct {
		TotalEnvironments       int `json:"totalEnvironments"`
		SupportedEnvironments   int `json:"supportedEnvironments"`
		UnsupportedEnvironments int `json:"unsupportedEnvironments"`
		Conflicts               []struct {
			EnvironmentGroupID      int    `json:"environmentGroupId"`
			EnvironmentGroupName    string `json:"environmentGroupName"`
			EnvironmentCount        int    `json:"environmentCount"`
			SupportedEnvironments   int    `json:"supportedEnvironments"`
			UnsupportedEnvironments int    `json:"unsupportedEnvironments"`
			ExistingPolicyID        int    `json:"existingPolicyId"`
			ExistingPolicyName      string `json:"existingPolicyName"`
		} `json:"conflicts"`
		NewGroups []struct {
			EnvironmentGroupID      int    `json:"environmentGroupId"`
			EnvironmentGroupName    string `json:"environmentGroupName"`
			EnvironmentCount        int    `json:"environmentCount"`
			SupportedEnvironments   int    `json:"supportedEnvironments"`
			UnsupportedEnvironments int    `json:"unsupportedEnvironments"`
		} `json:"newGroups"`
	}
	if err := doJSON(ctx, d.client, http.MethodPost, d.client.Endpoint+"/policies/conflicts", payload, &response); err != nil {
		resp.Diagnostics.AddError("Failed to preview the policy conflicts", fmt.Errorf("failed to preview the policy conflicts: %w", err).Error())
		return
	}

	conflicts := make([]attr.Value, 0, len(response.Conflicts))
	for _, c := range response.Conflicts {
		conflicts = append(conflicts, types.ObjectValueMust(policyConflictObjectType.AttrTypes, map[string]attr.Value{
			"environment_group_id":     types.Int64Value(int64(c.EnvironmentGroupID)),
			"environment_group_name":   types.StringValue(c.EnvironmentGroupName),
			"environment_count":        types.Int64Value(int64(c.EnvironmentCount)),
			"supported_environments":   types.Int64Value(int64(c.SupportedEnvironments)),
			"unsupported_environments": types.Int64Value(int64(c.UnsupportedEnvironments)),
			"existing_policy_id":       types.Int64Value(int64(c.ExistingPolicyID)),
			"existing_policy_name":     types.StringValue(c.ExistingPolicyName),
		}))
	}
	newGroups := make([]attr.Value, 0, len(response.NewGroups))
	for _, g := range response.NewGroups {
		newGroups = append(newGroups, types.ObjectValueMust(policyNewGroupObjectType.AttrTypes, map[string]attr.Value{
			"environment_group_id":     types.Int64Value(int64(g.EnvironmentGroupID)),
			"environment_group_name":   types.StringValue(g.EnvironmentGroupName),
			"environment_count":        types.Int64Value(int64(g.EnvironmentCount)),
			"supported_environments":   types.Int64Value(int64(g.SupportedEnvironments)),
			"unsupported_environments": types.Int64Value(int64(g.UnsupportedEnvironments)),
		}))
	}

	m.ID = types.StringValue("portainer-policy-conflicts")
	m.TotalEnvironments = types.Int64Value(int64(response.TotalEnvironments))
	m.SupportedEnvironments = types.Int64Value(int64(response.SupportedEnvironments))
	m.UnsupportedEnvironments = types.Int64Value(int64(response.UnsupportedEnvironments))
	m.Conflicts = types.ListValueMust(policyConflictObjectType, conflicts)
	m.NewGroups = types.ListValueMust(policyNewGroupObjectType, newGroups)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// ---------------------------------------------------------------------------
// portainer_policy_observability_test

func newPolicyObservabilityTestDataSource() datasource.DataSource {
	return &policyObservabilityTestDataSource{}
}

type policyObservabilityTestDataSource struct{ client *APIClient }

type policyObservabilityTestDataSourceModel struct {
	ID                        types.String `tfsdk:"id"`
	OneUptimeURL              types.String `tfsdk:"one_uptime_url"`
	APIKey                    types.String `tfsdk:"api_key"`
	ProjectID                 types.String `tfsdk:"project_id"`
	PolicyID                  types.Int64  `tfsdk:"policy_id"`
	TLSSkipVerify             types.Bool   `tfsdk:"tls_skip_verify"`
	AutoProvisionIngestionKey types.Bool   `tfsdk:"auto_provision_ingestion_key"`
	AutoRotateKey             types.Bool   `tfsdk:"auto_rotate_key"`
	FailOnError               types.Bool   `tfsdk:"fail_on_error"`
	Success                   types.Bool   `tfsdk:"success"`
	Message                   types.String `tfsdk:"message"`
}

func (d *policyObservabilityTestDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_policy_observability_test"
}

// Data source schemas have no defaults: the SDK defaults (false, and true for
// fail_on_error) are applied in Read to an argument left out.
func (d *policyObservabilityTestDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id":             schema.StringAttribute{Description: "Identifier of the data source.", Computed: true},
			"one_uptime_url": schema.StringAttribute{Required: true, Description: "URL of the OneUptime instance to test against."},
			"api_key":        schema.StringAttribute{Optional: true, Sensitive: true, Description: "OneUptime API key to authenticate with."},
			"project_id":     schema.StringAttribute{Optional: true, Description: "OneUptime project the policy would report into."},
			"policy_id":      schema.Int64Attribute{Optional: true, Description: "Existing policy to test the connection for, when there is one."},
			"tls_skip_verify": schema.BoolAttribute{
				Optional:    true,
				Description: "Whether to skip verification of the OneUptime server's TLS certificate.",
			},
			"auto_provision_ingestion_key": schema.BoolAttribute{
				Optional:    true,
				Description: "Whether the test should provision an ingestion key as part of the check.",
			},
			"auto_rotate_key": schema.BoolAttribute{
				Optional:    true,
				Description: "Whether the test should rotate the key as part of the check.",
			},
			"fail_on_error": schema.BoolAttribute{
				Optional:    true,
				Description: "Whether a failed connection fails the plan. Defaults to `true`, which is the point of a pre-flight check; set it to `false` to branch on `success` instead.",
			},
			"success": schema.BoolAttribute{Computed: true, Description: "Whether Portainer could reach OneUptime with these details."},
			"message": schema.StringAttribute{Computed: true, Description: "What Portainer reported about the attempt."},
		},
	}
}

func (d *policyObservabilityTestDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// policyBoolOr is a bool argument, or its default when left out.
func policyBoolOr(v types.Bool, def bool) bool {
	if v.IsNull() || v.IsUnknown() {
		return def
	}
	return v.ValueBool()
}

func (d *policyObservabilityTestDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m policyObservabilityTestDataSourceModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}

	payload := map[string]interface{}{
		"oneUptimeURL":              m.OneUptimeURL.ValueString(),
		"tlsSkipVerify":             policyBoolOr(m.TLSSkipVerify, false),
		"autoProvisionIngestionKey": policyBoolOr(m.AutoProvisionIngestionKey, false),
		"autoRotateKey":             policyBoolOr(m.AutoRotateKey, false),
	}
	// The credentials are omitted when unset so the test never sends a blank
	// key that OneUptime would reject for the wrong reason.
	if v := m.APIKey.ValueString(); v != "" {
		payload["apiKey"] = v
	}
	if v := m.ProjectID.ValueString(); v != "" {
		payload["projectID"] = v
	}
	if v := m.PolicyID.ValueInt64(); v != 0 {
		payload["policyId"] = int(v)
	}

	var response struct {
		Success bool   `json:"success"`
		Message string `json:"message"`
	}
	testURL := d.client.Endpoint + "/policies/observability-k8s/test"
	if err := doJSON(ctx, d.client, http.MethodPost, testURL, payload, &response); err != nil {
		resp.Diagnostics.AddError("Failed to test the OneUptime connection", fmt.Errorf("failed to test the OneUptime connection: %w", err).Error())
		return
	}

	if !response.Success && policyBoolOr(m.FailOnError, true) {
		resp.Diagnostics.AddError("OneUptime connection test failed",
			fmt.Sprintf("the OneUptime connection test failed: %s (set fail_on_error = false to read the result instead)", response.Message))
		return
	}

	m.ID = types.StringValue("portainer-policy-observability-test")
	m.Success = types.BoolValue(response.Success)
	m.Message = types.StringValue(response.Message)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}
