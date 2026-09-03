package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// gitopsSourceDetail is the sources.SourceDetail payload returned by
// GET /gitops/sources/{id}.
type gitopsSourceDetail struct {
	ID         int    `json:"id"`
	Name       string `json:"name"`
	URL        string `json:"url"`
	Type       string `json:"type"`
	Status     string `json:"status"`
	Interval   string `json:"interval"`
	LastSync   int    `json:"lastSync"`
	Error      string `json:"error"`
	Connection struct {
		TLSSkipVerify  bool `json:"tlsSkipVerify"`
		Authentication struct {
			Username string `json:"username"`
		} `json:"authentication"`
	} `json:"connection"`
	Access struct {
		Public bool  `json:"public"`
		Teams  []int `json:"teams"`
		Users  []int `json:"users"`
	} `json:"access"`
}

func resourceGitopsSource() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceGitopsSourceCreate,
		ReadContext:   resourceGitopsSourceRead,
		UpdateContext: resourceGitopsSourceUpdate,
		DeleteContext: resourceGitopsSourceDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"url": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "URL of the Git repository the source syncs from.",
			},
			"name": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Name of the GitOps source as displayed in Portainer. Portainer derives one from the repository URL when this is not set.",
			},
			"interval": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Polling interval for the repository as a Go duration string (for example `5m`). Leave unset to keep Portainer's default.",
			},
			"username": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Username used to authenticate against the Git repository. Leave unset for a public repository.",
			},
			"password": {
				Type:        schema.TypeString,
				Optional:    true,
				Sensitive:   true,
				Description: "Password or personal access token paired with `username`. Portainer never returns it, so it is kept from configuration and cannot detect drift.",
			},
			"tls_skip_verify": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				Description: "Whether to skip TLS certificate verification when contacting the Git repository.",
			},
			"administrators_only": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				ForceNew:    true,
				Description: "Whether the source is restricted to administrators. Only honoured at creation time; Portainer's update API does not carry this field.",
			},
			"public": {
				Type:        schema.TypeBool,
				Optional:    true,
				Computed:    true,
				Description: "Whether every user can use this source. Changing it is applied through the source's access-control endpoint.",
			},
			"user_accesses": {
				Type:        schema.TypeList,
				Optional:    true,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "IDs of users granted access to the source. Applied through the source's access-control endpoint.",
			},
			"team_accesses": {
				Type:        schema.TypeList,
				Optional:    true,
				Computed:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "IDs of teams granted access to the source. Applied through the source's access-control endpoint.",
			},
			// Computed attributes
			"type": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Source type reported by Portainer: `git`, `helm` or `oci`. This resource always creates a `git` source.",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Sync status of the source: `healthy`, `syncing`, `error`, `paused` or `unknown`.",
			},
			"status_error": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Error reported by the last synchronisation attempt, empty while the source is healthy.",
			},
			"last_sync": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Unix timestamp of the last successful synchronisation, zero when the source has never synced.",
			},
		},
	}
}

// gitopsAuthPayload returns the authentication object for a create/update
// payload, or nil when no credentials are configured.
func gitopsAuthPayload(d *schema.ResourceData) map[string]interface{} {
	username := d.Get("username").(string)
	password := d.Get("password").(string)
	if username == "" && password == "" {
		return nil
	}
	return map[string]interface{}{"username": username, "password": password}
}

func resourceGitopsSourceCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := map[string]interface{}{
		"url":                d.Get("url").(string),
		"tlsSkipVerify":      d.Get("tls_skip_verify").(bool),
		"administratorsOnly": d.Get("administrators_only").(bool),
		"public":             d.Get("public").(bool),
	}
	if v, ok := d.GetOk("name"); ok {
		payload["name"] = v.(string)
	}
	if v, ok := d.GetOk("interval"); ok {
		payload["interval"] = v.(string)
	}
	if auth := gitopsAuthPayload(d); auth != nil {
		payload["authentication"] = auth
	}
	if v, ok := d.GetOk("user_accesses"); ok {
		payload["userAccesses"] = toIntSlice(v.([]interface{}))
	}
	if v, ok := d.GetOk("team_accesses"); ok {
		payload["teamAccesses"] = toIntSlice(v.([]interface{}))
	}

	var created struct {
		ID int `json:"id"`
	}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/gitops/sources/git", payload, &created); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create GitOps source: %w", err))
	}
	if created.ID == 0 {
		return diag.FromErr(fmt.Errorf("failed to create GitOps source: Portainer returned no source ID"))
	}

	d.SetId(strconv.Itoa(created.ID))
	return resourceGitopsSourceRead(ctx, d, meta)
}

func resourceGitopsSourceRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var source gitopsSourceDetail
	url := fmt.Sprintf("%s/gitops/sources/%s", client.Endpoint, d.Id())
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &source); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read GitOps source %s: %w", d.Id(), err))
	}

	teams := source.Access.Teams
	if teams == nil {
		teams = []int{}
	}
	users := source.Access.Users
	if users == nil {
		users = []int{}
	}

	// The password is never returned, so it stays as configured; everything
	// else reflects the API, including the username Portainer stores.
	if err := setFields(d, map[string]interface{}{
		"name":            source.Name,
		"url":             source.URL,
		"interval":        source.Interval,
		"type":            source.Type,
		"status":          source.Status,
		"status_error":    source.Error,
		"last_sync":       source.LastSync,
		"tls_skip_verify": source.Connection.TLSSkipVerify,
		"username":        source.Connection.Authentication.Username,
		"public":          source.Access.Public,
		"team_accesses":   teams,
		"user_accesses":   users,
	}); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceGitopsSourceUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	// Portainer splits the update in two: the git connection is updated on the
	// source itself, while access control has its own endpoint and is not part
	// of sources.GitSourceUpdatePayload.
	if d.HasChanges("url", "name", "interval", "tls_skip_verify", "username", "password") {
		payload := map[string]interface{}{
			"url":           d.Get("url").(string),
			"name":          d.Get("name").(string),
			"interval":      d.Get("interval").(string),
			"tlsSkipVerify": d.Get("tls_skip_verify").(bool),
		}
		if auth := gitopsAuthPayload(d); auth != nil {
			payload["authentication"] = auth
		}
		url := fmt.Sprintf("%s/gitops/sources/%s", client.Endpoint, d.Id())
		if err := doJSON(ctx, client, http.MethodPut, url, payload, nil); err != nil {
			return diag.FromErr(fmt.Errorf("failed to update GitOps source %s: %w", d.Id(), err))
		}
	}

	if d.HasChanges("public", "user_accesses", "team_accesses") {
		payload := map[string]interface{}{
			"public": d.Get("public").(bool),
			"users":  toIntSlice(d.Get("user_accesses").([]interface{})),
			"teams":  toIntSlice(d.Get("team_accesses").([]interface{})),
		}
		url := fmt.Sprintf("%s/gitops/sources/%s/access", client.Endpoint, d.Id())
		if err := doJSON(ctx, client, http.MethodPut, url, payload, nil); err != nil {
			return diag.FromErr(fmt.Errorf("failed to update access control of GitOps source %s: %w", d.Id(), err))
		}
	}

	return resourceGitopsSourceRead(ctx, d, meta)
}

func resourceGitopsSourceDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	url := fmt.Sprintf("%s/gitops/sources/%s", client.Endpoint, d.Id())
	if err := doJSON(ctx, client, http.MethodDelete, url, nil, nil); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete GitOps source %s: %w", d.Id(), err))
	}

	d.SetId("")
	return nil
}
