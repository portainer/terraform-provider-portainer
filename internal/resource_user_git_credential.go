package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourcePortainerUserGitCredential() *schema.Resource {
	return &schema.Resource{
		Create: resourcePortainerUserGitCredentialCreate,
		Read:   resourcePortainerUserGitCredentialRead,
		Update: resourcePortainerUserGitCredentialUpdate,
		Delete: resourcePortainerUserGitCredentialDelete,
		Importer: &schema.ResourceImporter{
			State: func(d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
				// Expect ID in format "<user_id>:<credential_id>"
				parts := strings.SplitN(d.Id(), ":", 2)
				if len(parts) != 2 {
					return nil, fmt.Errorf("unexpected format of ID (%q), expected <user_id>:<credential_id>", d.Id())
				}
				userID, err := strconv.Atoi(parts[0])
				if err != nil {
					return nil, fmt.Errorf("invalid user ID: %w", err)
				}
				credentialID, err := strconv.Atoi(parts[1])
				if err != nil {
					return nil, fmt.Errorf("invalid credential ID: %w", err)
				}
				_ = d.Set("user_id", userID)
				d.SetId(fmt.Sprintf("%d:%d", userID, credentialID))
				return []*schema.ResourceData{d}, nil
			},
		},
		Schema: map[string]*schema.Schema{
			"user_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "ID of the user who owns this git credential",
			},
			"credential_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "ID of the git credential",
			},
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  "Name of the git credential",
				ValidateFunc: validation.NoZeroValues,
			},
			"username": {
				Type:        schema.TypeString,
				Required:    true,
				Description: "Username for git authentication",
			},
			"password": {
				Type:        schema.TypeString,
				Required:    true,
				Sensitive:   true,
				Description: "Password or token for git authentication",
			},
			"authorization_type": {
				Type:         schema.TypeInt,
				Optional:     true,
				Default:      0,
				Description:  "Authorization type: 0 = Basic, 1 = Token",
				ValidateFunc: validation.IntBetween(0, 1),
			},
		},
	}
}

func resourcePortainerUserGitCredentialCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	userID := d.Get("user_id").(int)

	payload := map[string]interface{}{
		"name":              d.Get("name").(string),
		"username":          d.Get("username").(string),
		"password":          d.Get("password").(string),
		"authorizationType": d.Get("authorization_type").(int),
	}

	path := fmt.Sprintf("/users/%d/gitcredentials", userID)
	resp, err := client.DoRequest(http.MethodPost, path, nil, payload)
	if err != nil {
		return fmt.Errorf("failed to create user git credential: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create user git credential: HTTP %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		GitCredential struct {
			ID int `json:"id"`
		} `json:"gitCredential"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode user git credential response: %w", err)
	}

	credentialID := result.GitCredential.ID
	d.SetId(fmt.Sprintf("%d:%d", userID, credentialID))
	_ = d.Set("credential_id", credentialID)

	return resourcePortainerUserGitCredentialRead(d, meta)
}

func resourcePortainerUserGitCredentialRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	userID, credentialID, err := parseUserGitCredentialID(d.Id())
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/users/%d/gitcredentials/%d", userID, credentialID)
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to read user git credential: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read user git credential: HTTP %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID                int    `json:"id"`
		Name              string `json:"name"`
		Username          string `json:"username"`
		AuthorizationType int    `json:"authorizationType"`
		UserID            int    `json:"userId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode user git credential response: %w", err)
	}

	_ = d.Set("user_id", userID)
	_ = d.Set("credential_id", credentialID)
	_ = d.Set("name", result.Name)
	_ = d.Set("username", result.Username)
	_ = d.Set("authorization_type", result.AuthorizationType)

	return nil
}

func resourcePortainerUserGitCredentialUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	userID, credentialID, err := parseUserGitCredentialID(d.Id())
	if err != nil {
		return err
	}

	payload := map[string]interface{}{
		"name":              d.Get("name").(string),
		"username":          d.Get("username").(string),
		"password":          d.Get("password").(string),
		"authorizationType": d.Get("authorization_type").(int),
	}

	path := fmt.Sprintf("/users/%d/gitcredentials/%d", userID, credentialID)
	resp, err := client.DoRequest(http.MethodPut, path, nil, payload)
	if err != nil {
		return fmt.Errorf("failed to update user git credential: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update user git credential: HTTP %d - %s", resp.StatusCode, string(body))
	}

	return resourcePortainerUserGitCredentialRead(d, meta)
}

func resourcePortainerUserGitCredentialDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	userID, credentialID, err := parseUserGitCredentialID(d.Id())
	if err != nil {
		return err
	}

	path := fmt.Sprintf("/users/%d/gitcredentials/%d", userID, credentialID)
	resp, err := client.DoRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete user git credential: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete user git credential: HTTP %d - %s", resp.StatusCode, string(body))
	}

	d.SetId("")
	return nil
}

func parseUserGitCredentialID(id string) (int, int, error) {
	parts := strings.SplitN(id, ":", 2)
	if len(parts) != 2 {
		return 0, 0, fmt.Errorf("unexpected format of ID (%q), expected <user_id>:<credential_id>", id)
	}
	userID, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid user ID in composite ID: %w", err)
	}
	credentialID, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, fmt.Errorf("invalid credential ID in composite ID: %w", err)
	}
	return userID, credentialID, nil
}
