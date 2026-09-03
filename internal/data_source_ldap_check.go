package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceLDAPCheck() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceLDAPCheckRead,

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
			"fail_on_error": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     true,
				Description: "Whether an unreachable directory makes the data source itself fail. Defaults to true, which is the point of a pre-flight check; set it to false to branch on `success` instead.",
			},
			// Computed attributes
			"success": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether Portainer could reach and bind to the directory.",
			},
			"error": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Reason the check failed, empty on success.",
			},
		},
	}
}

func dataSourceLDAPCheckRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	settings := map[string]interface{}{
		"URL":           d.Get("url").(string),
		"AnonymousMode": d.Get("anonymous_mode").(bool),
		"StartTLS":      d.Get("start_tls").(bool),
		"TLSConfig": map[string]interface{}{
			"TLSSkipVerify": d.Get("tls_skip_verify").(bool),
		},
	}
	if v, ok := d.GetOk("reader_dn"); ok {
		settings["ReaderDN"] = v.(string)
	}
	if v, ok := d.GetOk("password"); ok {
		settings["Password"] = v.(string)
	}

	// The endpoint answers 204 on success and 400/500 with a message otherwise,
	// so the outcome is the status code rather than a body.
	err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/ldap/check",
		map[string]interface{}{"LDAPSettings": settings}, nil)

	success := err == nil
	message := ""
	if err != nil {
		var se *apiStatusError
		if errors.As(err, &se) {
			message = se.Body
		} else {
			// A transport-level failure is not a directory answer, so it is
			// raised rather than reported as a failed check.
			return diag.FromErr(fmt.Errorf("failed to check the LDAP connection: %w", err))
		}
	}

	if err := setFields(d, map[string]interface{}{
		"success": success,
		"error":   message,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId("ldap-check-" + d.Get("url").(string))

	if !success && d.Get("fail_on_error").(bool) {
		return diag.FromErr(fmt.Errorf("the LDAP connection check failed: %s", message))
	}

	return nil
}
