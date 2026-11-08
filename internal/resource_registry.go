package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceRegistry() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceRegistryCreate,
		ReadContext:   resourceRegistryRead,
		DeleteContext: resourceRegistryDelete,
		UpdateContext: resourceRegistryUpdate,
		CustomizeDiff: registryAuthenticationDiff,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"name":           {Type: schema.TypeString, Required: true, ValidateFunc: validation.NoZeroValues, Description: "Name of the registry as displayed in Portainer."},
			"url":            {Type: schema.TypeString, Required: true, ValidateFunc: validation.NoZeroValues, Description: "URL of the registry (e.g. registry hostname). Used by Portainer to push and pull images."},
			"base_url":       {Type: schema.TypeString, Optional: true, Description: "Optional base URL of the registry used by Portainer for web-based actions and links."},
			"type":           {Type: schema.TypeInt, Required: true, ForceNew: true, ValidateFunc: validation.IntBetween(1, 8), Description: "Registry type: 1 = Quay, 2 = Azure, 3 = Custom, 4 = GitLab, 5 = ProGet, 6 = DockerHub, 7 = ECR, 8 = GitHub. Changing this value forces resource recreation."},
			"authentication": {Type: schema.TypeBool, Optional: true, Computed: true, Description: "Whether the registry requires authentication. When true, username and password are sent to Portainer. Defaults to true when `username` is set, false otherwise."},
			"username":       {Type: schema.TypeString, Optional: true, Description: "Username used to authenticate against the registry when authentication is enabled."},
			"password":       {Type: schema.TypeString, Optional: true, Sensitive: true, ConflictsWith: []string{"password_wo"}, Description: "Password used to authenticate against the registry when authentication is enabled. Stored in state as a sensitive value; use `password_wo` to keep it out of state."},
			"password_wo": {
				Type:          schema.TypeString,
				Optional:      true,
				Sensitive:     true,
				WriteOnly:     true,
				ConflictsWith: []string{"password"},
				RequiredWith:  []string{"password_wo_version"},
				Description:   "Write-only registry password (supports ephemeral values; never stored in Terraform state). Sent on create and whenever `password_wo_version` changes. Requires Terraform 1.11 or later.",
			},
			"password_wo_version": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Version of `password_wo`. Increment it to send a new password; the registry is updated in place.",
			},
			"instance_url":             {Type: schema.TypeString, Optional: true, Description: "Instance URL used by GitLab (type 4) registries to identify the GitLab instance."},
			"aws_region":               {Type: schema.TypeString, Optional: true, Description: "AWS region used by ECR (type 7) registries."},
			"github_use_organisation":  {Type: schema.TypeBool, Optional: true, Default: false, Description: "Whether the GitHub (type 8) registry should be scoped to an organisation rather than a user account."},
			"github_organisation_name": {Type: schema.TypeString, Optional: true, Description: "Name of the GitHub organisation used when github_use_organisation is true (type 8)."},
			"quay_use_organisation":    {Type: schema.TypeBool, Optional: true, Default: false, Description: "Whether the Quay (type 1) registry should be scoped to an organisation rather than a user account."},
			"quay_organisation_name":   {Type: schema.TypeString, Optional: true, Description: "Name of the Quay organisation used when quay_use_organisation is true (type 1)."},
		},
	}
}

func resourceRegistryCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	existingID, err := findRegistryByName(ctx, client, name)
	if err != nil {
		return diag.FromErr(err)
	}
	if existingID != 0 {
		d.SetId(strconv.Itoa(existingID))
		return resourceRegistryUpdate(ctx, d, meta)
	}

	registryType := int64(d.Get("type").(int))
	url := d.Get("url").(string)
	baseURL := d.Get("base_url").(string)
	auth := registryAuthentication(d)

	body := registryPayload{
		Name:           name,
		Type:           &registryType,
		URL:            url,
		BaseURL:        baseURL,
		Authentication: auth,
	}

	if auth {
		password, err := registryPassword(d, true)
		if err != nil {
			return diag.FromErr(err)
		}
		body.Username = d.Get("username").(string)
		body.Password = password
	}

	switch registryType {
	case 1: // Quay.io
		body.Quay = &registryOrganisation{
			UseOrganisation:  d.Get("quay_use_organisation").(bool),
			OrganisationName: d.Get("quay_organisation_name").(string),
		}
	case 4: // GitLab
		body.Gitlab = &registryGitlab{
			InstanceURL: d.Get("instance_url").(string),
		}
	case 7: // AWS ECR
		body.Ecr = &registryEcr{
			Region: d.Get("aws_region").(string),
		}
	case 8: // GitHub
		body.Github = &registryOrganisation{
			UseOrganisation:  d.Get("github_use_organisation").(bool),
			OrganisationName: d.Get("github_organisation_name").(string),
		}
	}

	var created portainerRegistry
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+"/registries", body, &created); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create registry: %w", err))
	}

	d.SetId(strconv.FormatInt(created.ID, 10))
	return resourceRegistryRead(ctx, d, meta)
}

func findRegistryByName(ctx context.Context, client *APIClient, name string) (int, error) {
	registryList, err := listRegistries(ctx, client)
	if err != nil {
		return 0, err
	}

	for _, r := range registryList {
		if r.Name == name {
			return int(r.ID), nil
		}
	}

	return 0, nil
}

func resourceRegistryRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	registry, err := getRegistry(ctx, client, id)
	if err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read registry: %w", err))
	}

	if err := d.Set("name", registry.Name); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("url", registry.URL); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("base_url", registry.BaseURL); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("type", int(registry.Type)); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("authentication", registry.Authentication); err != nil {
		return diag.FromErr(err)
	}
	if err := d.Set("username", registry.Username); err != nil {
		return diag.FromErr(err)
	}

	if registry.Github != nil {
		if err := d.Set("github_use_organisation", registry.Github.UseOrganisation); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("github_organisation_name", registry.Github.OrganisationName); err != nil {
			return diag.FromErr(err)
		}
	}
	if registry.Quay != nil {
		if err := d.Set("quay_use_organisation", registry.Quay.UseOrganisation); err != nil {
			return diag.FromErr(err)
		}
		if err := d.Set("quay_organisation_name", registry.Quay.OrganisationName); err != nil {
			return diag.FromErr(err)
		}
	}
	if registry.Gitlab != nil {
		if err := d.Set("instance_url", registry.Gitlab.InstanceURL); err != nil {
			return diag.FromErr(err)
		}
	}
	if registry.Ecr != nil {
		if err := d.Set("aws_region", registry.Ecr.Region); err != nil {
			return diag.FromErr(err)
		}
	}

	return nil
}

func resourceRegistryUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	name := d.Get("name").(string)
	url := d.Get("url").(string)
	auth := registryAuthentication(d)

	password, err := registryPassword(d, d.HasChange("password_wo_version"))
	if err != nil {
		return diag.FromErr(err)
	}

	body := registryPayload{
		Name:           name,
		URL:            url,
		BaseURL:        d.Get("base_url").(string),
		Authentication: auth,
		Username:       d.Get("username").(string),
		// An empty password is left out, so Portainer keeps the stored one.
		Password: password,
	}

	registryType := d.Get("type").(int)
	switch registryType {
	case 1: // Quay.io
		body.Quay = &registryOrganisation{
			UseOrganisation:  d.Get("quay_use_organisation").(bool),
			OrganisationName: d.Get("quay_organisation_name").(string),
		}
	// The update payload has no GitLab settings and no type.
	case 7: // AWS ECR
		body.Ecr = &registryEcr{
			Region: d.Get("aws_region").(string),
		}
	case 8: // GitHub
		body.Github = &registryOrganisation{
			UseOrganisation:  d.Get("github_use_organisation").(bool),
			OrganisationName: d.Get("github_organisation_name").(string),
		}
	}

	if err := doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/registries/%d", client.Endpoint, id), body, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update registry: %w", err))
	}

	return resourceRegistryRead(ctx, d, meta)
}

