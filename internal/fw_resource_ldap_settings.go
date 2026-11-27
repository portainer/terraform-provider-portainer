package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework-validators/stringvalidator"
	"github.com/hashicorp/terraform-plugin-framework/diag"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/planmodifier"
	"github.com/hashicorp/terraform-plugin-framework/resource/schema/stringplanmodifier"
	"github.com/hashicorp/terraform-plugin-framework/schema/validator"
	"github.com/hashicorp/terraform-plugin-framework/tfsdk"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

var (
	_ resource.Resource                   = (*ldapSettingsResource)(nil)
	_ resource.ResourceWithConfigure      = (*ldapSettingsResource)(nil)
	_ resource.ResourceWithImportState    = (*ldapSettingsResource)(nil)
	_ resource.ResourceWithValidateConfig = (*ldapSettingsResource)(nil)
	_ resource.ResourceWithModifyPlan     = (*ldapSettingsResource)(nil)
)

const ldapSettingsID = "portainer-ldap-settings"

func newLDAPSettingsResource() resource.Resource { return &ldapSettingsResource{} }

type ldapSettingsResource struct{ client *APIClient }

type ldapSettingsModel struct {
	ID                       types.String              `tfsdk:"id"`
	AnonymousMode            types.Bool                `tfsdk:"anonymous_mode"`
	AutoCreateUsers          types.Bool                `tfsdk:"auto_create_users"`
	Password                 types.String              `tfsdk:"password"`
	PasswordWO               types.String              `tfsdk:"password_wo"`
	PasswordWOVersion        types.Int64               `tfsdk:"password_wo_version"`
	ReaderDN                 types.String              `tfsdk:"reader_dn"`
	StartTLS                 types.Bool                `tfsdk:"start_tls"`
	URL                      types.String              `tfsdk:"url"`
	URLs                     types.List                `tfsdk:"urls"`
	ServerType               types.Int64               `tfsdk:"server_type"`
	AdminAutoPopulate        types.Bool                `tfsdk:"admin_auto_populate"`
	AdminGroups              types.List                `tfsdk:"admin_groups"`
	SearchSettings           []settingsLDAPSearchModel `tfsdk:"search_settings"`
	GroupSearchSettings      []settingsLDAPGroupModel  `tfsdk:"group_search_settings"`
	AdminGroupSearchSettings []settingsLDAPGroupModel  `tfsdk:"admin_group_search_settings"`
	TLSConfig                []settingsLDAPTLSModel    `tfsdk:"tls_config"`
	TLSKeyWO                 types.String              `tfsdk:"tls_key_wo"`
	TLSKeyWOVersion          types.Int64               `tfsdk:"tls_key_wo_version"`
}

func (r *ldapSettingsResource) Metadata(_ context.Context, req resource.MetadataRequest, resp *resource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ldap_settings"
}

