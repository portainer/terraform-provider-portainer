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

func resourceHelmUserRepository() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceHelmUserRepositoryCreate,
		ReadContext:   resourceHelmUserRepositoryRead,
		DeleteContext: resourceHelmUserRepositoryDelete,
		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},
		Schema: map[string]*schema.Schema{
			"user_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "User identifier.",
			},
			"url": {
				Type:         schema.TypeString,
				Required:     true,
				ForceNew:     true,
				Description:  "Helm repository URL (e.g. https://charts.bitnami.com/bitnami).",
				ValidateFunc: validation.IsURLWithHTTPorHTTPS,
			},
		},
	}
}

func resourceHelmUserRepositoryCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	userID := d.Get("user_id").(int)
	repoURL := d.Get("url").(string)

	payload := map[string]string{
		"url": repoURL,
	}

	path := fmt.Sprintf("/users/%d/helm/repositories", userID)
	var result struct {
		ID     int    `json:"Id"`
		URL    string `json:"URL"`
		UserID int    `json:"UserId"`
	}
	if err := doJSON(ctx, client, http.MethodPost, client.Endpoint+path, payload, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create helm user repository: %w", err))
	}

	d.SetId(strconv.Itoa(result.ID))
	if err := d.Set("url", result.URL); err != nil {
		return diag.FromErr(err)
	}

	return nil
}

func resourceHelmUserRepositoryRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	userID := d.Get("user_id").(int)
	repoID := d.Id()

	path := fmt.Sprintf("/users/%d/helm/repositories", userID)
	var result struct {
		UserRepositories []struct {
			ID     int    `json:"Id"`
			URL    string `json:"URL"`
			UserID int    `json:"UserId"`
		} `json:"UserRepositories"`
	}
	if err := doJSON(ctx, client, http.MethodGet, client.Endpoint+path, nil, &result); err != nil {
		return diag.FromErr(fmt.Errorf("failed to read helm user repositories: %w", err))
	}

	for _, repo := range result.UserRepositories {
		if strconv.Itoa(repo.ID) == repoID {
			if err := setFields(d, map[string]interface{}{
				"url":     repo.URL,
				"user_id": repo.UserID,
			}); err != nil {
				return diag.FromErr(err)
			}
			return nil
		}
	}

	// Repository not found - remove from state
	d.SetId("")
	return nil
}

func resourceHelmUserRepositoryDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	userID := d.Get("user_id").(int)
	repoID := d.Id()

	path := fmt.Sprintf("/users/%d/helm/repositories/%s", userID, repoID)
	if err := doJSON(ctx, client, http.MethodDelete, client.Endpoint+path, nil, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to delete helm user repository: %w", err))
	}

	d.SetId("")
	return nil
}
