package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceUser() *schema.Resource {
	return &schema.Resource{
		Create: resourceUserCreate,
		Read:   resourceUserRead,
		Delete: resourceUserDelete,
		Update: resourceUserUpdate,

		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},

		Schema: map[string]*schema.Schema{
			"username": {
				Type:     schema.TypeString,
				Required: true,
			},
			"password": {
				Type:      schema.TypeString,
				Optional:  true,
				Sensitive: true,
			},
			"role": {
				Type:     schema.TypeInt,
				Optional: true,
				Default:  2,
			},
			"ldap_user": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
				ForceNew: true,
			},
			"team_id": {
				Type:        schema.TypeInt,
				Optional:    true,
				Description: "Optional Portainer team ID. Only applicable for standard users (role = 2).",
			},
			"generate_api_key": {
				Type:     schema.TypeBool,
				Optional: true,
				Default:  false,
			},
			"api_key_description": {
				Type:     schema.TypeString,
				Optional: true,
				Default:  "terraform-generated-api-key",
			},
			"api_key_raw": {
				Type:      schema.TypeString,
				Computed:  true,
				Sensitive: true,
			},
		},
	}
}

func resourceUserCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	username := d.Get("username").(string)
	password := d.Get("password").(string)
	role := d.Get("role").(int)
	ldapUser := d.Get("ldap_user").(bool)

	if ldapUser && password != "" {
		return fmt.Errorf("cannot set password for LDAP user")
	}
	if !ldapUser && password == "" {
		return fmt.Errorf("password is required for non-LDAP user")
	}

	body := map[string]interface{}{
		"Username": username,
		"Role":     role,
	}
	if !ldapUser {
		body["Password"] = password
	}

	resp, err := client.DoRequest("POST", "/users", nil, body)
	if err != nil {
		return fmt.Errorf("failed to create user: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create user: %s", string(data))
	}

	var result struct {
		ID int `json:"Id"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&result)

	if result.ID == 0 {
		return resourceUserReadByUsername(d, meta)
	}
	d.SetId(strconv.Itoa(result.ID))

	if teamID, ok := d.GetOk("team_id"); ok {
		if role != 2 {
			return fmt.Errorf("team_id can only be used with standard users (role = 2)")
		}

		teamMembership := map[string]interface{}{
			"UserID": result.ID,
			"TeamID": teamID.(int),
			"Role":   2,
		}

		resp, err := client.DoRequest("POST", "/team_memberships", nil, teamMembership)
		if err != nil {
			return fmt.Errorf("failed to assign user to team: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode < 200 || resp.StatusCode >= 300 {
			data, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to assign user to team: %s", string(data))
		}
	}

	if d.Get("generate_api_key").(bool) {
		description := d.Get("api_key_description").(string)
		if password == "" {
			return fmt.Errorf("password must be set to generate API key")
		}
		apiPayload := map[string]interface{}{
			"description": description,
			"password":    password,
		}

		resp, err := client.DoRequest("POST", fmt.Sprintf("/users/%d/tokens", result.ID), nil, apiPayload)
		if err != nil {
			return fmt.Errorf("failed to generate API key: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != 200 {
			data, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to generate API key: %s", string(data))
		}

		var tokenResp struct {
			RawAPIKey string `json:"rawAPIKey"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
			return err
		}
		d.Set("api_key_raw", tokenResp.RawAPIKey)
	}

	return resourceUserRead(d, meta)
}

func resourceUserRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	resp, err := client.DoRequest("GET", fmt.Sprintf("/users/%s", d.Id()), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	} else if resp.StatusCode != 200 {
		return fmt.Errorf("failed to read user")
	}

	var user struct {
		Username string `json:"Username"`
		Role     int    `json:"Role"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&user); err != nil {
		return err
	}

	d.Set("username", user.Username)
	d.Set("role", user.Role)
	return nil
}

func resourceUserReadByUsername(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	resp, err := client.DoRequest("GET", "/users", nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("failed to list users for lookup")
	}

	var users []struct {
		ID       int    `json:"Id"`
		Username string `json:"Username"`
		Role     int    `json:"Role"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&users); err != nil {
		return err
	}

	username := d.Get("username").(string)
	for _, u := range users {
		if u.Username == username {
			d.SetId(strconv.Itoa(u.ID))
			d.Set("role", u.Role)
			return nil
		}
	}

	return fmt.Errorf("user created but not found in user list")
}

func resourceUserUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Id()

	if d.HasChange("password") {
		oldPw, newPw := d.GetChange("password")
		payload := map[string]string{
			"password":    oldPw.(string),
			"newPassword": newPw.(string),
		}
		resp, err := client.DoRequest("PUT", fmt.Sprintf("/users/%s/passwd", id), nil, payload)
		if err != nil {
			return err
		}
		defer resp.Body.Close()

		if resp.StatusCode != 204 {
			data, _ := io.ReadAll(resp.Body)
			return fmt.Errorf("failed to update password: %s", string(data))
		}
	}

	body := map[string]interface{}{
		"username": d.Get("username").(string),
		"role":     d.Get("role").(int),
		"useCache": true,
	}
	resp, err := client.DoRequest("PUT", fmt.Sprintf("/users/%s", id), nil, body)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		data, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update user: %s", string(data))
	}

	return resourceUserRead(d, meta)
}

func resourceUserDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Id()

	if keyID, ok := d.Get("api_key_id").(int); ok && keyID > 0 {
		_, _ = client.DoRequest("DELETE", fmt.Sprintf("/users/%s/tokens/%d", id, keyID), nil, nil)
	}

	resp, err := client.DoRequest("DELETE", fmt.Sprintf("/users/%s", id), nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 || resp.StatusCode == 204 {
		return nil
	}

	data, _ := io.ReadAll(resp.Body)
	return fmt.Errorf("failed to delete user: %s", string(data))
}
