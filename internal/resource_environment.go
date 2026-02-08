package internal

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/go-openapi/runtime"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/client-api-go/v2/pkg/client/endpoints"
	"github.com/portainer/client-api-go/v2/pkg/models"
)

func resourceEnvironment() *schema.Resource {
	return &schema.Resource{
		Create: resourceEnvironmentCreate,
		Read:   resourceEnvironmentRead,
		Delete: resourceEnvironmentDelete,
		Update: resourceEnvironmentUpdate,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:     schema.TypeString,
				Required: true,
			},
			"environment_address": {
				Type:     schema.TypeString,
				Required: true,
			},
			"public_ip": {
				Type:        schema.TypeString,
				Optional:    true,
				Description: "Public IP/URL used by Portainer for Published Ports (maps to PublicURL).",
			},
			"type": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment type: 1 = Docker, 2 = Agent, 3 = Azure, 4 = Edge Agent, 5 = Kubernetes, 6 = Kubernetes via agent, 7 = Kubernetes Edge Agent",
				ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
					t := val.(int)
					if t < 1 || t > 7 {
						errs = append(errs, fmt.Errorf("%q must be between 1 and 7", key))
					}
					return
				},
				DiffSuppressFunc: func(k, old, new string, d *schema.ResourceData) bool {
					// Portainer converts Edge Agent type 4 to Kubernetes Edge Agent type 7
					// after agent connection. Suppress this expected drift.
					if old == "7" && new == "4" {
						return true
					}
					return false
				},
			},
			"group_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Default:     1,
				Description: "ID of the Portainer endpoint group. Default is 1 (Unassigned).",
			},
			"tag_ids": {
				Type:        schema.TypeList,
				Optional:    true,
				Elem:        &schema.Schema{Type: schema.TypeInt},
				Description: "List of tag IDs to assign to the environment.",
			},
			"tls_enabled": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"tls_skip_verify": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"tls_ca_cert": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
			},
			"tls_cert": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
			},
			"tls_key": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
			},
			"tls_skip_client_verify": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  true,
			},
			"edge_key": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"edge_id": {
				Type:     schema.TypeString,
				Computed: true,
			},
			"user_access_policies": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeInt,
				},
				Description: "Map of user IDs to role IDs (e.g. userID -> roleID)",
			},
			"team_access_policies": {
				Type:     schema.TypeMap,
				Optional: true,
				Elem: &schema.Schema{
					Type: schema.TypeInt,
				},
				Description: "Map of team IDs to role IDs (e.g. teamID -> roleID)",
			},
		},
	}
}

func resourceEnvironmentCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	name := d.Get("name").(string)

	if existingID, err := findExistingEnvironmentByName(client, name); err != nil {
		return fmt.Errorf("failed to check for existing environment: %w", err)
	} else if existingID != 0 {
		d.SetId(strconv.Itoa(existingID))
		return resourceEnvironmentUpdate(d, meta)
	}

	envType := d.Get("type").(int)
	endpointCreationType := int64(envType)
	if envType == 6 {
		endpointCreationType = 2
	}

	params := endpoints.NewEndpointCreateParams()
	params.SetName(name)
	params.SetEndpointCreationType(endpointCreationType)

	url := strings.TrimSpace(d.Get("environment_address").(string))
	params.SetURL(&url)

	groupID := int64(d.Get("group_id").(int))
	params.SetGroupID(&groupID)

	if v, ok := d.GetOk("public_ip"); ok && v.(string) != "" {
		purl := v.(string)
		params.SetPublicURL(&purl)
	}

	if v, ok := d.GetOk("tag_ids"); ok {
		tagIDs := []int64{}
		for _, id := range v.([]interface{}) {
			tagIDs = append(tagIDs, int64(id.(int)))
		}
		params.SetTagIds(tagIDs)
	}

	tlsEnabled := d.Get("tls_enabled").(bool)
	params.SetTLS(&tlsEnabled)
	tlsSkipVerify := d.Get("tls_skip_verify").(bool)
	params.SetTLSSkipVerify(&tlsSkipVerify)
	tlsSkipClientVerify := d.Get("tls_skip_client_verify").(bool)
	params.SetTLSSkipClientVerify(&tlsSkipClientVerify)

	if tlsEnabled && !tlsSkipVerify {
		if v, ok := d.GetOk("tls_ca_cert"); ok && v.(string) != "" {
			params.SetTLSCACertFile(runtime.NamedReader("ca.pem", strings.NewReader(v.(string))))
		}
		if v, ok := d.GetOk("tls_cert"); ok && v.(string) != "" {
			params.SetTLSCertFile(runtime.NamedReader("cert.pem", strings.NewReader(v.(string))))
		}
		if v, ok := d.GetOk("tls_key"); ok && v.(string) != "" {
			params.SetTLSKeyFile(runtime.NamedReader("key.pem", strings.NewReader(v.(string))))
		}
	}

	resp, err := client.Client.Endpoints.EndpointCreate(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to create environment: %w", err)
	}

	d.SetId(strconv.FormatInt(resp.Payload.ID, 10))

	if resp.Payload.EdgeKey != "" {
		_ = d.Set("edge_key", resp.Payload.EdgeKey)
	}
	if resp.Payload.EdgeID != "" {
		_ = d.Set("edge_id", resp.Payload.EdgeID)
	}

	if _, ok := d.GetOk("user_access_policies"); ok {
		if err := resourceEnvironmentUpdate(d, meta); err != nil {
			return fmt.Errorf("failed to apply user access policies after creation: %w", err)
		}
	}
	if _, ok := d.GetOk("team_access_policies"); ok {
		if err := resourceEnvironmentUpdate(d, meta); err != nil {
			return fmt.Errorf("failed to apply team access policies after creation: %w", err)
		}
	}

	return resourceEnvironmentRead(d, meta)
}

