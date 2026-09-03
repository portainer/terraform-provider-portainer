package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func dataSourceEndpointRegistries() *schema.Resource {
	return &schema.Resource{
		ReadContext: dataSourceEndpointRegistriesRead,

		Schema: map[string]*schema.Schema{
			"environment_id": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment (endpoint) identifier whose accessible registries are listed.",
			},
			"namespace": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Kubernetes namespace to scope the list to, for an environment where registry access is granted per namespace.",
			},
			"dockerhub_registry_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Identifier of a Docker Hub registry whose pull rate limit should also be fetched. Leave unset to skip that lookup.",
			},
			// Computed attributes
			"registries": {
				Type:        schema.TypeList,
				Computed:    true,
				Description: "Registries this environment can pull from.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"id":             {Type: schema.TypeInt, Computed: true, Description: "Identifier of the registry."},
						"name":           {Type: schema.TypeString, Computed: true, Description: "Name of the registry."},
						"url":            {Type: schema.TypeString, Computed: true, Description: "URL of the registry."},
						"base_url":       {Type: schema.TypeString, Computed: true, Description: "Base URL of the registry, where the type distinguishes it from `url`."},
						"type":           {Type: schema.TypeInt, Computed: true, Description: "Registry type: 1 = Quay, 2 = Azure, 3 = Custom, 4 = GitLab, 5 = ProGet, 6 = DockerHub, 7 = ECR."},
						"authentication": {Type: schema.TypeBool, Computed: true, Description: "Whether the registry requires authentication."},
						"username":       {Type: schema.TypeString, Computed: true, Description: "Username Portainer authenticates with. The password and any access token are deliberately not exposed."},
					},
				},
			},
			"dockerhub_rate_limit": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Docker Hub pull rate limit for the registry named by `dockerhub_registry_id`, zero when that argument is unset.",
			},
			"dockerhub_rate_remaining": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Pulls still available within the current window. Watch this before a large rollout that pulls from Docker Hub.",
			},
		},
	}
}

func dataSourceEndpointRegistriesRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	envID := d.Get("environment_id").(int)

	target := fmt.Sprintf("%s/endpoints/%d/registries", client.Endpoint, envID)
	if v, ok := d.GetOk("namespace"); ok {
		q := url.Values{}
		q.Set("namespace", v.(string))
		target += "?" + q.Encode()
	}

	// The API returns the full registry object, which carries Password and
	// AccessToken. Those are deliberately not mapped: a data source that copies
	// registry credentials into every state file would be a liability, and
	// nothing in Terraform needs them.
	var list []struct {
		ID             int    `json:"Id"`
		Name           string `json:"Name"`
		URL            string `json:"URL"`
		BaseURL        string `json:"BaseURL"`
		Type           int    `json:"Type"`
		Authentication bool   `json:"Authentication"`
		Username       string `json:"Username"`
	}
	if err := doJSON(ctx, client, http.MethodGet, target, nil, &list); err != nil {
		return diag.FromErr(fmt.Errorf("failed to list the registries of environment %d: %w", envID, err))
	}

	registries := make([]map[string]interface{}, len(list))
	for i, r := range list {
		registries[i] = map[string]interface{}{
			"id": r.ID, "name": r.Name, "url": r.URL, "base_url": r.BaseURL,
			"type": r.Type, "authentication": r.Authentication, "username": r.Username,
		}
	}

	limit, remaining := 0, 0
	if v, ok := d.GetOk("dockerhub_registry_id"); ok {
		var status struct {
			Limit     int `json:"limit"`
			Remaining int `json:"remaining"`
		}
		hubURL := fmt.Sprintf("%s/endpoints/%d/dockerhub/%d", client.Endpoint, envID, v.(int))
		if err := doJSON(ctx, client, http.MethodGet, hubURL, nil, &status); err != nil {
			return diag.FromErr(fmt.Errorf("failed to read the Docker Hub rate limit for registry %d: %w", v.(int), err))
		}
		limit, remaining = status.Limit, status.Remaining
	}

	if err := setFields(d, map[string]interface{}{
		"registries":               registries,
		"dockerhub_rate_limit":     limit,
		"dockerhub_rate_remaining": remaining,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(envID) + "/registries")
	return nil
}
