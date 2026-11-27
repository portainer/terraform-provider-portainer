package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

// The LDAP browse endpoints all take a full settings object describing how to
// reach and search the directory, rather than reusing the instance's own
// configuration. That is what lets them be used to try a configuration out
// before it is saved - and why the connection arguments repeat on each of
// these data sources (and on portainer_ldap_check).

var (
	_ datasource.DataSource              = (*ldapCheckDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*ldapCheckDataSource)(nil)
	_ datasource.DataSource              = (*ldapBrowseDataSource)(nil)
	_ datasource.DataSourceWithConfigure = (*ldapBrowseDataSource)(nil)
)

// ldapConnectionModel holds the connection arguments every LDAP data source
// takes.
type ldapConnectionModel struct {
	URL           types.String `tfsdk:"url"`
	ReaderDN      types.String `tfsdk:"reader_dn"`
	Password      types.String `tfsdk:"password"`
	AnonymousMode types.Bool   `tfsdk:"anonymous_mode"`
	StartTLS      types.Bool   `tfsdk:"start_tls"`
	TLSSkipVerify types.Bool   `tfsdk:"tls_skip_verify"`
}

// ldapConnectionAttributes is the schema of ldapConnectionModel. Data sources
// take no defaults, so the false defaults are applied in Read.
func ldapConnectionAttributes() map[string]schema.Attribute {
	return map[string]schema.Attribute{
		"id":              schema.StringAttribute{Description: "Identifier of the resource.", Computed: true},
		"url":             schema.StringAttribute{Required: true, Description: "URL of the LDAP server, as `host:port` (for example `ldap.example.com:389`)."},
		"reader_dn":       schema.StringAttribute{Optional: true, Description: "Distinguished name of the account used to browse the directory. Leave unset when `anonymous_mode` is enabled."},
		"password":        schema.StringAttribute{Optional: true, Sensitive: true, Description: "Password of the reader account. Stored in state as a sensitive value."},
		"anonymous_mode":  schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether to bind anonymously instead of using `reader_dn` and `password`."},
		"start_tls":       schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether to upgrade the connection with StartTLS."},
		"tls_skip_verify": schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether to skip verification of the LDAP server's TLS certificate."},
	}
}

// defaults applies the false defaults SDKv2 declared.
func (c *ldapConnectionModel) defaults() {
	for _, b := range []*types.Bool{&c.AnonymousMode, &c.StartTLS, &c.TLSSkipVerify} {
		if b.IsNull() || b.IsUnknown() {
			*b = types.BoolValue(false)
		}
	}
}

// settings builds the LDAPSettings object every one of these endpoints takes.
func (c *ldapConnectionModel) settings() map[string]interface{} {
	settings := map[string]interface{}{
		"URL":           c.URL.ValueString(),
		"AnonymousMode": c.AnonymousMode.ValueBool(),
		"StartTLS":      c.StartTLS.ValueBool(),
		"TLSConfig": map[string]interface{}{
			"TLSSkipVerify": c.TLSSkipVerify.ValueBool(),
		},
	}
	if v := c.ReaderDN.ValueString(); v != "" {
		settings["ReaderDN"] = v
	}
	if v := c.Password.ValueString(); v != "" {
		settings["Password"] = v
	}
	return settings
}

type ldapSearchModel struct {
	BaseDN            types.String `tfsdk:"base_dn"`
	Filter            types.String `tfsdk:"filter"`
	UserNameAttribute types.String `tfsdk:"user_name_attribute"`
}

type ldapGroupSearchModel struct {
	GroupBaseDN    types.String `tfsdk:"group_base_dn"`
	GroupFilter    types.String `tfsdk:"group_filter"`
	GroupAttribute types.String `tfsdk:"group_attribute"`
}

func ldapUserSearchBlock() schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: "Where and how to look for users. Repeatable, and searched in order.",
		NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{
			"base_dn":             schema.StringAttribute{Required: true, Description: "Distinguished name the search starts from."},
			"filter":              schema.StringAttribute{Optional: true, Description: "LDAP filter narrowing the search, for example `(objectClass=person)`."},
			"user_name_attribute": schema.StringAttribute{Optional: true, Description: "Attribute holding the login name, for example `uid` or `sAMAccountName`."},
		}},
	}
}

