package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceDockerPlugin() *schema.Resource {
	return &schema.Resource{
		Create: resourceDockerPluginCreate,
		Delete: resourceDockerPluginDelete,
		Read:   resourceDockerPluginRead,
		Importer: &schema.ResourceImporter{
			State: func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
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
				Type:     schema.TypeInt,
				Required: true,
				ForceNew: true,
			},
			"remote": {
				Type:     schema.TypeString,
				Required: true,
				ForceNew: true,
			},
			"name": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
			},
			"registry_auth": {
				Type:     schema.TypeString,
				Optional: true,
				ForceNew: true,
				Default:  "e30=",
			},
			"enable": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
				ForceNew: true,
			},
			"settings": {
				Type:     schema.TypeList,
				Optional: true,
				ForceNew: true,
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:     schema.TypeString,
							Required: true,
						},
						"description": {
							Type:     schema.TypeString,
							Optional: true,
						},
						"value": {
							Type:     schema.TypeList,
							Required: true,
							Elem:     &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
		},
	}
}

func resourceDockerPluginCreate(d *schema.ResourceData, meta interface{}) error {
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
		return fmt.Errorf("failed to install plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to install plugin: %s", string(body))
	}

	// enable if desired

	if enable {
		enablePath := fmt.Sprintf("/endpoints/%d/docker/plugins/%s/enable", endpointID, name)
		enableResp, err := client.DoRequest(http.MethodPost, enablePath, nil, nil)
		if err != nil {
			return fmt.Errorf("plugin installed but failed to enable: %w", err)
		}
		defer enableResp.Body.Close()

		if enableResp.StatusCode >= 300 {
			body, _ := io.ReadAll(enableResp.Body)
			return fmt.Errorf("plugin installed but failed to enable: %s", string(body))
		}
	}

	d.SetId(name)
	return nil
}

func resourceDockerPluginDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	plugin := d.Id()

	path := fmt.Sprintf("/endpoints/%d/docker/plugins/%s", endpointID, plugin)
	resp, err := client.DoRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 204 && resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete plugin: %s", string(body))
	}

	d.SetId("")
	return nil
}

func resourceDockerPluginRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	pluginName := d.Id()
	url := fmt.Sprintf("%s/endpoints/%d/docker/plugins/%s/json", client.Endpoint, endpointID, pluginName)
	req, _ := http.NewRequest("GET", url, nil)

	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch docker plugin: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	}
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read docker plugin, status: %d, body: %s", resp.StatusCode, string(body))
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
		return fmt.Errorf("failed to decode plugin data: %w", err)
	}

	d.Set("enable", plugin.Enabled)
	d.Set("remote", plugin.Config.Remote)

	// settings reconstruction (limited to env-based ones)
	var settings []map[string]interface{}
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
		d.Set("settings", settings)
	}

	return nil
}
