package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceRegistry() *schema.Resource {
	return &schema.Resource{
		Create: resourceRegistryCreate,
		Read:   resourceRegistryRead,
		Delete: resourceRegistryDelete,
		Update: resourceRegistryUpdate,

		Schema: map[string]*schema.Schema{
			"name":           {Type: schema.TypeString, Required: true},
			"url":            {Type: schema.TypeString, Required: true},
			"base_url":       {Type: schema.TypeString, Optional: true},
			"type":           {Type: schema.TypeInt, Required: true, ForceNew: true},
			"authentication": {Type: schema.TypeBool, Optional: true, Default: false},
			"username":       {Type: schema.TypeString, Optional: true},
			"password":       {Type: schema.TypeString, Optional: true, Sensitive: true},
			"instance_url":   {Type: schema.TypeString, Optional: true},
			"aws_region":     {Type: schema.TypeString, Optional: true},
		},
	}
}

func resourceRegistryCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	registryType := d.Get("type").(int)
	name := d.Get("name").(string)
	url := d.Get("url").(string)
	baseURL := d.Get("base_url").(string)
	auth := d.Get("authentication").(bool)

	body := map[string]interface{}{
		"name": name,
		"type": registryType,
	}

	switch registryType {
	case 1: // Quay.io
		body["url"] = url
		body["authentication"] = true
		body["username"] = d.Get("username").(string)
		body["password"] = d.Get("password").(string)
	case 2: // Azure
		body["url"] = url
		body["baseURL"] = baseURL
		body["authentication"] = true
		body["username"] = d.Get("username").(string)
		body["password"] = d.Get("password").(string)
	case 3: // Custom
		body["url"] = url
		body["baseURL"] = baseURL
		body["authentication"] = auth
		if auth {
			body["username"] = d.Get("username").(string)
			body["password"] = d.Get("password").(string)
		}
	case 4: // GitLab
		body["url"] = url
		body["authentication"] = true
		body["username"] = d.Get("username").(string)
		body["password"] = d.Get("password").(string)
		body["gitlab"] = map[string]interface{}{
			"InstanceURL": d.Get("instance_url").(string),
		}
	case 5: // ProGet
		body["url"] = url
		body["baseURL"] = baseURL
		body["authentication"] = true
		body["username"] = d.Get("username").(string)
		body["password"] = d.Get("password").(string)
	case 6: // DockerHub
		body["url"] = url
		body["authentication"] = true
		body["username"] = d.Get("username").(string)
		body["password"] = d.Get("password").(string)
	case 7: // AWS ECR
		ecr := map[string]interface{}{}
		if v, ok := d.GetOk("aws_region"); ok {
			ecr["Region"] = v.(string)
		}
		body["url"] = url
		body["ecr"] = ecr
		body["authentication"] = auth
		if auth {
			body["username"] = d.Get("username").(string)
			body["password"] = d.Get("password").(string)
		}
	default:
		return fmt.Errorf("unsupported registry type: %d", registryType)
	}

	resp, err := client.DoRequest("POST", "/registries", nil, body)
	if err != nil {
		return fmt.Errorf("failed to create registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create registry: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	d.SetId(strconv.Itoa(result.ID))
	return resourceRegistryRead(d, meta)
}

func resourceRegistryRead(d *schema.ResourceData, meta interface{}) error {
	return nil
}

func resourceRegistryUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Id()

	body := map[string]interface{}{
		"name":           d.Get("name").(string),
		"url":            d.Get("url").(string),
		"baseURL":        d.Get("base_url").(string),
		"authentication": d.Get("authentication").(bool),
		"username":       d.Get("username").(string),
		"password":       d.Get("password").(string),
	}

	if d.Get("type").(int) == 4 {
		body["gitlab"] = map[string]interface{}{
			"InstanceURL": d.Get("instance_url").(string),
		}
	}
	if d.Get("type").(int) == 7 {
		body["ecr"] = map[string]interface{}{
			"Region": d.Get("aws_region").(string),
		}
	}

	resp, err := client.DoRequest("PUT", fmt.Sprintf("/registries/%s", id), nil, body)
	if err != nil {
		return fmt.Errorf("failed to update registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update registry: %s", string(data))
	}

	return nil
}

func resourceRegistryDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	resp, err := client.DoRequest("DELETE", fmt.Sprintf("/registries/%s", d.Id()), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 204 {
		return fmt.Errorf("failed to delete registry")
	}
	return nil
}
