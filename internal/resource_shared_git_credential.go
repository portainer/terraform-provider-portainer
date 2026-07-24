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

	var result struct {
		GitCredential struct {
			ID int `json:"id"`
		} `json:"gitCredential"`
	}
	if err := doJSON(ctx, client, http.MethodPost, fmt.Sprintf("%s/cloud/gitcredentials", client.Endpoint), payload, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create shared git credential: %w", err))
	}

	d.SetId(strconv.Itoa(result.GitCredential.ID))
	return resourcePortainerSharedGitCredentialRead(ctx, d, meta)
}

func resourcePortainerSharedGitCredentialRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Id()

	var result struct {
		ID                int    `json:"id"`
		Name              string `json:"name"`
		Username          string `json:"username"`
		AuthorizationType int    `json:"authorizationType"`
		UserID            int    `json:"userId"`
	}
	if err := doJSON(ctx, client, http.MethodGet, fmt.Sprintf("%s/cloud/gitcredentials/%s", client.Endpoint, id), nil, &result); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read shared git credential: %w", err))
	}

	if err := setFields(d, map[string]interface{}{
		"name":               result.Name,
		"username":           result.Username,
		"authorization_type": result.AuthorizationType,
		"user_id":            result.UserID,
	}); err != nil {
		return diag.FromErr(err)
	}

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

	if err := doJSON(ctx, client, http.MethodPut, fmt.Sprintf("%s/cloud/gitcredentials/%s", client.Endpoint, id), payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to update shared git credential: %w", err))
	}

	return resourcePortainerSharedGitCredentialRead(ctx, d, meta)
}

func resourcePortainerSharedGitCredentialDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	id := d.Id()

	if err := doJSON(ctx, client, http.MethodDelete, fmt.Sprintf("%s/cloud/gitcredentials/%s", client.Endpoint, id), nil, nil); err != nil {
		if !isAPINotFound(err) {
			return diag.FromErr(fmt.Errorf("failed to delete shared git credential: %w", err))
		}
	}

	d.SetId("")
	return nil
}