// registryAuthentication is the configured authentication, or whether a
// username is set. The documented Azure, GitLab, ProGet and Docker Hub
// examples set credentials without authentication = true, which used to
// create those registries without them.
func registryAuthentication(d *schema.ResourceData) bool {
	if v := configuredBool(d, "authentication"); v != nil {
		return *v
	}
	return d.Get("username").(string) != ""
}

// registryAuthenticationDiff plans the authentication registryAuthentication
// will send when it is not configured.
func registryAuthenticationDiff(_ context.Context, d *schema.ResourceDiff, _ interface{}) error {
	raw := d.GetRawConfig()
	if raw.IsNull() || !raw.GetAttr("authentication").IsNull() || !d.NewValueKnown("username") {
		return nil
	}
	want := d.Get("username").(string) != ""
	if d.Get("authentication").(bool) == want {
		return nil
	}
	return d.SetNew("authentication", want)
}

func resourceRegistryDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	if err := doJSON(ctx, client, http.MethodDelete, fmt.Sprintf("%s/registries/%d", client.Endpoint, id), nil, nil); err != nil {
		if isAPINotFound(err) {
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to delete registry: %w", err))
	}
	return nil
}

// registryPassword returns the password to send: password_wo when sendWO is
// set (on create, or when password_wo_version changed), otherwise password.
func registryPassword(d *schema.ResourceData, sendWO bool) (string, error) {
	if sendWO && d.Get("password_wo_version").(int) != 0 {
		wo, err := writeOnlyString(d, "password_wo")
		if err != nil || wo != "" {
			return wo, err
		}
	}
	return d.Get("password").(string), nil
}

// registryPayload is the POST /registries and PUT /registries/{id} body. Type
// and GitLab settings are only part of the create payload.
type registryPayload struct {
	Name           string                `json:"name"`
	Type           *int64                `json:"type,omitempty"`
	URL            string                `json:"url"`
	BaseURL        string                `json:"baseURL,omitempty"`
	Authentication bool                  `json:"authentication"`
	Username       string                `json:"username,omitempty"`
	Password       string                `json:"password,omitempty"`
	Quay           *registryOrganisation `json:"quay,omitempty"`
	Github         *registryOrganisation `json:"github,omitempty"`
	Gitlab         *registryGitlab       `json:"gitlab,omitempty"`
	Ecr            *registryEcr          `json:"ecr,omitempty"`
}

type registryOrganisation struct {
	UseOrganisation  bool   `json:"UseOrganisation,omitempty"`
	OrganisationName string `json:"OrganisationName,omitempty"`
}

type registryGitlab struct {
	InstanceURL string `json:"InstanceURL,omitempty"`
}

type registryEcr struct {
	Region string `json:"Region,omitempty"`
}

type portainerRegistry struct {
	ID               int64                             `json:"Id"`
	Name             string                            `json:"Name"`
	Type             int64                             `json:"Type"`
	URL              string                            `json:"URL"`
	BaseURL          string                            `json:"BaseURL"`
	Authentication   bool                              `json:"Authentication"`
	Username         string                            `json:"Username"`
	Quay             *registryOrganisation             `json:"Quay"`
	Github           *registryOrganisation             `json:"Github"`
	Gitlab           *registryGitlab                   `json:"Gitlab"`
	Ecr              *registryEcr                      `json:"Ecr"`
	RegistryAccesses map[string]registryAccessPolicies `json:"RegistryAccesses"`
}

// getRegistry returns the registry (GET /registries/{id}).
func getRegistry(ctx context.Context, client *APIClient, id int64) (*portainerRegistry, error) {
	var registry portainerRegistry
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/registries/%d", client.Endpoint, id), nil, &registry); err != nil {
		return nil, err
	}
	return &registry, nil
}

// listRegistries returns every registry (GET /registries).
func listRegistries(ctx context.Context, client *APIClient) ([]portainerRegistry, error) {
	var registryList []portainerRegistry
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/registries", nil, &registryList); err != nil {
		return nil, fmt.Errorf("failed to list registries: %w", err)
	}
	return registryList, nil
}
