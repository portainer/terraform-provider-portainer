package internal

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceLDAPSettings() *schema.Resource {
	return &schema.Resource{
		Create: resourceLDAPSettingsApply,
		Read:   resourceLDAPSettingsRead,
		Update: resourceLDAPSettingsApply,
		Delete: resourceLDAPSettingsDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"anonymous_mode": {
				Type:        schema.TypeBool,
				Optional:    true,
				Computed:    true,
				Description: "Enable anonymous mode. When enabled, ReaderDN and Password will not be used",
			},
			"auto_create_users": {
				Type:        schema.TypeBool,
				Optional:    true,
				Computed:    true,
				Description: "Automatically provision users and assign them to matching LDAP group names",
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Sensitive:   true,
				Description: "Password of the account used to search users",
			},
			"reader_dn": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Account that will be used to search for users (e.g. cn=readonly-account,dc=ldap,dc=domain,dc=tld)",
			},
			"start_tls": {
				Type:        schema.TypeBool,
				Optional:    true,
				Computed:    true,
				Description: "Whether LDAP connection should use StartTLS",
			},
			"url": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "URL or IP address of the LDAP server (deprecated, use urls)",
			},
			"urls": {
				Type:        schema.TypeList,
				Optional:    true,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "URLs or IP addresses of the LDAP server",
			},
			"server_type": {
				Type:        schema.TypeInt,
				Optional:    true,
				Computed:    true,
				Description: "LDAP server type",
			},
			"admin_auto_populate": {
				Type:        schema.TypeBool,
				Optional:    true,
				Computed:    true,
				Description: "Whether auto admin population is enabled",
			},
			"admin_groups": {
				Type:        schema.TypeList,
				Optional:    true,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeString},
				Description: "Saved admin group list",
			},
			"search_settings": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"base_dn": {
							Type:        schema.TypeString,
							Optional:    true,
							Computed:    true,
							Description: "Base DN for user search",
						},
						"filter": {
							Type:        schema.TypeString,
							Optional:    true,
							Computed:    true,
							Description: "LDAP search filter",
						},
						"user_name_attribute": {
							Type:        schema.TypeString,
							Optional:    true,
							Computed:    true,
							Description: "Attribute used for username",
						},
					},
				},
				Description: "LDAP user search settings",
			},
			"group_search_settings": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"group_attribute": {
							Type:        schema.TypeString,
							Optional:    true,
							Computed:    true,
							Description: "LDAP group attribute",
						},
						"group_base_dn": {
							Type:        schema.TypeString,
							Optional:    true,
							Computed:    true,
							Description: "Base DN for group search",
						},
						"group_filter": {
							Type:        schema.TypeString,
							Optional:    true,
							Computed:    true,
							Description: "LDAP group search filter",
						},
					},
				},
				Description: "LDAP group search settings",
			},
			"admin_group_search_settings": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"group_attribute": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
						"group_base_dn": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
						"group_filter": {
							Type:     schema.TypeString,
							Optional: true,
							Computed: true,
						},
					},
				},
				Description: "LDAP admin group search settings",
			},
			"tls_config": {
				Type:     schema.TypeList,
				Optional: true,
				Computed: true,
				MaxItems: 1,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"tls": {
							Type:        schema.TypeBool,
							Optional:    true,
							Computed:    true,
							Description: "Whether TLS is enabled",
						},
						"tls_ca_cert": {
							Type:        schema.TypeString,
							Optional:    true,
							Computed:    true,
							Description: "TLS CA certificate",
						},
						"tls_cert": {
							Type:        schema.TypeString,
							Optional:    true,
							Computed:    true,
							Description: "TLS certificate",
						},
						"tls_key": {
							Type:        schema.TypeString,
							Optional:    true,
							Computed:    true,
							Sensitive:   true,
							Description: "TLS key",
						},
						"tls_skip_verify": {
							Type:        schema.TypeBool,
							Optional:    true,
							Computed:    true,
							Description: "Skip TLS verification",
						},
					},
				},
				Description: "TLS configuration for LDAP",
			},
		},
	}
}