func (r *ldapSettingsResource) Schema(_ context.Context, _ resource.SchemaRequest, resp *resource.SchemaResponse) {
	password := ocString("Password of the account used to search users. Stored in state; use `password_wo` to keep it out.")
	password.Sensitive = true
	tlsKey := ocString("TLS key. Stored in state as a sensitive value; use the top-level `tls_key_wo` to keep it out of state.")
	tlsKey.Sensitive = true
	groupSearch := func(desc, attrDesc, baseDesc, filterDesc string) schema.ListNestedBlock {
		return schema.ListNestedBlock{
			Description: desc,
			NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{
				"group_attribute": ocString(attrDesc),
				"group_base_dn":   ocString(baseDesc),
				"group_filter":    ocString(filterDesc),
			}},
		}
	}

	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{
			"id": schema.StringAttribute{
				Description:   "Identifier of the resource.",
				Computed:      true,
				PlanModifiers: []planmodifier.String{stringplanmodifier.UseStateForUnknown()},
			},
			"anonymous_mode":      ocBool("Enable anonymous mode. When enabled, ReaderDN and Password will not be used"),
			"auto_create_users":   ocBool("Automatically provision users and assign them to matching LDAP group names"),
			"password":            password,
			"password_wo":         gsbWriteOnlySecretAttribute("password", "password_wo_version", "password of the account used to search users"),
			"password_wo_version": gsbWriteOnlyVersionAttribute("password_wo", false),
			"reader_dn":           ocString("Account that will be used to search for users (e.g. cn=readonly-account,dc=ldap,dc=domain,dc=tld)"),
			"start_tls":           ocBool("Whether LDAP connection should use StartTLS"),
			"url":                 ocString("URL or IP address of the LDAP server (deprecated, use urls)"),
			"urls":                ocList(types.StringType, "URLs or IP addresses of the LDAP server"),
			"server_type":         ocInt("LDAP server type"),
			"admin_auto_populate": ocBool("Whether auto admin population is enabled"),
			"admin_groups":        ocList(types.StringType, "Saved admin group list"),
			// Top level, because tls_config is computed and write-only
			// attributes cannot sit inside a computed block.
			"tls_key_wo": schema.StringAttribute{
				Optional:  true,
				Sensitive: true,
				WriteOnly: true,
				Validators: []validator.String{
					stringvalidator.ConflictsWith(path.MatchRoot("tls_config").AtAnyListIndex().AtName("tls_key")),
					stringvalidator.AlsoRequires(path.MatchRoot("tls_key_wo_version")),
				},
				Description: "Write-only TLS key for `tls_config` (supports ephemeral values; never stored in Terraform state). Conflicts with `tls_config.tls_key`. Requires Terraform 1.11 or later.",
			},
			"tls_key_wo_version": gsbWriteOnlyVersionAttribute("tls_key_wo", false),
		},
		// SDKv2 declared these blocks optional and computed; see
		// ldapSettingsKeepBlock for how Read handles that.
		Blocks: map[string]schema.Block{
			"search_settings": schema.ListNestedBlock{
				Description: "LDAP user search settings",
				NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{
					"base_dn":             ocString("Base DN for user search"),
					"filter":              ocString("LDAP search filter"),
					"user_name_attribute": ocString("Attribute used for username"),
				}},
			},
			"group_search_settings": groupSearch("LDAP group search settings", "LDAP group attribute", "Base DN for group search", "LDAP group search filter"),
			"admin_group_search_settings": groupSearch("LDAP admin group search settings",
				"LDAP attribute used to identify group membership for admin users (e.g. `member`).",
				"Base distinguished name under which to search for admin groups.",
				"LDAP search filter used to match admin groups (e.g. `(objectClass=groupOfNames)`)."),
			"tls_config": schema.ListNestedBlock{
				Description: "TLS configuration for LDAP",
				Validators:  maxOne(),
				NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{
					"tls":             ocBool("Whether TLS is enabled"),
					"tls_ca_cert":     ocString("TLS CA certificate"),
					"tls_cert":        ocString("TLS certificate"),
					"tls_key":         tlsKey,
					"tls_skip_verify": ocBool("Skip TLS verification"),
				}},
			},
		},
	}
}

func (r *ldapSettingsResource) Configure(_ context.Context, req resource.ConfigureRequest, resp *resource.ConfigureResponse) {
	r.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

// ValidateConfig requires tls_config with tls_key_wo, as SDKv2's RequiredWith
// did.
func (r *ldapSettingsResource) ValidateConfig(ctx context.Context, req resource.ValidateConfigRequest, resp *resource.ValidateConfigResponse) {
	var wo types.String
	var block types.List
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("tls_key_wo"), &wo)...)
	resp.Diagnostics.Append(req.Config.GetAttribute(ctx, path.Root("tls_config"), &block)...)
	if wo.IsNull() || block.IsUnknown() {
		return
	}
	if block.IsNull() || len(block.Elements()) == 0 {
		resp.Diagnostics.AddAttributeError(path.Root("tls_key_wo"), "Missing required argument",
			"\"tls_key_wo\": all of `tls_config,tls_key_wo,tls_key_wo_version` must be specified")
	}
}

