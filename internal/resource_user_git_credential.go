package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourcePortainerUserGitCredential() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourcePortainerUserGitCredentialCreate,
		ReadContext:   resourcePortainerUserGitCredentialRead,
		UpdateContext: resourcePortainerUserGitCredentialUpdate,
		DeleteContext: resourcePortainerUserGitCredentialDelete,
		Importer: &schema.ResourceImporter{
			StateContext: func(ctx context.Context, d *schema.ResourceData, meta interface{}) ([]*schema.ResourceData, error) {
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
				if err := d.Set("user_id", userID); err != nil {
					return nil, err
				}
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

func resourcePortainerUserGitCredentialCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	userID := d.Get("user_id").(int)

	payload := map[string]interface{}{
		"name":              d.Get("name").(string),
		"username":          d.Get("username").(string),
		"password":          d.Get("password").(string),
		"authorizationType": d.Get("authorization_type").(int),
	}

	var result struct {
		GitCredential struct {
			ID int `json:"id"`
		} `json:"gitCredential"`
	}
	if err := doJSON(ctx, client, http.MethodPost, fmt.Sprintf("%s/users/%d/gitcredentials", client.Endpoint, userID), payload, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create user git credential: %w", err))
	}

	credentialID := result.GitCredential.ID
	d.SetId(fmt.Sprintf("%d:%d", userID, credentialID))
	if err := d.Set("credential_id", credentialID); err != nil {
		return diag.FromErr(err)
	}

	return resourcePortainerUserGitCredentialRead(ctx, d, meta)
}

func resourcePortainerUserGitCredentialRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	userID, credentialID, err := parseUserGitCredentialID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	var result struct {
		ID                int    `json:"id"`
		Name              string `json:"name"`
		Username          string `json:"username"`
		AuthorizationType int    `json:"authorizationType"`
		UserID            int    `json:"userId"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/users/%d/gitcredentials/%d", client.Endpoint, userID, credentialID), nil, &result); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read user git credential: %w", err))
	}

	if err := setFields(d, map[string]interface{}{
		"user_id":            userID,
		"credential_id":      credentialID,
		"name":               result.Name,
		"username":           result.Username,
		"authorization_type": result.AuthorizationType,
	}); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourcePortainerUserGitCredentialUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	userID, credentialID, err := parseUserGitCredentialID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	payload := map[string]interface{}{
		"name":              d.Get("name").(string),
		"username":          d.Get("username").(string),
		"password":          d.Get("password").(string),
		"authorizationType": d.Get("authorization_type").(int),
	}

	if err := doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/users/%d/gitcredentials/%d", client.Endpoint, userID, credentialID), payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update user git credential: %w", err))
	}

	return resourcePortainerUserGitCredentialRead(ctx, d, meta)
}

func resourcePortainerUserGitCredentialDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	userID, credentialID, err := parseUserGitCredentialID(d.Id())
	if err != nil {
		return diag.FromErr(err)
	}

	if err := doJSON(ctx, client, http.MethodDelete, fmt.Sprintf("%s/users/%d/gitcredentials/%d", client.Endpoint, userID, credentialID), nil, nil); err != nil {
		if !isAPINotFound(err) {
			return diag.FromErr(fmt.Errorf("failed to delete user git credential: %w", err))
		}
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
