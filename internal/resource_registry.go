package internal

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/registries"
	"github.com/portainer/client-api-go/v2/pkg/models"
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
			"name":                     {Type: schema.TypeString, Required: true},
			"url":                      {Type: schema.TypeString, Required: true},
			"base_url":                 {Type: schema.TypeString, Optional: true},
			"type":                     {Type: schema.TypeInt, Required: true, ForceNew: true},
			"authentication":           {Type: schema.TypeBool, Optional: true, Default: false},
			"username":                 {Type: schema.TypeString, Optional: true},
			"password":                 {Type: schema.TypeString, Optional: true, Sensitive: true},
			"instance_url":             {Type: schema.TypeString, Optional: true},
			"aws_region":               {Type: schema.TypeString, Optional: true},
			"github_use_organisation":  {Type: schema.TypeBool, Optional: true, Default: false},
			"github_organisation_name": {Type: schema.TypeString, Optional: true},
			"quay_use_organisation":    {Type: schema.TypeBool, Optional: true, Default: false},
			"quay_organisation_name":   {Type: schema.TypeString, Optional: true},
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

	registryType := int64(d.Get("type").(int))
	url := d.Get("url").(string)
	baseURL := d.Get("base_url").(string)
	auth := d.Get("authentication").(bool)

	params := registries.NewRegistryCreateParams()
	params.Body = &models.RegistriesRegistryCreatePayload{
		Name:           &name,
		Type:           &registryType,
		URL:            &url,
		BaseURL:        baseURL,
		Authentication: &auth,
	}

	if auth {
		params.Body.Username = d.Get("username").(string)
		params.Body.Password = d.Get("password").(string)
	}

	switch registryType {
	case 1: // Quay.io
		params.Body.Quay = &models.PortainerQuayRegistryData{
			UseOrganisation:  d.Get("quay_use_organisation").(bool),
			OrganisationName: d.Get("quay_organisation_name").(string),
		}
	case 4: // GitLab
		params.Body.Gitlab = &models.PortainerGitlabRegistryData{
			InstanceURL: d.Get("instance_url").(string),
		}
	case 7: // AWS ECR
		params.Body.Ecr = &models.PortainerEcrData{
			Region: d.Get("aws_region").(string),
		}
	case 8: // GitHub
		params.Body.Github = &models.PortainereeGithubRegistryData{
			UseOrganisation:  d.Get("github_use_organisation").(bool),
			OrganisationName: d.Get("github_organisation_name").(string),
		}
	}

	resp, err := client.Client.Registries.RegistryCreate(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to create registry: %w", err)
	}

	d.SetId(strconv.FormatInt(resp.Payload.ID, 10))
	return resourceRegistryRead(d, meta)
}

func findRegistryByName(client *APIClient, name string) (int, error) {
	params := registries.NewRegistryListParams()
	resp, err := client.Client.Registries.RegistryList(params, client.AuthInfo)
	if err != nil {
		return 0, fmt.Errorf("failed to list registries: %w", err)
	}

	for _, r := range resp.Payload {
		if r.Name == name {
			return int(r.ID), nil
		}
	}

	return 0, nil
}

func resourceRegistryRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	params := registries.NewRegistryInspectParams()
	params.ID = id

	resp, err := client.Client.Registries.RegistryInspect(params, client.AuthInfo)
	if err != nil {
		if _, ok := err.(*registries.RegistryInspectNotFound); ok {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("failed to read registry: %w", err)
	}

	d.Set("name", resp.Payload.Name)
	d.Set("url", resp.Payload.URL)
	d.Set("base_url", resp.Payload.BaseURL)
	d.Set("type", int(resp.Payload.Type))
	d.Set("authentication", resp.Payload.Authentication)
	d.Set("username", resp.Payload.Username)

	if resp.Payload.Github != nil {
		d.Set("github_use_organisation", resp.Payload.Github.UseOrganisation)
		d.Set("github_organisation_name", resp.Payload.Github.OrganisationName)
	}
	if resp.Payload.Quay != nil {
		d.Set("quay_use_organisation", resp.Payload.Quay.UseOrganisation)
		d.Set("quay_organisation_name", resp.Payload.Quay.OrganisationName)
	}
	if resp.Payload.Gitlab != nil {
		d.Set("instance_url", resp.Payload.Gitlab.InstanceURL)
	}
	if resp.Payload.Ecr != nil {
		d.Set("aws_region", resp.Payload.Ecr.Region)
	}

	return nil
}

func resourceRegistryUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	name := d.Get("name").(string)
	url := d.Get("url").(string)
	auth := d.Get("authentication").(bool)

	params := registries.NewRegistryUpdateParams()
	params.ID = id
	params.Body = &models.RegistriesRegistryUpdatePayload{
		Name:           &name,
		URL:            &url,
		BaseURL:        d.Get("base_url").(string),
		Authentication: &auth,
		Username:       d.Get("username").(string),
		Password:       d.Get("password").(string),
	}

	registryType := d.Get("type").(int)
	switch registryType {
	case 1: // Quay.io
		params.Body.Quay = &models.PortainerQuayRegistryData{
			UseOrganisation:  d.Get("quay_use_organisation").(bool),
			OrganisationName: d.Get("quay_organisation_name").(string),
		}
	// Note: According to SDK documentation, Gitlab might not be present in UpdatePayload.
	// We will attempt to use reflection or check for it if needed, but for now we follow the SDK.
	case 7: // AWS ECR
		params.Body.Ecr = &models.PortainerEcrData{
			Region: d.Get("aws_region").(string),
		}
	case 8: // GitHub
		params.Body.Github = &models.PortainereeGithubRegistryData{
			UseOrganisation:  d.Get("github_use_organisation").(bool),
			OrganisationName: d.Get("github_organisation_name").(string),
		}
	}

	_, err := client.Client.Registries.RegistryUpdate(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to update registry: %w", err)
	}

	return resourceRegistryRead(d, meta)
}

func resourceRegistryDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	params := registries.NewRegistryDeleteParams()
	params.ID = id

	_, err := client.Client.Registries.RegistryDelete(params, client.AuthInfo)
	if err != nil {
		if _, ok := err.(*registries.RegistryDeleteNotFound); ok {
			return nil
		}
		// SDK expects 204 but Portainer API returns 200 - treat status 200 as success
		if strings.Contains(err.Error(), "status 200") {
			return nil
		}
		return fmt.Errorf("failed to delete registry: %w", err)
	}
	return nil
}
