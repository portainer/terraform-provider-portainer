package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceDockerPlugin() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceDockerPluginCreate,
		DeleteContext: resourceDockerPluginDelete,
		ReadContext:   resourceDockerPluginRead,
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				parts := strings.SplitN(d.Id(), ":", 2)
				if len(parts) != 2 {
					return nil, fmt.Errorf("unexpected ID format (%s), expected endpoint_id:plugin_name", d.Id())
				}
				endpointID, err := strconv.Atoi(parts[0])
				if err != nil {
					return nil, fmt.Errorf("invalid endpoint ID in ID: %w", err)
				}
				if err := d.Set("endpoint_id", endpointID); err != nil {
					return nil, err
				}
				// The registry credentials cannot be read back; assume the default.
				if err := setFields(d, map[string]interface{}{"name": parts[1], "registry_auth": "e30="}); err != nil {
					return nil, err
				}
				d.SetId(parts[1])
				return []*schema.ResourceData{d}, nil
			},
		},
		Schema: map[string]*schema.Schema{
			"endpoint_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "ID of the Portainer environment (Docker host) where the plugin is installed.",
			},
			"remote": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Remote source URL or registry reference of the Docker plugin to pull.",
			},
			"name": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				ForceNew:    true,
				Description: "Local alias name for the installed Docker plugin. Defaults to `remote`.",
			},
			"registry_auth": {
				Type:        schema.TypeString,
				Optional:    true,
				ForceNew:    true,
				Default:     "e30=",
				Description: "Base64-encoded registry authentication payload used to pull the plugin from a private registry.",
			},
			"enable": {
				Type:        schema.TypeBool,
				Optional:    true,
				Default:     false,
				ForceNew:    true,
				Description: "Whether to enable the Docker plugin immediately after installation.",
			},
			"settings": {
				Type:        schema.TypeList,
				Optional:    true,
				ForceNew:    true,
				Description: "Configuration settings applied to the Docker plugin during installation.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Name of the Docker plugin setting.",
						},
						"description": {
							Type:        schema.TypeString,
							Optional:    true,
							Description: "Human-readable description of the plugin setting.",
						},
						"value": {
							Type:        schema.TypeList,
							Required:    true,
							Elem:        &schema.Schema{Type: schema.TypeString},
							Description: "List of values assigned to the plugin setting.",
						},
					},
				},
			},
		},
	}
}

func resourceDockerPluginCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	remote := d.Get("remote").(string)
	name := d.Get("name").(string)
	enable := d.Get("enable").(bool)
	auth := d.Get("registry_auth").(string)

	query := url.Values{"remote": {remote}}
	if name != "" {
		query.Set("name", name)
	}

	var settings []interface{}
	if v, ok := d.GetOk("settings"); ok {
		for _, s := range v.([]interface{}) {
			item := s.(map[string]interface{})
			entry := map[string]interface{}{
				"Name":  item["name"].(string),
				"Value": item["value"].([]interface{}),
			}
			if desc, ok := item["description"]; ok && desc.(string) != "" {
				entry["Description"] = desc.(string)
			}
			settings = append(settings, entry)
		}
	}

	headers := map[string]string{
		"X-Registry-Auth": auth, // base64 encoded '{}'
	}

	path := fmt.Sprintf("/endpoints/%d/docker/plugins/pull?%s", endpointID, query.Encode())
	resp, err := client.DoRequest(ctx, http.MethodPost, path, headers, settings)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to install plugin: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to install plugin: %s", string(body)))
	}
	// Docker pulls the plugin while it streams the progress, and cancels the
	// pull when the client hangs up, so read the stream to the end. Failures
	// arrive inside it with status 200.
	if err := checkDockerStream(resp.Body); err != nil {
		return diag.FromErr(fmt.Errorf("failed to install plugin: %w", err))
	}

	// Docker names the plugin after the remote when no alias is given.
	id := name
	if id == "" {
		id = remote
	}

	// Set before enabling, so a plugin that installed but failed to enable
	// stays in state (tainted) instead of being left behind unmanaged.
	d.SetId(id)
	if err := d.Set("name", id); err != nil {
		return diag.FromErr(err)
	}

	if enable {
		// Docker requires the timeout parameter; without it the enable call
		// fails with "strconv.Atoi: parsing \"\"", so enable = true never
		// worked.
		enablePath := fmt.Sprintf("/endpoints/%d/docker/plugins/%s/enable?timeout=0", endpointID, id)
		enableResp, err := client.DoRequest(ctx, http.MethodPost, enablePath, nil, nil)
		if err != nil {
			return diag.FromErr(fmt.Errorf("plugin installed but failed to enable: %w", err))
		}
		defer enableResp.Body.Close()

		if enableResp.StatusCode >= 300 {
			body, _ := io.ReadAll(enableResp.Body)
			return diag.FromErr(fmt.Errorf("plugin installed but failed to enable: %s", string(body)))
		}
	}

	return nil
}