// ModifyPlan plans the removal of a password or TLS key stored in state once
// its write-only twin is in use, which SDKv2 did during apply.
func (r *ldapSettingsResource) ModifyPlan(ctx context.Context, req resource.ModifyPlanRequest, resp *resource.ModifyPlanResponse) {
	if req.Plan.Raw.IsNull() {
		return
	}
	var plan, config ldapSettingsModel
	resp.Diagnostics.Append(req.Plan.Get(ctx, &plan)...)
	resp.Diagnostics.Append(req.Config.Get(ctx, &config)...)
	if resp.Diagnostics.HasError() {
		return
	}
	if config.Password.IsNull() && plan.PasswordWOVersion.ValueInt64() != 0 && !config.PasswordWO.IsNull() && config.PasswordWO.ValueString() != "" &&
		(plan.Password.IsUnknown() || plan.Password.ValueString() != "") {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("password"), "")...)
	}
	if len(plan.TLSConfig) > 0 && len(config.TLSConfig) > 0 && config.TLSConfig[0].TLSKey.IsNull() &&
		plan.TLSKeyWOVersion.ValueInt64() != 0 && config.TLSKeyWO.ValueString() != "" &&
		(plan.TLSConfig[0].TLSKey.IsUnknown() || plan.TLSConfig[0].TLSKey.ValueString() != "") {
		resp.Diagnostics.Append(resp.Plan.SetAttribute(ctx, path.Root("tls_config").AtListIndex(0).AtName("tls_key"), "")...)
	}
}

func (r *ldapSettingsResource) payload(ctx context.Context, config tfsdk.Config, m *ldapSettingsModel, diags *diag.Diagnostics) map[string]interface{} {
	// Like SDKv2's GetOk, a zero value is left out.
	ldap := map[string]interface{}{}
	if m.AnonymousMode.ValueBool() {
		ldap["AnonymousMode"] = true
	}
	if m.AutoCreateUsers.ValueBool() {
		ldap["AutoCreateUsers"] = true
	}
	if password, _ := gsbTwinSecret(ctx, config, m.Password, path.Root("password_wo"), m.PasswordWOVersion, diags); password != "" {
		ldap["Password"] = password
	}
	if v := m.ReaderDN.ValueString(); v != "" {
		ldap["ReaderDN"] = v
	}
	if m.StartTLS.ValueBool() {
		ldap["StartTLS"] = true
	}
	if v := m.URL.ValueString(); v != "" {
		ldap["URL"] = v
	}
	if urls := stringsFromList(ctx, m.URLs, diags); len(urls) > 0 {
		ldap["URLs"] = urls
	}
	if v := m.ServerType.ValueInt64(); v != 0 {
		ldap["ServerType"] = int(v)
	}
	if m.AdminAutoPopulate.ValueBool() {
		ldap["AdminAutoPopulate"] = true
	}
	if groups := stringsFromList(ctx, m.AdminGroups, diags); len(groups) > 0 {
		ldap["AdminGroups"] = groups
	}
	if len(m.SearchSettings) > 0 {
		settings := make([]map[string]interface{}, 0, len(m.SearchSettings))
		for _, s := range m.SearchSettings {
			settings = append(settings, map[string]interface{}{
				"BaseDN":            s.BaseDN.ValueString(),
				"Filter":            s.Filter.ValueString(),
				"UserNameAttribute": s.UserNameAttribute.ValueString(),
			})
		}
		ldap["SearchSettings"] = settings
	}
	groupSettings := func(blocks []settingsLDAPGroupModel) []map[string]interface{} {
		settings := make([]map[string]interface{}, 0, len(blocks))
		for _, s := range blocks {
			settings = append(settings, map[string]interface{}{
				"GroupAttribute": s.GroupAttribute.ValueString(),
				"GroupBaseDN":    s.GroupBaseDN.ValueString(),
				"GroupFilter":    s.GroupFilter.ValueString(),
			})
		}
		return settings
	}
	if len(m.GroupSearchSettings) > 0 {
		ldap["GroupSearchSettings"] = groupSettings(m.GroupSearchSettings)
	}
	if len(m.AdminGroupSearchSettings) > 0 {
		ldap["AdminGroupSearchSettings"] = groupSettings(m.AdminGroupSearchSettings)
	}
	if len(m.TLSConfig) > 0 {
		t := m.TLSConfig[0]
		tlsKey := t.TLSKey.ValueString()
		if m.TLSKeyWOVersion.ValueInt64() != 0 {
			if wo := writeOnlyConfigString(ctx, config, path.Root("tls_key_wo"), diags); wo != "" {
				tlsKey = wo
			}
		}
		ldap["TLSConfig"] = map[string]interface{}{
			"TLS":           t.TLS.ValueBool(),
			"TLSCACert":     t.TLSCACert.ValueString(),
			"TLSCert":       t.TLSCert.ValueString(),
			"TLSKey":        tlsKey,
			"TLSSkipVerify": t.TLSSkipVerify.ValueBool(),
		}
	}
	return ldap
}

