package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceDockerNetwork() *schema.Resource {
	return &schema.Resource{
		Create: resourceDockerNetworkCreate,
		Read:   resourceDockerNetworkRead,
		Delete: resourceDockerNetworkDelete,
		Update: nil,
		Schema: map[string]*schema.Schema{
			"endpoint_id": {Type: schema.TypeInt, Required: true, ForceNew: true},
			"name":        {Type: schema.TypeString, Required: true, ForceNew: true},
			"driver":      {Type: schema.TypeString, Optional: true, Default: "bridge", ForceNew: true},
			"scope":       {Type: schema.TypeString, Optional: true, ForceNew: true},
			"internal":    {Type: schema.TypeBool, Optional: true, Default: false, ForceNew: true},
			"attachable":  {Type: schema.TypeBool, Optional: true, Default: false, ForceNew: true},
			"ingress":     {Type: schema.TypeBool, Optional: true, Default: false, ForceNew: true},
			"config_only": {Type: schema.TypeBool, Optional: true, Default: false, ForceNew: true},
			"config_from": {Type: schema.TypeString, Optional: true, ForceNew: true},
			"enable_ipv4": {Type: schema.TypeBool, Optional: true, Default: true, ForceNew: true},
			"enable_ipv6": {Type: schema.TypeBool, Optional: true, Default: false, ForceNew: true},
			"options":     {Type: schema.TypeMap, Optional: true, Elem: &schema.Schema{Type: schema.TypeString}, ForceNew: true},
			"labels":      {Type: schema.TypeMap, Optional: true, Elem: &schema.Schema{Type: schema.TypeString}, ForceNew: true},
		},
	}
}

func resourceDockerNetworkCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)

	payload := map[string]interface{}{
		"Name":       d.Get("name").(string),
		"Driver":     d.Get("driver").(string),
		"Internal":   d.Get("internal").(bool),
		"Attachable": d.Get("attachable").(bool),
		"Ingress":    d.Get("ingress").(bool),
		"ConfigOnly": d.Get("config_only").(bool),
		"EnableIPv4": d.Get("enable_ipv4").(bool),
		"EnableIPv6": d.Get("enable_ipv6").(bool),
		"Options":    d.Get("options").(map[string]interface{}),
		"Labels":     d.Get("labels").(map[string]interface{}),
	}

	if v, ok := d.GetOk("scope"); ok {
		payload["Scope"] = v.(string)
	}
	if v, ok := d.GetOk("config_from"); ok {
		payload["ConfigFrom"] = map[string]string{"Network": v.(string)}
	}

	var response struct {
		ID string `json:"Id"`
	}

	path := fmt.Sprintf("/endpoints/%d/docker/networks/create", endpointID)
	resp, err := client.DoRequest(http.MethodPost, path, nil, payload)
	if err != nil {
		return fmt.Errorf("failed to create docker network: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create docker network: %s", string(body))
	}

	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return err
	}

	d.SetId(response.ID)
	return nil
}

func resourceDockerNetworkRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceDockerNetworkDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	endpointID := d.Get("endpoint_id").(int)
	id := d.Id()

	path := fmt.Sprintf("/endpoints/%d/docker/networks/%s", endpointID, id)
	resp, err := client.DoRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete docker network: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 && resp.StatusCode != 200 && resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete docker network: %s", string(body))
	}

	d.SetId("")
	return nil
}
