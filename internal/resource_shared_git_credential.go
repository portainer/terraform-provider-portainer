package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourcePortainerSharedGitCredential() *schema.Resource {
	return &schema.Resource{
		Create: resourcePortainerSharedGitCredentialCreate,
		Read:   resourcePortainerSharedGitCredentialRead,
		Update: resourcePortainerSharedGitCredentialUpdate,
		Delete: resourcePortainerSharedGitCredentialDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"name": {
				Type:         schema.TypeString,
				Required:     true,
				Description:  "Name of the shared git credential",
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
			"user_id": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "User ID of the credential owner",
			},
		},
	}
}

func resourcePortainerSharedGitCredentialCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	payload := map[string]interface{}{
		"name":              d.Get("name").(string),
		"username":          d.Get("username").(string),
		"password":          d.Get("password").(string),
		"authorizationType": d.Get("authorization_type").(int),
	}

	resp, err := client.DoRequest(http.MethodPost, "/cloud/gitcredentials", nil, payload)
	if err != nil {
		return fmt.Errorf("failed to create shared git credential: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to create shared git credential: HTTP %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID int `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode shared git credential response: %w", err)
	}

	d.SetId(strconv.Itoa(result.ID))
	return resourcePortainerSharedGitCredentialRead(d, meta)
}

func resourcePortainerSharedGitCredentialRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Id()

	path := fmt.Sprintf("/cloud/gitcredentials/%s", id)
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to read shared git credential: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to read shared git credential: HTTP %d - %s", resp.StatusCode, string(body))
	}

	var result struct {
		ID                int    `json:"id"`
		Name              string `json:"name"`
		Username          string `json:"username"`
		AuthorizationType int    `json:"authorizationType"`
		UserID            int    `json:"userId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode shared git credential response: %w", err)
	}

	_ = d.Set("name", result.Name)
	_ = d.Set("username", result.Username)
	_ = d.Set("authorization_type", result.AuthorizationType)
	_ = d.Set("user_id", result.UserID)

	return nil
}

func resourcePortainerSharedGitCredentialUpdate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Id()

	payload := map[string]interface{}{
		"name":              d.Get("name").(string),
		"username":          d.Get("username").(string),
		"password":          d.Get("password").(string),
		"authorizationType": d.Get("authorization_type").(int),
	}

	path := fmt.Sprintf("/cloud/gitcredentials/%s", id)
	resp, err := client.DoRequest(http.MethodPut, path, nil, payload)
	if err != nil {
		return fmt.Errorf("failed to update shared git credential: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to update shared git credential: HTTP %d - %s", resp.StatusCode, string(body))
	}

	return resourcePortainerSharedGitCredentialRead(d, meta)
}

func resourcePortainerSharedGitCredentialDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)
	id := d.Id()

	path := fmt.Sprintf("/cloud/gitcredentials/%s", id)
	resp, err := client.DoRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete shared git credential: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete shared git credential: HTTP %d - %s", resp.StatusCode, string(body))
	}

	d.SetId("")
	return nil
}