func resourceLDAPSettingsApply(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	// First read current settings to get the full payload
	resp, err := client.DoRequest("GET", "/settings", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to read current settings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read current settings (status %d): %s", resp.StatusCode, string(data))
	}

	var currentSettings map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&currentSettings); err != nil {
		return fmt.Errorf("failed to decode current settings: %w", err)
	}

	// Build LDAP settings
	ldap := map[string]interface{}{}

	if v, ok := d.GetOk("anonymous_mode"); ok {
		ldap["AnonymousMode"] = v.(bool)
	}
	if v, ok := d.GetOk("auto_create_users"); ok {
		ldap["AutoCreateUsers"] = v.(bool)
	}
	if v, ok := d.GetOk("password"); ok {
		ldap["Password"] = v.(string)
	}
	if v, ok := d.GetOk("reader_dn"); ok {
		ldap["ReaderDN"] = v.(string)
	}
	if v, ok := d.GetOk("start_tls"); ok {
		ldap["StartTLS"] = v.(bool)
	}
	if v, ok := d.GetOk("url"); ok {
		ldap["URL"] = v.(string)
	}
	if v, ok := d.GetOk("urls"); ok {
		urls := make([]string, 0)
		for _, u := range v.([]interface{}) {
			urls = append(urls, u.(string))
		}
		ldap["URLs"] = urls
	}
	if v, ok := d.GetOk("server_type"); ok {
		ldap["ServerType"] = v.(int)
	}
	if v, ok := d.GetOk("admin_auto_populate"); ok {
		ldap["AdminAutoPopulate"] = v.(bool)
	}
	if v, ok := d.GetOk("admin_groups"); ok {
		groups := make([]string, 0)
		for _, g := range v.([]interface{}) {
			groups = append(groups, g.(string))
		}
		ldap["AdminGroups"] = groups
	}

	if v, ok := d.GetOk("search_settings"); ok {
		settings := make([]map[string]interface{}, 0)
		for _, raw := range v.([]interface{}) {
			s := raw.(map[string]interface{})
			settings = append(settings, map[string]interface{}{
				"BaseDN":            s["base_dn"].(string),
				"Filter":            s["filter"].(string),
				"UserNameAttribute": s["user_name_attribute"].(string),
			})
		}
		ldap["SearchSettings"] = settings
	}

	if v, ok := d.GetOk("group_search_settings"); ok {
		settings := make([]map[string]interface{}, 0)
		for _, raw := range v.([]interface{}) {
			s := raw.(map[string]interface{})
			settings = append(settings, map[string]interface{}{
				"GroupAttribute": s["group_attribute"].(string),
				"GroupBaseDN":    s["group_base_dn"].(string),
				"GroupFilter":    s["group_filter"].(string),
			})
		}
		ldap["GroupSearchSettings"] = settings
	}

	if v, ok := d.GetOk("admin_group_search_settings"); ok {
		settings := make([]map[string]interface{}, 0)
		for _, raw := range v.([]interface{}) {
			s := raw.(map[string]interface{})
			settings = append(settings, map[string]interface{}{
				"GroupAttribute": s["group_attribute"].(string),
				"GroupBaseDN":    s["group_base_dn"].(string),
				"GroupFilter":    s["group_filter"].(string),
			})
		}
		ldap["AdminGroupSearchSettings"] = settings
	}

	if v, ok := d.GetOk("tls_config"); ok {
		items := v.([]interface{})
		if len(items) > 0 && items[0] != nil {
			m := items[0].(map[string]interface{})
			ldap["TLSConfig"] = map[string]interface{}{
				"TLS":           m["tls"].(bool),
				"TLSCACert":     m["tls_ca_cert"].(string),
				"TLSCert":       m["tls_cert"].(string),
				"TLSKey":        m["tls_key"].(string),
				"TLSSkipVerify": m["tls_skip_verify"].(bool),
			}
		}
	}

	// Set authentication method to LDAP (2) and update LDAP settings
	payload := map[string]interface{}{
		"authenticationMethod": 2,
		"ldapsettings":         ldap,
	}

	resp2, err := client.DoRequest("PUT", "/settings", nil, payload)
	if err != nil {
		return fmt.Errorf("failed to update LDAP settings: %w", err)
	}
	defer resp2.Body.Close()

	if resp2.StatusCode < 200 || resp2.StatusCode >= 300 {
		data, _ := io.ReadAll(resp2.Body)
		return fmt.Errorf("failed to update LDAP settings (status %d): %s", resp2.StatusCode, string(data))
	}

	d.SetId("portainer-ldap-settings")
	return resourceLDAPSettingsRead(d, meta)
}

func resourceLDAPSettingsRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	resp, err := client.DoRequest("GET", "/settings", nil, nil)
	if err != nil {
		return fmt.Errorf("failed to read settings: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read settings (status %d): %s", resp.StatusCode, string(data))
	}

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode settings: %w", err)
	}

	ldapRaw, ok := result["LDAPSettings"]
	if !ok || ldapRaw == nil {
		d.SetId("")
		return nil
	}

	ldapJSON, err := json.Marshal(ldapRaw)
	if err != nil {
		return fmt.Errorf("failed to marshal LDAP settings: %w", err)
	}

	var ldap map[string]interface{}
	if err := json.Unmarshal(ldapJSON, &ldap); err != nil {
		return fmt.Errorf("failed to unmarshal LDAP settings: %w", err)
	}

	d.SetId("portainer-ldap-settings")

	if v, ok := ldap["AnonymousMode"]; ok {
		d.Set("anonymous_mode", v)
	}
	if v, ok := ldap["AutoCreateUsers"]; ok {
		d.Set("auto_create_users", v)
	}
	if v, ok := ldap["ReaderDN"]; ok {
		d.Set("reader_dn", v)
	}
	if v, ok := ldap["StartTLS"]; ok {
		d.Set("start_tls", v)
	}
	if v, ok := ldap["URL"]; ok {
		d.Set("url", v)
	}
	if v, ok := ldap["URLs"]; ok {
		d.Set("urls", v)
	}
	if v, ok := ldap["ServerType"]; ok {
		d.Set("server_type", int(v.(float64)))
	}
	if v, ok := ldap["AdminAutoPopulate"]; ok {
		d.Set("admin_auto_populate", v)
	}
	if v, ok := ldap["AdminGroups"]; ok {
		d.Set("admin_groups", v)
	}

	// Preserve sensitive password from state
	if currentPW, ok := d.GetOk("password"); ok {
		if pwStr, ok := currentPW.(string); ok && pwStr != "" {
			d.Set("password", pwStr)
		}
	}

	// search_settings
	if raw, ok := ldap["SearchSettings"]; ok && raw != nil {
		if items, ok := raw.([]interface{}); ok {
			settings := make([]map[string]interface{}, 0, len(items))
			for _, item := range items {
				s := item.(map[string]interface{})
				settings = append(settings, map[string]interface{}{
					"base_dn":             s["BaseDN"],
					"filter":              s["Filter"],
					"user_name_attribute": s["UserNameAttribute"],
				})
			}
			d.Set("search_settings", settings)
		}
	}

	// group_search_settings
	if raw, ok := ldap["GroupSearchSettings"]; ok && raw != nil {
		if items, ok := raw.([]interface{}); ok {
			settings := make([]map[string]interface{}, 0, len(items))
			for _, item := range items {
				s := item.(map[string]interface{})
				settings = append(settings, map[string]interface{}{
					"group_attribute": s["GroupAttribute"],
					"group_base_dn":   s["GroupBaseDN"],
					"group_filter":    s["GroupFilter"],
				})
			}
			d.Set("group_search_settings", settings)
		}
	}

	// admin_group_search_settings
	if raw, ok := ldap["AdminGroupSearchSettings"]; ok && raw != nil {
		if items, ok := raw.([]interface{}); ok {
			settings := make([]map[string]interface{}, 0, len(items))
			for _, item := range items {
				s := item.(map[string]interface{})
				settings = append(settings, map[string]interface{}{
					"group_attribute": s["GroupAttribute"],
					"group_base_dn":   s["GroupBaseDN"],
					"group_filter":    s["GroupFilter"],
				})
			}
			d.Set("admin_group_search_settings", settings)
		}
	}

	// tls_config
	if raw, ok := ldap["TLSConfig"]; ok && raw != nil {
		tc := raw.(map[string]interface{})
		tlsConfig := map[string]interface{}{
			"tls":             tc["TLS"],
			"tls_ca_cert":     tc["TLSCACert"],
			"tls_cert":        tc["TLSCert"],
			"tls_skip_verify": tc["TLSSkipVerify"],
		}
		// Preserve sensitive tls_key from state
		if currentTLS, ok := d.GetOk("tls_config"); ok {
			items := currentTLS.([]interface{})
			if len(items) > 0 && items[0] != nil {
				current := items[0].(map[string]interface{})
				if keyRaw, ok := current["tls_key"]; ok {
					if keyStr, ok := keyRaw.(string); ok && keyStr != "" {
						tlsConfig["tls_key"] = keyStr
					}
				}
			}
		}
		d.Set("tls_config", []interface{}{tlsConfig})
	}

	return nil
}

func resourceLDAPSettingsDelete(d *schema.ResourceData, meta interface{}) error {
	d.SetId("")
	return nil
}
