package internal

import (
	"context"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// The LDAP browse endpoints all take a full settings object describing how to
// reach and search the directory, rather than reusing the instance's own
// configuration. That is what lets them be used to try a configuration out
// before it is saved - and why the connection arguments below repeat on each
// of these data sources. They are written out in full rather than shared from
// a helper because docsdriftlint and schemalint read the schemas statically.

// ldapConnectionSettings builds the LDAPSettings object every one of these
// endpoints takes.
func ldapConnectionSettings(d *schema.ResourceData) map[string]interface{} {
	settings := map[string]interface{}{
		"URL":           d.Get("url").(string),
		"AnonymousMode": d.Get("anonymous_mode").(bool),
		"StartTLS":      d.Get("start_tls").(bool),
		"TLSConfig": map[string]interface{}{
			"TLSSkipVerify": d.Get("tls_skip_verify").(bool),
		},
	}
	if v, ok := d.GetOk("reader_dn"); ok && v.(string) != "" {
		settings["ReaderDN"] = v.(string)
	}
	if v, ok := d.GetOk("password"); ok && v.(string) != "" {
		settings["Password"] = v.(string)
	}
	return settings
}

// ldapSearchSettings encodes the user search blocks.
func ldapSearchSettings(raw interface{}) []map[string]interface{} {
	list, _ := raw.([]interface{})
	out := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		entry := map[string]interface{}{"BaseDN": block["base_dn"]}
		if v, ok := block["filter"].(string); ok && v != "" {
			entry["Filter"] = v
		}
		if v, ok := block["user_name_attribute"].(string); ok && v != "" {
			entry["UserNameAttribute"] = v
		}
		out = append(out, entry)
	}
	return out
}

// ldapGroupSearchSettings encodes the group search blocks, which both the
// group listing and the admin group listing take.
func ldapGroupSearchSettings(raw interface{}) []map[string]interface{} {
	list, _ := raw.([]interface{})
	out := make([]map[string]interface{}, 0, len(list))
	for _, item := range list {
		block, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		entry := map[string]interface{}{"GroupBaseDN": block["group_base_dn"]}
		if v, ok := block["group_filter"].(string); ok && v != "" {
			entry["GroupFilter"] = v
		}
		if v, ok := block["group_attribute"].(string); ok && v != "" {
			entry["GroupAttribute"] = v
		}
		out = append(out, entry)
	}
	return out
}

// ldapEntries flattens the directory entries the user and group searches
// return; both endpoints answer with the same shape.
func ldapEntries(entries []struct {
	Name   string   `json:"Name"`
	Groups []string `json:"Groups"`
},
) []interface{} {
	out := make([]interface{}, 0, len(entries))
	for _, entry := range entries {
		out = append(out, map[string]interface{}{"name": entry.Name, "groups": entry.Groups})
	}
	return out
}

func dataSourceLDAPUsers() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceLDAPUsersRead,

		Schema: map[string]*schema.Schema{
			"url": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "URL of the LDAP server, as `host:port` (for example `ldap.example.com:389`).",
			},
			"reader_dn": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Distinguished name of the account used to browse the directory. Leave unset when `anonymous_mode` is enabled.",
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Password of the reader account. Stored in state as a sensitive value.",
			},
			"anonymous_mode": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to bind anonymously instead of using `reader_dn` and `password`.",
			},
			"start_tls": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to upgrade the connection with StartTLS.",
			},
			"tls_skip_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to skip verification of the LDAP server's TLS certificate.",
			},
			"search": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Where and how to look for users. Repeatable, and searched in order.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"base_dn": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Distinguished name the search starts from.",
						},
						"filter": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "LDAP filter narrowing the search, for example `(objectClass=person)`.",
						},
						"user_name_attribute": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Attribute holding the login name, for example `uid` or `sAMAccountName`.",
						},
					},
				},
			},
			// Computed attributes
			"entries": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The users the directory returned for the searches above.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name the directory reports for the entry.",
						},
						"groups": {
							Type:        schema.TypeList,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Groups the entry belongs to.",
						},
					},
				},
			},
		},
	}
}

func dataSourceLDAPUsersRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	settings := ldapConnectionSettings(d)
	if searches := ldapSearchSettings(d.Get("search")); len(searches) > 0 {
		settings["SearchSettings"] = searches
	}

	var users []struct {
		Name   string   `json:"Name"`
		Groups []string `json:"Groups"`
	}
	payload := map[string]interface{}{"LDAPSettings": settings}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/ldap/users", payload, &users); err != nil {
		return diag.FromErr(fmt.Errorf("failed to search the directory for users: %w", err))
	}

	if err := setFields(d, map[string]interface{}{"entries": ldapEntries(users)}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-ldap-users-" + d.Get("url").(string))
	return nil
}

func dataSourceLDAPGroups() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceLDAPGroupsRead,

		Schema: map[string]*schema.Schema{
			"url": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "URL of the LDAP server, as `host:port` (for example `ldap.example.com:389`).",
			},
			"reader_dn": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Distinguished name of the account used to browse the directory. Leave unset when `anonymous_mode` is enabled.",
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Password of the reader account. Stored in state as a sensitive value.",
			},
			"anonymous_mode": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to bind anonymously instead of using `reader_dn` and `password`.",
			},
			"start_tls": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to upgrade the connection with StartTLS.",
			},
			"tls_skip_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to skip verification of the LDAP server's TLS certificate.",
			},
			"group_search": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Where and how to look for groups. Repeatable, and searched in order.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"group_base_dn": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Distinguished name the group search starts from.",
						},
						"group_filter": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "LDAP filter narrowing the group search.",
						},
						"group_attribute": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Attribute on a group entry listing its members.",
						},
					},
				},
			},
			// Computed attributes
			"entries": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "The groups the directory returned for the searches above.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Computed:    true,
							Description: "Name the directory reports for the entry.",
						},
						"groups": {
							Type:        schema.TypeList,
							Computed:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "Groups the entry belongs to.",
						},
					},
				},
			},
		},
	}
}

func dataSourceLDAPGroupsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	settings := ldapConnectionSettings(d)
	if searches := ldapGroupSearchSettings(d.Get("group_search")); len(searches) > 0 {
		settings["GroupSearchSettings"] = searches
	}

	var groups []struct {
		Name   string   `json:"Name"`
		Groups []string `json:"Groups"`
	}
	payload := map[string]interface{}{"LDAPSettings": settings}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/ldap/groups", payload, &groups); err != nil {
		return diag.FromErr(fmt.Errorf("failed to search the directory for groups: %w", err))
	}

	if err := setFields(d, map[string]interface{}{"entries": ldapEntries(groups)}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-ldap-groups-" + d.Get("url").(string))
	return nil
}

func dataSourceLDAPAdminGroups() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceLDAPAdminGroupsRead,

		Schema: map[string]*schema.Schema{
			"url": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "URL of the LDAP server, as `host:port` (for example `ldap.example.com:389`).",
			},
			"reader_dn": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Distinguished name of the account used to browse the directory. Leave unset when `anonymous_mode` is enabled.",
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Password of the reader account. Stored in state as a sensitive value.",
			},
			"anonymous_mode": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to bind anonymously instead of using `reader_dn` and `password`.",
			},
			"start_tls": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to upgrade the connection with StartTLS.",
			},
			"tls_skip_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to skip verification of the LDAP server's TLS certificate.",
			},
			"admin_group_search": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Where and how to look for the groups whose members become administrators. Repeatable, and searched in order.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"group_base_dn": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Distinguished name the group search starts from.",
						},
						"group_filter": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "LDAP filter narrowing the group search.",
						},
						"group_attribute": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Attribute on a group entry listing its members.",
						},
					},
				},
			},
			// Computed attributes
			"group_names": {
				Type:        schema.TypeList,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Names of the groups whose members Portainer would make administrators.",
			},
		},
	}
}

func dataSourceLDAPAdminGroupsRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	settings := ldapConnectionSettings(d)
	if searches := ldapGroupSearchSettings(d.Get("admin_group_search")); len(searches) > 0 {
		settings["AdminGroupSearchSettings"] = searches
	}

	// This endpoint answers with a plain list of names rather than the entry
	// objects the user and group searches return.
	var names []string
	payload := map[string]interface{}{"LDAPSettings": settings}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/ldap/admin-groups", payload, &names); err != nil {
		return diag.FromErr(fmt.Errorf("failed to fetch the LDAP administrator groups: %w", err))
	}

	if err := setFields(d, map[string]interface{}{"group_names": names}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-ldap-admin-groups-" + d.Get("url").(string))
	return nil
}

func dataSourceLDAPLoginTest() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceLDAPLoginTestRead,

		Schema: map[string]*schema.Schema{
			"url": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "URL of the LDAP server, as `host:port` (for example `ldap.example.com:389`).",
			},
			"reader_dn": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Distinguished name of the account used to browse the directory. Leave unset when `anonymous_mode` is enabled.",
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Password of the reader account. Stored in state as a sensitive value.",
			},
			"anonymous_mode": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to bind anonymously instead of using `reader_dn` and `password`.",
			},
			"start_tls": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to upgrade the connection with StartTLS.",
			},
			"tls_skip_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to skip verification of the LDAP server's TLS certificate.",
			},
			"search": {
				Type:        schema.TypeList,
				Optional:    true,
				Description: "Where and how to look for users. Repeatable, and searched in order.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"base_dn": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Distinguished name the search starts from.",
						},
						"filter": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "LDAP filter narrowing the search, for example `(objectClass=person)`.",
						},
						"user_name_attribute": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Attribute holding the login name, for example `uid` or `sAMAccountName`.",
						},
					},
				},
			},
			"username": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Login name to try.",
			},
			"test_password": {
				Type:      schema.TypeString,
				Required:  true,
				Sensitive: true,
				// Not "password": that argument is already the reader account's
				// password, and conflating the two would be a security trap.
				Description: "Password to try for `username`. This is the password being tested, not the reader account's.",
			},
			"fail_on_error": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether credentials the directory rejects fail the plan. Defaults to `false`, because a rejected login is usually the answer being asked for rather than an error.",
			},
			// Computed attributes
			"valid": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether the directory accepted the credentials.",
			},
		},
	}
}

func dataSourceLDAPLoginTestRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	settings := ldapConnectionSettings(d)
	if searches := ldapSearchSettings(d.Get("search")); len(searches) > 0 {
		settings["SearchSettings"] = searches
	}

	payload := map[string]interface{}{
		"LDAPSettings": settings,
		"Username":     d.Get("username").(string),
		"Password":     d.Get("test_password").(string),
	}

	var response struct {
		Valid bool `json:"valid"`
	}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/ldap/test", payload, &response); err != nil {
		return diag.FromErr(fmt.Errorf("failed to test the LDAP login: %w", err))
	}

	if !response.Valid && d.Get("fail_on_error").(bool) {
		return diag.FromErr(fmt.Errorf("the directory rejected the credentials of %s (set fail_on_error = false to read the result instead)", d.Get("username").(string)))
	}

	if err := setFields(d, map[string]interface{}{"valid": response.Valid}); err != nil {
		return diag.FromErr(err)
	}
	d.SetId("portainer-ldap-login-test-" + d.Get("username").(string))
	return nil
}