func ldapGroupSearchBlock(description string) schema.ListNestedBlock {
	return schema.ListNestedBlock{
		Description: description,
		NestedObject: schema.NestedBlockObject{Attributes: map[string]schema.Attribute{
			"group_base_dn":   schema.StringAttribute{Required: true, Description: "Distinguished name the group search starts from."},
			"group_filter":    schema.StringAttribute{Optional: true, Description: "LDAP filter narrowing the group search."},
			"group_attribute": schema.StringAttribute{Optional: true, Description: "Attribute on a group entry listing its members."},
		}},
	}
}

// ldapSearchSettings encodes the user search blocks.
func ldapSearchSettings(blocks []ldapSearchModel) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(blocks))
	for _, b := range blocks {
		entry := map[string]interface{}{"BaseDN": b.BaseDN.ValueString()}
		if v := b.Filter.ValueString(); v != "" {
			entry["Filter"] = v
		}
		if v := b.UserNameAttribute.ValueString(); v != "" {
			entry["UserNameAttribute"] = v
		}
		out = append(out, entry)
	}
	return out
}

// ldapGroupSearchSettings encodes the group search blocks, which both the
// group listing and the admin group listing take.
func ldapGroupSearchSettings(blocks []ldapGroupSearchModel) []map[string]interface{} {
	out := make([]map[string]interface{}, 0, len(blocks))
	for _, b := range blocks {
		entry := map[string]interface{}{"GroupBaseDN": b.GroupBaseDN.ValueString()}
		if v := b.GroupFilter.ValueString(); v != "" {
			entry["GroupFilter"] = v
		}
		if v := b.GroupAttribute.ValueString(); v != "" {
			entry["GroupAttribute"] = v
		}
		out = append(out, entry)
	}
	return out
}

var ldapEntryType = types.ObjectType{AttrTypes: map[string]attr.Type{
	"name":   types.StringType,
	"groups": types.ListType{ElemType: types.StringType},
}}

// ldapEntries flattens the directory entries the user and group searches
// return; both endpoints answer with the same shape.
func ldapEntries(entries []ldapDirectoryEntry) types.List {
	elems := make([]attr.Value, 0, len(entries))
	for _, e := range entries {
		groups := make([]attr.Value, 0, len(e.Groups))
		for _, g := range e.Groups {
			groups = append(groups, types.StringValue(g))
		}
		elems = append(elems, types.ObjectValueMust(ldapEntryType.AttrTypes, map[string]attr.Value{
			"name":   types.StringValue(e.Name),
			"groups": types.ListValueMust(types.StringType, groups),
		}))
	}
	return types.ListValueMust(ldapEntryType, elems)
}

type ldapDirectoryEntry struct {
	Name   string   `json:"Name"`
	Groups []string `json:"Groups"`
}

// ---- portainer_ldap_check ----

func newLDAPCheckDataSource() datasource.DataSource { return &ldapCheckDataSource{} }

type ldapCheckDataSource struct{ client *APIClient }

type ldapCheckModel struct {
	ID types.String `tfsdk:"id"`
	ldapConnectionModel
	FailOnError types.Bool   `tfsdk:"fail_on_error"`
	Success     types.Bool   `tfsdk:"success"`
	Error       types.String `tfsdk:"error"`
}

func (d *ldapCheckDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ldap_check"
}

func (d *ldapCheckDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := ldapConnectionAttributes()
	attrs["fail_on_error"] = schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether an unreachable directory makes the data source itself fail. Defaults to true, which is the point of a pre-flight check; set it to false to branch on `success` instead."}
	attrs["success"] = schema.BoolAttribute{Computed: true, Description: "Whether Portainer could reach and bind to the directory."}
	attrs["error"] = schema.StringAttribute{Computed: true, Description: "Reason the check failed, empty on success."}
	resp.Schema = schema.Schema{Attributes: attrs}
}

func (d *ldapCheckDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *ldapCheckDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var m ldapCheckModel
	resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
	if resp.Diagnostics.HasError() {
		return
	}
	m.defaults()
	if m.FailOnError.IsNull() {
		m.FailOnError = types.BoolValue(true)
	}

	// The endpoint answers 204 on success and 400/500 with a message otherwise,
	// so the outcome is the status code rather than a body.
	err := doJSON(ctx, d.client, http.MethodPost, d.client.Endpoint+"/ldap/check",
		map[string]interface{}{"LDAPSettings": m.settings()}, nil)
	success := err == nil
	message := ""
	if err != nil {
		var se *apiStatusError
		if !errors.As(err, &se) {
			// A transport-level failure is not a directory answer, so it is
			// raised rather than reported as a failed check.
			resp.Diagnostics.AddError("Failed to check the LDAP connection", fmt.Sprintf("failed to check the LDAP connection: %s", err))
			return
		}
		message = se.Body
	}

	m.Success = types.BoolValue(success)
	m.Error = types.StringValue(message)
	m.ID = types.StringValue("ldap-check-" + m.URL.ValueString())
	if !success && m.FailOnError.ValueBool() {
		resp.Diagnostics.AddError("LDAP connection check failed", fmt.Sprintf("the LDAP connection check failed: %s", message))
		return
	}
	resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
}