// ldapSettingsAPI is GET /settings' LDAPSettings, kept raw: a field Portainer
// leaves out keeps its state value.
type ldapSettingsAPI map[string]json.RawMessage

func (a ldapSettingsAPI) get(key string, v interface{}) bool {
	raw, ok := a[key]
	if !ok {
		return false
	}
	return json.Unmarshal(raw, v) == nil
}

func (r *ldapSettingsResource) fetch(ctx context.Context) (ldapSettingsAPI, error) {
	var result map[string]json.RawMessage
	if err := doJSON(ctx, r.client, http.MethodGet, r.client.Endpoint+"/settings", nil, &result); err != nil {
		return nil, fmt.Errorf("failed to read settings: %w", err)
	}
	raw, ok := result["LDAPSettings"]
	if !ok || string(raw) == "null" {
		return nil, nil
	}
	var ldap ldapSettingsAPI
	if err := json.Unmarshal(raw, &ldap); err != nil {
		return nil, fmt.Errorf("failed to unmarshal LDAP settings: %w", err)
	}
	return ldap, nil
}

// ldapSettingsKeepBlock tells whether Read reads a block back. Terraform
// plans as many blocks as the configuration has, so a block read back that
// the configuration leaves out would plan its removal on every run: a state
// the framework wrote keeps the blocks it has. A state v3.0.0 wrote (or an
// import) holds every block Portainer reported; there a block counts as
// configured when Portainer reports a non-empty value in it, since its
// defaults for these blocks are empty.
func ldapSettingsKeepBlock(fw bool, prior int, nonEmpty bool) bool {
	if fw {
		return prior > 0
	}
	return nonEmpty
}