func findExistingEnvironmentByName(client *APIClient, name string) (int, error) {
	params := endpoints.NewEndpointListParams()
	resp, err := client.Client.Endpoints.EndpointList(params, client.AuthInfo)
	if err != nil {
		return 0, fmt.Errorf("failed to list environments: %w", err)
	}

	for _, e := range resp.Payload {
		if e.Name == name {
			return int(e.ID), nil
		}
	}
	return 0, nil
}

func resourceEnvironmentRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	params := endpoints.NewEndpointInspectParams()
	params.ID = id

	resp, err := client.Client.Endpoints.EndpointInspect(params, client.AuthInfo)
	if err != nil {
		if _, ok := err.(*endpoints.EndpointInspectNotFound); ok {
			d.SetId("")
			return nil
		}
		return fmt.Errorf("failed to read environment: %w", err)
	}

	d.Set("name", resp.Payload.Name)
	d.Set("type", int(resp.Payload.Type))
	d.Set("group_id", int(resp.Payload.GroupID))
	d.Set("edge_id", resp.Payload.EdgeID)
	d.Set("edge_key", resp.Payload.EdgeKey)
	d.Set("environment_address", resp.Payload.URL)

	if resp.Payload.PublicURL != "" {
		d.Set("public_ip", resp.Payload.PublicURL)
	} else {
		d.Set("public_ip", "")
	}

	tagIDs := []int{}
	for _, tid := range resp.Payload.TagIds {
		tagIDs = append(tagIDs, int(tid))
	}
	d.Set("tag_ids", tagIDs)

	return nil
}

func resourceEnvironmentUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	params := endpoints.NewEndpointUpdateParams()
	params.ID = id
	params.Body = &models.EndpointsEndpointUpdatePayload{
		Name:                d.Get("name").(string),
		URL:                 d.Get("environment_address").(string),
		GroupID:             int64(d.Get("group_id").(int)),
		TLS:                 d.Get("tls_enabled").(bool),
		TlsskipVerify:       d.Get("tls_skip_verify").(bool),
		TlsskipClientVerify: d.Get("tls_skip_client_verify").(bool),
	}

	if v, ok := d.GetOk("public_ip"); ok && v.(string) != "" {
		params.Body.PublicURL = v.(string)
	} else {
		params.Body.PublicURL = d.Get("environment_address").(string)
	}

	if v, ok := d.GetOk("tag_ids"); ok {
		tagIDs := []int64{}
		for _, tid := range v.([]interface{}) {
			tagIDs = append(tagIDs, int64(tid.(int)))
		}
		params.Body.TagIDs = tagIDs
	}

	if v, ok := d.GetOk("user_access_policies"); ok {
		policies := make(models.PortainerUserAccessPolicies)
		for userID, role := range v.(map[string]interface{}) {
			policies[userID] = models.PortainerAccessPolicy{RoleID: int64(role.(int))}
		}
		params.Body.UserAccessPolicies = policies
	}

	if v, ok := d.GetOk("team_access_policies"); ok {
		policies := make(models.PortainerTeamAccessPolicies)
		for teamID, role := range v.(map[string]interface{}) {
			policies[teamID] = models.PortainerAccessPolicy{RoleID: int64(role.(int))}
		}
		params.Body.TeamAccessPolicies = policies
	}

	_, err := client.Client.Endpoints.EndpointUpdate(params, client.AuthInfo)
	if err != nil {
		return fmt.Errorf("failed to update environment: %w", err)
	}

	return resourceEnvironmentRead(d, meta)
}

func resourceEnvironmentDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id, _ := strconv.ParseInt(d.Id(), 10, 64)

	params := endpoints.NewEndpointDeleteParams()
	params.ID = id

	_, err := client.Client.Endpoints.EndpointDelete(params, client.AuthInfo)
	if err != nil {
		if _, ok := err.(*endpoints.EndpointDeleteNotFound); ok {
			return nil
		}
		return fmt.Errorf("failed to delete environment: %w", err)
	}

	return nil
}