// checkDockerStream reads a Docker progress stream to the end and returns
// the first error reported in it.
func checkDockerStream(r io.Reader) error {
	dec := json.NewDecoder(r)
	for {
		var msg struct {
			Error string `json:"error"`
		}
		if err := dec.Decode(&msg); err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("reading the progress stream: %w", err)
		}
		if msg.Error != "" {
			return errors.New(msg.Error)
		}
	}
}

func resourceDockerPluginDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	plugin := d.Id()

	// force: Docker refuses to remove an enabled plugin, which made every
	// enable = true plugin impossible to destroy or replace.
	reqURL := fmt.Sprintf("%s/endpoints/%d/docker/plugins/%s?force=true", client.Endpoint, endpointID, plugin)
	if err := doJSON(ctx, client, http.MethodDelete, reqURL, nil, nil); err != nil && !isAPINotFound(err) {
		return diag.FromErr(fmt.Errorf("failed to delete plugin: %w", err))
	}

	d.SetId("")
	return nil
}

func resourceDockerPluginRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	pluginName := d.Id()
	reqURL := fmt.Sprintf("%s/endpoints/%d/docker/plugins/%s/json", client.Endpoint, endpointID, pluginName)

	var plugin struct {
		Enabled         bool   `json:"Enabled"`
		PluginReference string `json:"PluginReference"`
	}
	if err := doJSON(ctx, client, http.MethodGet, reqURL, nil, &plugin); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read docker plugin: %w", err))
	}

	if err := d.Set("enable", plugin.Enabled); err != nil {
		return diag.FromErr(err)
	}
	// Docker does not keep the remote as written nor the privileges granted at
	// install (settings), and both force a new plugin, so they keep their
	// configured values. Only an import fills them in: the remote from the
	// plugin reference and the settings from the privileges it requires.
	if d.Get("remote").(string) == "" {
		remote := normalizePluginReference(plugin.PluginReference)
		fields := map[string]interface{}{"remote": remote}
		// The privileges come from the registry, which a private or
		// unreachable one may refuse; the import then just has no settings.
		if settings, err := dockerPluginPrivileges(ctx, client, endpointID, remote); err != nil {
			log.Printf("[WARN] docker_plugin %s: settings not imported: %v", pluginName, err)
		} else {
			fields["settings"] = settings
		}
		if err := setFields(d, fields); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

// dockerPluginPrivileges returns the privileges the plugin asks for, which
// Docker requires the install to grant exactly, in the shape of settings.
func dockerPluginPrivileges(ctx context.Context, client *APIClient, endpointID int, remote string) ([]interface{}, error) {
	reqURL := fmt.Sprintf("%s/endpoints/%d/docker/plugins/privileges?%s", client.Endpoint, endpointID, url.Values{"remote": {remote}}.Encode())
	var privileges []struct {
		Name  string   `json:"Name"`
		Value []string `json:"Value"`
	}
	if err := doJSON(ctx, client, http.MethodGet, reqURL, nil, &privileges); err != nil {
		return nil, fmt.Errorf("failed to read the privileges of plugin %s: %w", remote, err)
	}
	settings := make([]interface{}, 0, len(privileges))
	for _, p := range privileges {
		values := make([]interface{}, 0, len(p.Value))
		for _, v := range p.Value {
			values = append(values, v)
		}
		settings = append(settings, map[string]interface{}{"name": p.Name, "value": values})
	}
	return settings, nil
}

// normalizePluginReference turns Docker's canonical plugin reference
// ("docker.io/rclone/docker-volume-rclone:latest") back into the short form
// users write ("rclone/docker-volume-rclone").
func normalizePluginReference(ref string) string {
	ref = strings.TrimPrefix(ref, "docker.io/")
	ref = strings.TrimPrefix(ref, "library/")
	return strings.TrimSuffix(ref, ":latest")
}
