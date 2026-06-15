package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
				ForceNew:    true,
				Description: "Local alias name for the installed Docker plugin.",
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

	query := fmt.Sprintf("?remote=%s", remote)
	if name != "" {
		query += fmt.Sprintf("&name=%s", name)
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

	path := fmt.Sprintf("/endpoints/%d/docker/plugins/pull%s", endpointID, query)
	resp, err := client.DoRequest(http.MethodPost, path, headers, settings)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to install plugin: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to install plugin: %s", string(body)))
	}

	// enable if desired

	if enable {
		enablePath := fmt.Sprintf("/endpoints/%d/docker/plugins/%s/enable", endpointID, name)
		enableResp, err := client.DoRequest(http.MethodPost, enablePath, nil, nil)
		if err != nil {
			return diag.FromErr(fmt.Errorf("plugin installed but failed to enable: %w", err))
		}
		defer enableResp.Body.Close()

		if enableResp.StatusCode >= 300 {
			body, _ := io.ReadAll(enableResp.Body)
			return diag.FromErr(fmt.Errorf("plugin installed but failed to enable: %s", string(body)))
		}
	}

	d.SetId(name)
	return nil
}

func resourceDockerPluginDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	plugin := d.Id()

	path := fmt.Sprintf("/endpoints/%d/docker/plugins/%s", endpointID, plugin)
	resp, err := client.DoRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to delete plugin: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to delete plugin: %s", string(body)))
	}

	d.SetId("")
	return nil
}

func resourceDockerPluginRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	pluginName := d.Id()
	url := fmt.Sprintf("%s/endpoints/%d/docker/plugins/%s/json", client.Endpoint, endpointID, pluginName)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)

	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return diag.FromErr(fmt.Errorf("no valid authentication method provided (api_key or jwt token)"))
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to fetch docker plugin: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		d.SetId("")
		return nil
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to read docker plugin, status: %d, body: %s", resp.StatusCode, string(body)))
	}

	var plugin struct {
		Enabled  bool `json:"Enabled"`
		Settings struct {
			Args []string `json:"Args"`
		} `json:"Settings"`
		Config struct {
			Remote      string `json:"Remote"`
			Description string `json:"Description"`
			Interface   struct {
				Types json.RawMessage `json:"Types"`
			} `json:"Interface"`
			Settings struct {
				Env []string `json:"Env"`
			} `json:"Settings"`
		} `json:"Config"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&plugin); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode plugin data: %w", err))
	}

	if err := d.Set("enable", plugin.Enabled); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("remote", plugin.Config.Remote); err != nil {
		return diag.FromErr(err)
	}

	// settings reconstruction (limited to env-based ones)
	settings := make([]map[string]interface{}, 0, len(plugin.Config.Settings.Env))
	for _, env := range plugin.Config.Settings.Env {
		// env is of form "KEY=value"
		var name, value string
		n := strings.Index(env, "=")
		if n >= 0 {
			name = env[:n]
			value = env[n+1:]
		} else {
			name = env
			value = ""
		}
		settings = append(settings, map[string]interface{}{
			"name":  name,
			"value": []interface{}{value},
		})
	}
	if len(settings) > 0 {
		if err := d.Set("settings", settings); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}