// fromAPI updates m with what Portainer reports.
func ldapSettingsFromAPI(ldap ldapSettingsAPI, m *ldapSettingsModel, fw, all bool) {
	var b bool
	var s string
	var n float64
	var list []string
	if ldap.get("AnonymousMode", &b) {
		m.AnonymousMode = types.BoolValue(b)
	}
	if ldap.get("AutoCreateUsers", &b) {
		m.AutoCreateUsers = types.BoolValue(b)
	}
	if ldap.get("ReaderDN", &s) {
		m.ReaderDN = types.StringValue(s)
	}
	if ldap.get("StartTLS", &b) {
		m.StartTLS = types.BoolValue(b)
	}
	if ldap.get("URL", &s) {
		m.URL = types.StringValue(s)
	}
	if list = nil; ldap.get("URLs", &list) {
		m.URLs = listOfStrings(list)
	}
	if ldap.get("ServerType", &n) {
		m.ServerType = types.Int64Value(int64(n))
	}
	if ldap.get("AdminAutoPopulate", &b) {
		m.AdminAutoPopulate = types.BoolValue(b)
	}
	if list = nil; ldap.get("AdminGroups", &list) {
		m.AdminGroups = listOfStrings(list)
	}
	// The password is never read back: the state keeps the configured one.
	m.Password = types.StringValue(m.Password.ValueString())

	var search []SearchSetting
	if ldap.get("SearchSettings", &search) && search != nil {
		nonEmpty := false
		for _, e := range search {
			nonEmpty = nonEmpty || e != (SearchSetting{})
		}
		if all || ldapSettingsKeepBlock(fw, len(m.SearchSettings), nonEmpty) {
			m.SearchSettings = []settingsLDAPSearchModel{}
			for _, e := range search {
				m.SearchSettings = append(m.SearchSettings, settingsLDAPSearchModel{
					BaseDN: types.StringValue(e.BaseDN), Filter: types.StringValue(e.Filter), UserNameAttribute: types.StringValue(e.UserNameAttribute),
				})
			}
		} else {
			m.SearchSettings = []settingsLDAPSearchModel{}
		}
	} else if !fw && !all {
		m.SearchSettings = []settingsLDAPSearchModel{}
	}

	groups := func(key string, current []settingsLDAPGroupModel) []settingsLDAPGroupModel {
		var entries []GroupSearch
		if !ldap.get(key, &entries) || entries == nil {
			if !fw && !all {
				return []settingsLDAPGroupModel{}
			}
			return current
		}
		nonEmpty := false
		for _, e := range entries {
			nonEmpty = nonEmpty || e != (GroupSearch{})
		}
		out := []settingsLDAPGroupModel{}
		if all || ldapSettingsKeepBlock(fw, len(current), nonEmpty) {
			for _, e := range entries {
				out = append(out, settingsLDAPGroupModel{
					GroupAttribute: types.StringValue(e.GroupAttribute), GroupBaseDN: types.StringValue(e.GroupBaseDN), GroupFilter: types.StringValue(e.GroupFilter),
				})
			}
		}
		return out
	}
	m.GroupSearchSettings = groups("GroupSearchSettings", m.GroupSearchSettings)
	m.AdminGroupSearchSettings = groups("AdminGroupSearchSettings", m.AdminGroupSearchSettings)

	var tlsConfig *TLSConfig
	if ldap.get("TLSConfig", &tlsConfig) && tlsConfig != nil {
		// The TLS key keeps the value from state.
		var key string
		if len(m.TLSConfig) > 0 {
			key = m.TLSConfig[0].TLSKey.ValueString()
		}
		nonEmpty := tlsConfig.TLS || tlsConfig.TLSSkipVerify || tlsConfig.TLSCACert != "" || tlsConfig.TLSCert != ""
		if all || ldapSettingsKeepBlock(fw, len(m.TLSConfig), nonEmpty) {
			m.TLSConfig = []settingsLDAPTLSModel{{
				TLS:           types.BoolValue(tlsConfig.TLS),
				TLSCACert:     types.StringValue(tlsConfig.TLSCACert),
				TLSCert:       types.StringValue(tlsConfig.TLSCert),
				TLSKey:        types.StringValue(key),
				TLSSkipVerify: types.BoolValue(tlsConfig.TLSSkipVerify),
			}}
		} else {
			m.TLSConfig = []settingsLDAPTLSModel{}
		}
	} else if !fw && !all {
		m.TLSConfig = []settingsLDAPTLSModel{}
	}

	for _, l := range []*[]settingsLDAPSearchModel{&m.SearchSettings} {
		if *l == nil {
			*l = []settingsLDAPSearchModel{}
		}
	}
	for _, l := range []*[]settingsLDAPGroupModel{&m.GroupSearchSettings, &m.AdminGroupSearchSettings} {
		if *l == nil {
			*l = []settingsLDAPGroupModel{}
		}
	}
	if m.TLSConfig == nil {
		m.TLSConfig = []settingsLDAPTLSModel{}
	}
	// A field Portainer does not report holds the empty value SDKv2 stored.
	for _, v := range []*types.Bool{&m.AnonymousMode, &m.AutoCreateUsers, &m.StartTLS, &m.AdminAutoPopulate} {
		if v.IsNull() {
			*v = types.BoolValue(false)
		}
	}
	for _, v := range []*types.String{&m.ReaderDN, &m.URL} {
		if v.IsNull() {
			*v = types.StringValue("")
		}
	}
	if m.ServerType.IsNull() {
		m.ServerType = types.Int64Value(0)
	}
	for _, v := range []*types.List{&m.URLs, &m.AdminGroups} {
		if v.IsNull() {
			*v = listOfStrings(nil)
		}
	}
	m.PasswordWO = types.StringNull()
	m.TLSKeyWO = types.StringNull()
	m.ID = types.StringValue(ldapSettingsID)
}

