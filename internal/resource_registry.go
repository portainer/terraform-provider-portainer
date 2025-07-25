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
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
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
	name := d.Get("name").(string)

	existingID, err := findRegistryByName(client, name)
	if err != nil {
		return err
	}
	if existingID != 0 {
		d.SetId(strconv.Itoa(existingID))
		return resourceRegistryUpdate(d, meta)
	}

	registryType := d.Get("type").(int)
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

func findRegistryByName(client *APIClient, name string) (int, error) {
	resp, err := client.DoRequest("GET", "/registries", nil, nil)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return 0, fmt.Errorf("failed to list registries: %s", string(data))
	}

	var registries []struct {
		Id   int    `json:"Id"`
		Name string `json:"Name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&registries); err != nil {
		return 0, err
	}

	for _, r := range registries {
		if r.Name == name {
			return r.Id, nil
		}
	}

	return 0, nil
}

func resourceRegistryRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	resp, err := client.DoRequest("GET", fmt.Sprintf("/registries/%s", d.Id()), nil, nil)
	if err != nil {
		return fmt.Errorf("failed to read registry: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	} else if resp.StatusCode != 200 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read registry: %s", string(data))
	}

	var registry struct {
		Name           string `json:"Name"`
		URL            string `json:"URL"`
		BaseURL        string `json:"BaseURL"`
		Type           int    `json:"Type"`
		Authentication bool   `json:"Authentication"`
		Username       string `json:"Username"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&registry); err != nil {
		return err
	}

	d.Set("name", registry.Name)
	d.Set("url", registry.URL)
	d.Set("base_url", registry.BaseURL)
	d.Set("type", registry.Type)
	d.Set("authentication", registry.Authentication)
	d.Set("username", registry.Username)

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

	return resourceRegistryRead(d, meta)
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
