package internal

import (
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceDockerPlugin() *schema.Resource {
	return &schema.Resource{
		Create: resourceDockerPluginCreate,
		Delete: resourceDockerPluginDelete,
		Read:   resourceDockerPluginRead,
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
	// optional: could query /plugins/{name}/json if needed
	return nil
}
