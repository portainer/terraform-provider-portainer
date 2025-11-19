package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
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
			"type": {
				Type:        schema.TypeInt,
				Required:    true,
				Description: "Environment type: 1 = Docker, 2 = Agent, 3 = Azure, 4 = Edge Agent, 5 = Kubernetes",
				ValidateFunc: func(val interface{}, key string) (warns []string, errs []error) {
					t := val.(int)
					if t < 1 || t > 5 {
						errs = append(errs, fmt.Errorf("%q must be between 1 and 5", key))
					}
					return
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

	var requestBody bytes.Buffer
	writer := multipart.NewWriter(&requestBody)

	_ = writer.WriteField("Name", d.Get("name").(string))
	_ = writer.WriteField("URL", d.Get("environment_address").(string))
	_ = writer.WriteField("EndpointCreationType", strconv.Itoa(d.Get("type").(int)))
	_ = writer.WriteField("GroupID", strconv.Itoa(d.Get("group_id").(int)))
	_ = writer.WriteField("TLS", strconv.FormatBool(d.Get("tls_enabled").(bool)))
	_ = writer.WriteField("TLSSkipVerify", strconv.FormatBool(d.Get("tls_skip_verify").(bool)))
	_ = writer.WriteField("TLSSkipClientVerify", strconv.FormatBool(d.Get("tls_skip_client_verify").(bool)))

	tlsEnabled := d.Get("tls_enabled").(bool)
	tlsSkipVerify := d.Get("tls_skip_verify").(bool)
	if tlsEnabled && !tlsSkipVerify {
		if v, ok := d.GetOk("tls_ca_cert"); ok && v.(string) != "" {
			part, err := writer.CreateFormFile("TLSCACertFile", "ca.pem")
			if err != nil {
				return fmt.Errorf("failed to create TLSCACertFile form file: %w", err)
			}
			if _, err := io.Copy(part, bytes.NewReader([]byte(v.(string)))); err != nil {
				return fmt.Errorf("failed to write TLSCACertFile content: %w", err)
			}
		}

		if v, ok := d.GetOk("tls_cert"); ok && v.(string) != "" {
			part, err := writer.CreateFormFile("TLSCertFile", "cert.pem")
			if err != nil {
				return fmt.Errorf("failed to create TLSCertFile form file: %w", err)
			}
			if _, err := io.Copy(part, bytes.NewReader([]byte(v.(string)))); err != nil {
				return fmt.Errorf("failed to write TLSCertFile content: %w", err)
			}
		}

		if v, ok := d.GetOk("tls_key"); ok && v.(string) != "" {
			part, err := writer.CreateFormFile("TLSKeyFile", "key.pem")
			if err != nil {
				return fmt.Errorf("failed to create TLSKeyFile form file: %w", err)
			}
			if _, err := io.Copy(part, bytes.NewReader([]byte(v.(string)))); err != nil {
				return fmt.Errorf("failed to write TLSKeyFile content: %w", err)
			}
		}
	}

	if v, ok := d.GetOk("tag_ids"); ok {
		tagIds := v.([]interface{})
		formatted := "["
		for i, id := range tagIds {
			if i > 0 {
				formatted += ","
			}
			formatted += fmt.Sprintf("%d", id.(int))
		}
		formatted += "]"
		_ = writer.WriteField("TagIds", formatted)
	}

	writer.Close()

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/endpoints", client.Endpoint), &requestBody)
	if err != nil {
		return err
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create environment: %s", string(data))
	}

	var result struct {
		ID      int    `json:"Id"`
		EdgeKey string `json:"EdgeKey,omitempty"`
		EdgeID  string `json:"EdgeID,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return err
	}

	d.SetId(strconv.Itoa(result.ID))

	// Optional logging
	fmt.Printf("Created environment with ID: %d\n", result.ID)
	if result.EdgeKey != "" {
		fmt.Printf("EdgeKey: %s\n", result.EdgeKey)
		_ = d.Set("edge_key", result.EdgeKey)
	}
	if result.EdgeID != "" {
		fmt.Printf("EdgeID: %s\n", result.EdgeID)
		_ = d.Set("edge_id", result.EdgeID)
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

func resourceEnvironmentRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	req, err := http.NewRequest("GET", fmt.Sprintf("%s/endpoints/%s", client.Endpoint, d.Id()), nil)
	if err != nil {
		return err
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	} else if resp.StatusCode != 200 {
		return fmt.Errorf("failed to read environment")
	}

	var env struct {
		Name      string `json:"Name"`
		Type      int    `json:"Type"`
		URL       string `json:"URL"`
		PublicURL string `json:"PublicURL"`
		GroupID   int    `json:"GroupId"`
		TagIds    []int  `json:"TagIds"`
		EdgeKey   string `json:"EdgeKey,omitempty"`
		EdgeID    string `json:"EdgeID,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&env); err != nil {
		return err
	}

	d.Set("name", env.Name)
	d.Set("type", env.Type)
	d.Set("group_id", env.GroupID)
	d.Set("tag_ids", env.TagIds)

	if env.Type == 1 {
		d.Set("environment_address", env.URL)
	} else {
		d.Set("environment_address", env.PublicURL)
	}

	if env.EdgeKey != "" {
		d.Set("edge_key", env.EdgeKey)
	}
	if env.EdgeID != "" {
		d.Set("edge_id", env.EdgeID)
	}

	return nil
}

func resourceEnvironmentUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	id := d.Id()

	payload := map[string]interface{}{
		"name":      d.Get("name").(string),
		"url":       d.Get("environment_address").(string),
		"publicURL": d.Get("environment_address").(string),
		"groupID":   d.Get("group_id").(int),
		"tagIDs":    d.Get("tag_ids").([]interface{}),
	}

	if v, ok := d.GetOk("user_access_policies"); ok {
		policies := map[string]map[string]int{}
		for userID, role := range v.(map[string]interface{}) {
			policies[userID] = map[string]int{"RoleId": role.(int)}
		}
		payload["userAccessPolicies"] = policies
	}

	if v, ok := d.GetOk("team_access_policies"); ok {
		policies := map[string]map[string]int{}
		for teamID, role := range v.(map[string]interface{}) {
			policies[teamID] = map[string]int{"RoleId": role.(int)}
		}
		payload["teamAccessPolicies"] = policies
	}

	jsonBody, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/endpoints/%s", client.Endpoint, id), bytes.NewBuffer(jsonBody))
	if err != nil {
		return err
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update environment: %s", string(data))
	}

	return resourceEnvironmentRead(d, meta)
}

func resourceEnvironmentDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	req, err := http.NewRequest("DELETE", fmt.Sprintf("%s/endpoints/%s", client.Endpoint, d.Id()), nil)
	if err != nil {
		return err
	}
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 || resp.StatusCode == 204 {
		return nil
	}

	data, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("failed to delete environment: %s", string(data))
}
