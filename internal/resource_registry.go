package internal

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
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
			"name":                     {Type: schema.TypeString, Required: true, ValidateFunc: validation.NoZeroValues, Description: "Name of the registry as displayed in Portainer."},
			"url":                      {Type: schema.TypeString, Required: true, ValidateFunc: validation.NoZeroValues, Description: "URL of the registry (e.g. registry hostname). Used by Portainer to push and pull images."},
			"base_url":                 {Type: schema.TypeString, Optional: true, Description: "Optional base URL of the registry used by Portainer for web-based actions and links."},
			"type":                     {Type: schema.TypeInt, Required: true, ForceNew: true, ValidateFunc: validation.IntBetween(1, 8), Description: "Registry type: 1 = Quay, 2 = Azure, 3 = Custom, 4 = GitLab, 5 = ProGet, 6 = DockerHub, 7 = ECR, 8 = GitHub. Changing this value forces resource recreation."},
			"authentication":           {Type: schema.TypeBool, Optional: true, Default: false, Description: "Whether the registry requires authentication. When true, username and password are sent to Portainer."},
			"username":                 {Type: schema.TypeString, Optional: true, Description: "Username used to authenticate against the registry when authentication is enabled."},
			"password":                 {Type: schema.TypeString, Optional: true, Sensitive: true, Description: "Password used to authenticate against the registry when authentication is enabled. Stored in state as a sensitive value."},
			"instance_url":             {Type: schema.TypeString, Optional: true, Description: "Instance URL used by GitLab (type 4) registries to identify the GitLab instance."},
			"aws_region":               {Type: schema.TypeString, Optional: true, Description: "AWS region used by ECR (type 7) registries."},
			"github_use_organisation":  {Type: schema.TypeBool, Optional: true, Default: false, Description: "Whether the GitHub (type 8) registry should be scoped to an organisation rather than a user account."},
			"github_organisation_name": {Type: schema.TypeString, Optional: true, Description: "Name of the GitHub organisation used when github_use_organisation is true (type 8)."},
			"quay_use_organisation":    {Type: schema.TypeBool, Optional: true, Default: false, Description: "Whether the Quay (type 1) registry should be scoped to an organisation rather than a user account."},
			"quay_organisation_name":   {Type: schema.TypeString, Optional: true, Description: "Name of the Quay organisation used when quay_use_organisation is true (type 1)."},
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

	ctx, errBody := withErrorCapture(context.Background())
	params := registries.NewRegistryCreateParams()
	params.SetContext(ctx)
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
		return fmt.Errorf("failed to create registry: %w", decorateSDKError(err, errBody))
	}

	d.SetId(strconv.FormatInt(resp.Payload.ID, 10))
	return resourceRegistryRead(d, meta)
}

func findRegistryByName(client *APIClient, name string) (int, error) {
	ctx, errBody := withErrorCapture(context.Background())
	params := registries.NewRegistryListParams()
	params.SetContext(ctx)
	resp, err := client.Client.Registries.RegistryList(params, client.AuthInfo)
	if err != nil {
		return 0, fmt.Errorf("failed to list registries: %w", decorateSDKError(err, errBody))
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

	ctx, errBody := withErrorCapture(context.Background())
	params := registries.NewRegistryInspectParams()
	params.SetContext(ctx)
	params.ID = id

	resp, err := client.Client.Registries.RegistryInspect(params, client.AuthInfo)
	if err != nil {
		var notFound *registries.RegistryInspectNotFound
		if errors.As(err, &notFound) {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("failed to read registry: %w", decorateSDKError(err, errBody))
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

	ctx, errBody := withErrorCapture(context.Background())
	params := registries.NewRegistryUpdateParams()
	params.SetContext(ctx)
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
		return fmt.Errorf("failed to update registry: %w", decorateSDKError(err, errBody))
	}

	return resourceRegistryRead(d, meta)
}

func resourceRegistryDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	ctx, errBody := withErrorCapture(context.Background())
	params := registries.NewRegistryDeleteParams()
	params.SetContext(ctx)
	params.ID = id

	_, err := client.Client.Registries.RegistryDelete(params, client.AuthInfo)
	if err != nil {
		var notFound *registries.RegistryDeleteNotFound
		if errors.As(err, &notFound) {
			return nil
		}
		// SDK expects 204 but Portainer API returns 200 - treat status 200 as success
		if strings.Contains(err.Error(), "status 200") {
			return nil
		}
		return fmt.Errorf("failed to delete registry: %w", decorateSDKError(err, errBody))
	}
	return nil
}