// ---- portainer_ldap_users, _groups, _admin_groups, _login_test ----

// ldapBrowseDataSource is one of the four browse data sources, which differ
// in endpoint, search block and result.
type ldapBrowseDataSource struct {
	client *APIClient
	kind   string
}

func newLDAPUsersDataSource() datasource.DataSource { return &ldapBrowseDataSource{kind: "users"} }

func newLDAPGroupsDataSource() datasource.DataSource { return &ldapBrowseDataSource{kind: "groups"} }

func newLDAPAdminGroupsDataSource() datasource.DataSource {
	return &ldapBrowseDataSource{kind: "admin_groups"}
}

func newLDAPLoginTestDataSource() datasource.DataSource {
	return &ldapBrowseDataSource{kind: "login_test"}
}

type ldapUsersModel struct {
	ID types.String `tfsdk:"id"`
	ldapConnectionModel
	Search  []ldapSearchModel `tfsdk:"search"`
	Entries types.List        `tfsdk:"entries"`
}

type ldapGroupsModel struct {
	ID types.String `tfsdk:"id"`
	ldapConnectionModel
	GroupSearch []ldapGroupSearchModel `tfsdk:"group_search"`
	Entries     types.List             `tfsdk:"entries"`
}

type ldapAdminGroupsModel struct {
	ID types.String `tfsdk:"id"`
	ldapConnectionModel
	AdminGroupSearch []ldapGroupSearchModel `tfsdk:"admin_group_search"`
	GroupNames       types.List             `tfsdk:"group_names"`
}

type ldapLoginTestModel struct {
	ID types.String `tfsdk:"id"`
	ldapConnectionModel
	Search       []ldapSearchModel `tfsdk:"search"`
	Username     types.String      `tfsdk:"username"`
	TestPassword types.String      `tfsdk:"test_password"`
	FailOnError  types.Bool        `tfsdk:"fail_on_error"`
	Valid        types.Bool        `tfsdk:"valid"`
}

func (d *ldapBrowseDataSource) Metadata(_ context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_ldap_" + d.kind
}

func (d *ldapBrowseDataSource) Schema(_ context.Context, _ datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	attrs := ldapConnectionAttributes()
	blocks := map[string]schema.Block{}
	switch d.kind {
	case "users":
		blocks["search"] = ldapUserSearchBlock()
		attrs["entries"] = schema.ListAttribute{Computed: true, ElementType: ldapEntryType, Description: "The users the directory returned for the searches above."}
	case "groups":
		blocks["group_search"] = ldapGroupSearchBlock("Where and how to look for groups. Repeatable, and searched in order.")
		attrs["entries"] = schema.ListAttribute{Computed: true, ElementType: ldapEntryType, Description: "The groups the directory returned for the searches above."}
	case "admin_groups":
		blocks["admin_group_search"] = ldapGroupSearchBlock("Where and how to look for the groups whose members become administrators. Repeatable, and searched in order.")
		attrs["group_names"] = schema.ListAttribute{Computed: true, ElementType: types.StringType, Description: "Names of the groups whose members Portainer would make administrators."}
	case "login_test":
		blocks["search"] = ldapUserSearchBlock()
		attrs["username"] = schema.StringAttribute{Required: true, Description: "Login name to try."}
		// Not "password": that argument is already the reader account's
		// password, and conflating the two would be a security trap.
		attrs["test_password"] = schema.StringAttribute{Required: true, Sensitive: true, Description: "Password to try for `username`. This is the password being tested, not the reader account's."}
		attrs["fail_on_error"] = schema.BoolAttribute{Optional: true, Computed: true, Description: "Whether credentials the directory rejects fail the plan. Defaults to `false`, because a rejected login is usually the answer being asked for rather than an error."}
		attrs["valid"] = schema.BoolAttribute{Computed: true, Description: "Whether the directory accepted the credentials."}
	}
	resp.Schema = schema.Schema{Attributes: attrs, Blocks: blocks}
}

