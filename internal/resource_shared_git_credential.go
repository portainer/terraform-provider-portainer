package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourcePortainerSharedGitCredential() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePortainerSharedGitCredentialCreate,
		ReadContext:   resourcePortainerSharedGitCredentialRead,
		UpdateContext: resourcePortainerSharedGitCredentialUpdate,
		DeleteContext: resourcePortainerSharedGitCredentialDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
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

func resourcePortainerSharedGitCredentialCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	payload := map[string]interface{}{
		"name":              d.Get("name").(string),
		"username":          d.Get("username").(string),
		"password":          d.Get("password").(string),
		"authorizationType": d.Get("authorization_type").(int),
	}

	resp, err := client.DoRequest(http.MethodPost, "/cloud/gitcredentials", nil, payload)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to create shared git credential: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to create shared git credential: HTTP %d - %s", resp.StatusCode, string(body)))
	}

	var result struct {
		GitCredential struct {
			ID int `json:"id"`
		} `json:"gitCredential"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode shared git credential response: %w", err))
	}

	d.SetId(strconv.Itoa(result.GitCredential.ID))
	return resourcePortainerSharedGitCredentialRead(ctx, d, meta)
}

func resourcePortainerSharedGitCredentialRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Id()

	path := fmt.Sprintf("/cloud/gitcredentials/%s", id)
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to read shared git credential: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		d.SetId("")
		return nil
	}

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to read shared git credential: HTTP %d - %s", resp.StatusCode, string(body)))
	}

	var result struct {
		ID                int    `json:"id"`
		Name              string `json:"name"`
		Username          string `json:"username"`
		AuthorizationType int    `json:"authorizationType"`
		UserID            int    `json:"userId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to decode shared git credential response: %w", err))
	}

	_ = d.Set("name", result.Name)
	_ = d.Set("username", result.Username)
	_ = d.Set("authorization_type", result.AuthorizationType)
	_ = d.Set("user_id", result.UserID)

	return nil
}

func resourcePortainerSharedGitCredentialUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
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
		return diag.FromErr(fmt.Errorf("failed to update shared git credential: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to update shared git credential: HTTP %d - %s", resp.StatusCode, string(body)))
	}

	return resourcePortainerSharedGitCredentialRead(ctx, d, meta)
}

func resourcePortainerSharedGitCredentialDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Id()

	path := fmt.Sprintf("/cloud/gitcredentials/%s", id)
	resp, err := client.DoRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return diag.FromErr(fmt.Errorf("failed to delete shared git credential: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != http.StatusNotFound {
		body, _ := io.ReadAll(resp.Body)
		return diag.FromErr(fmt.Errorf("failed to delete shared git credential: HTTP %d - %s", resp.StatusCode, string(body)))
	}

	d.SetId("")
	return nil
}