func (r *ldapSettingsResource) apply(ctx context.Context, config tfsdk.Config, plan tfsdk.Plan, state *tfsdk.State, diags *diag.Diagnostics) {
	var m ldapSettingsModel
	diags.Append(plan.Get(ctx, &m)...)
	if diags.HasError() {
		return
	}

	// The current settings are read first, as before; a failure stops here.
	var currentSettings map[string]interface{}
	if err := doJSON(ctx, r.client, http.MethodGet, r.client.Endpoint+"/settings", nil, &currentSettings); err != nil {
		diags.AddError("Failed to read current settings", fmt.Sprintf("failed to read current settings: %s", err))
		return
	}

	ldap := r.payload(ctx, config, &m, diags)
	if diags.HasError() {
		return
	}
	// Set authentication method to LDAP (2) and update LDAP settings
	payload := map[string]interface{}{
		"authenticationMethod": 2,
		"ldapsettings":         ldap,
	}
	if err := doJSON(ctx, r.client, http.MethodPut, r.client.Endpoint+"/settings", payload, nil); err != nil {
		diags.AddError("Failed to update LDAP settings", fmt.Sprintf("failed to update LDAP settings: %s", err))
		return
	}

	// The configured values are what was just written; values the plan
	// leaves unknown come from Portainer.
	current, err := r.fetch(ctx)
	if err != nil {
		diags.AddError("Failed to read settings", err.Error())
		return
	}
	readModel := m
	ldapSettingsFromAPI(current, &readModel, true, true)
	read := tfsdk.State{Schema: state.Schema, Raw: tftypes.NewValue(state.Schema.Type().TerraformType(ctx), nil)}
	diags.Append(read.Set(ctx, &readModel)...)
	if diags.HasError() {
		return
	}
	filled, err := gsbFillUnknown(plan.Raw, read.Raw)
	if err != nil {
		diags.AddError("Failed to update LDAP settings", err.Error())
		return
	}
	state.Raw = filled
	diags.Append(state.SetAttribute(ctx, path.Root("id"), ldapSettingsID)...)
}

func (r *ldapSettingsResource) Create(ctx context.Context, req resource.CreateRequest, resp *resource.CreateResponse) {
	r.apply(ctx, req.Config, req.Plan, &resp.State, &resp.Diagnostics)
	gsbMarkFrameworkState(ctx, resp.Private, &resp.Diagnostics)
}

func (r *ldapSettingsResource) Update(ctx context.Context, req resource.UpdateRequest, resp *resource.UpdateResponse) {
	r.apply(ctx, req.Config, req.Plan, &resp.State, &resp.Diagnostics)
	gsbMarkFrameworkState(ctx, resp.Private, &resp.Diagnostics)
}

func (r *ldapSettingsResource) Read(ctx context.Context, req resource.ReadRequest, resp *resource.ReadResponse) {
	var m ldapSettingsModel
	resp.Diagnostics.Append(req.State.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	ldap, err := r.fetch(ctx)
	if err != nil {
		resp.Diagnostics.AddError("Failed to read settings", err.Error())
		return
	}
	if ldap == nil {
		resp.State.RemoveResource(ctx)
		return
	}
	ldapSettingsFromAPI(ldap, &m, gsbFrameworkState(ctx, req.Private), false)
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	gsbMarkFrameworkState(ctx, resp.Private, &resp.Diagnostics)
}

// Delete only forgets the settings.
func (r *ldapSettingsResource) Delete(_ context.Context, _ resource.DeleteRequest, _ *resource.DeleteResponse) {
}

func (r *ldapSettingsResource) ImportState(ctx context.Context, req resource.ImportStateRequest, resp *resource.ImportStateResponse) {
	resource.ImportStatePassthroughID(ctx, path.Root("id"), req, resp)
}