func (d *ldapBrowseDataSource) Configure(_ context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	d.client = configuredClient(req.ProviderData, &resp.Diagnostics)
}

func (d *ldapBrowseDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	switch d.kind {
	case "users":
		var m ldapUsersModel
		resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
		if resp.Diagnostics.HasError() {
			return
		}
		m.defaults()
		settings := m.settings()
		if searches := ldapSearchSettings(m.Search); len(searches) > 0 {
			settings["SearchSettings"] = searches
		}
		var users []ldapDirectoryEntry
		if err := doJSON(ctx, d.client, http.MethodPost, d.client.Endpoint+"/ldap/users", map[string]interface{}{"LDAPSettings": settings}, &users); err != nil {
			resp.Diagnostics.AddError("Failed to search the directory for users", fmt.Sprintf("failed to search the directory for users: %s", err))
			return
		}
		m.Entries = ldapEntries(users)
		m.ID = types.StringValue("portainer-ldap-users-" + m.URL.ValueString())
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)

	case "groups":
		var m ldapGroupsModel
		resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
		if resp.Diagnostics.HasError() {
			return
		}
		m.defaults()
		settings := m.settings()
		if searches := ldapGroupSearchSettings(m.GroupSearch); len(searches) > 0 {
			settings["GroupSearchSettings"] = searches
		}
		var groups []ldapDirectoryEntry
		if err := doJSON(ctx, d.client, http.MethodPost, d.client.Endpoint+"/ldap/groups", map[string]interface{}{"LDAPSettings": settings}, &groups); err != nil {
			resp.Diagnostics.AddError("Failed to search the directory for groups", fmt.Sprintf("failed to search the directory for groups: %s", err))
			return
		}
		m.Entries = ldapEntries(groups)
		m.ID = types.StringValue("portainer-ldap-groups-" + m.URL.ValueString())
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)

	case "admin_groups":
		var m ldapAdminGroupsModel
		resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
		if resp.Diagnostics.HasError() {
			return
		}
		m.defaults()
		settings := m.settings()
		if searches := ldapGroupSearchSettings(m.AdminGroupSearch); len(searches) > 0 {
			settings["AdminGroupSearchSettings"] = searches
		}
		// This endpoint answers with a plain list of names rather than the entry
		// objects the user and group searches return.
		var names []string
		if err := doJSON(ctx, d.client, http.MethodPost, d.client.Endpoint+"/ldap/admin-groups", map[string]interface{}{"LDAPSettings": settings}, &names); err != nil {
			resp.Diagnostics.AddError("Failed to fetch the LDAP administrator groups", fmt.Sprintf("failed to fetch the LDAP administrator groups: %s", err))
			return
		}
		elems := make([]attr.Value, 0, len(names))
		for _, n := range names {
			elems = append(elems, types.StringValue(n))
		}
		m.GroupNames = types.ListValueMust(types.StringType, elems)
		m.ID = types.StringValue("portainer-ldap-admin-groups-" + m.URL.ValueString())
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)

	case "login_test":
		var m ldapLoginTestModel
		resp.Diagnostics.Append(req.Config.Get(ctx, &m)...)
		if resp.Diagnostics.HasError() {
			return
		}
		m.defaults()
		if m.FailOnError.IsNull() {
			m.FailOnError = types.BoolValue(false)
		}
		settings := m.settings()
		if searches := ldapSearchSettings(m.Search); len(searches) > 0 {
			settings["SearchSettings"] = searches
		}
		payload := map[string]interface{}{
			"LDAPSettings": settings,
			"Username":     m.Username.ValueString(),
			"Password":     m.TestPassword.ValueString(),
		}
		var response struct {
			Valid bool `json:"valid"`
		}
		if err := doJSON(ctx, d.client, http.MethodPost, d.client.Endpoint+"/ldap/test", payload, &response); err != nil {
			resp.Diagnostics.AddError("Failed to test the LDAP login", fmt.Sprintf("failed to test the LDAP login: %s", err))
			return
		}
		if !response.Valid && m.FailOnError.ValueBool() {
			resp.Diagnostics.AddError("LDAP login rejected", fmt.Sprintf("the directory rejected the credentials of %s (set fail_on_error = false to read the result instead)", m.Username.ValueString()))
			return
		}
		m.Valid = types.BoolValue(response.Valid)
		m.ID = types.StringValue("portainer-ldap-login-test-" + m.Username.ValueString())
		resp.Diagnostics.Append(resp.State.Set(ctx, &m)...)
	}
}
