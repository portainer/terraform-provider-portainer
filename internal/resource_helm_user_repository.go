package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

func resourceHelmUserRepository() *schema.Resource {
	return &schema.Resource{
		Create: resourceHelmUserRepositoryCreate,
		Read:   resourceHelmUserRepositoryRead,
		Delete: resourceHelmUserRepositoryDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
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

func resourceHelmUserRepositoryCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	userID := d.Get("user_id").(int)
	repoURL := d.Get("url").(string)

	payload := map[string]string{
		"url": repoURL,
	}

	path := fmt.Sprintf("/users/%d/helm/repositories", userID)
	resp, err := client.DoRequest(http.MethodPost, path, nil, payload)
	if err != nil {
		return fmt.Errorf("failed to create helm user repository: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to create helm user repository: HTTP %d", resp.StatusCode)
	}

	var result struct {
		ID     int    `json:"Id"`
		URL    string `json:"URL"`
		UserID int    `json:"UserId"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	d.SetId(strconv.Itoa(result.ID))
	_ = d.Set("url", result.URL)

	return nil
}

func resourceHelmUserRepositoryRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	userID := d.Get("user_id").(int)
	repoID := d.Id()

	path := fmt.Sprintf("/users/%d/helm/repositories", userID)
	resp, err := client.DoRequest(http.MethodGet, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to read helm user repositories: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to read helm user repositories: HTTP %d", resp.StatusCode)
	}

	var result struct {
		UserRepositories []struct {
			ID     int    `json:"Id"`
			URL    string `json:"URL"`
			UserID int    `json:"UserId"`
		} `json:"UserRepositories"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("failed to decode response: %w", err)
	}

	for _, repo := range result.UserRepositories {
		if strconv.Itoa(repo.ID) == repoID {
			_ = d.Set("url", repo.URL)
			_ = d.Set("user_id", repo.UserID)
			return nil
		}
	}

	// Repository not found - remove from state
	d.SetId("")
	return nil
}

func resourceHelmUserRepositoryDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	userID := d.Get("user_id").(int)
	repoID := d.Id()

	path := fmt.Sprintf("/users/%d/helm/repositories/%s", userID, repoID)
	resp, err := client.DoRequest(http.MethodDelete, path, nil, nil)
	if err != nil {
		return fmt.Errorf("failed to delete helm user repository: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to delete helm user repository: HTTP %d", resp.StatusCode)
	}

	d.SetId("")
	return nil
}
