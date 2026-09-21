package internal

import (
	"context"
	"fmt"
	"net/http"
	"strconv"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func resourceUserAPIKey() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceUserAPIKeyCreate,
		ReadContext:   resourceUserAPIKeyRead,
		DeleteContext: resourceUserAPIKeyDelete,

		Schema: map[string]*schema.Schema{
			"user_id": {
				Type:        schema.TypeInt,
				Required:    true,
				ForceNew:    true,
				Description: "Identifier of the user the API key belongs to.",
			},
			"description": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Description of the key, shown in Portainer's user settings. Portainer requires it to be unique per user.",
			},
			"password": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Sensitive:   true,
				Description: "Password of the user the key is generated for. Portainer requires it to authorize the request; it is not stored on the key itself.",
			},
			// Computed attributes
			"raw_api_key": {
				Type:        schema.TypeString,
				Computed:    true,
				Sensitive:   true,
				Description: "The generated API key. Portainer returns it exactly once, at creation, so it is only ever available from the state written by that apply.",
			},
			"prefix": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Non-secret prefix of the key, which is what Portainer displays to identify it.",
			},
			"date_created": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Unix timestamp at which the key was created.",
			},
			"last_used": {
				Type:        schema.TypeInt,
				Computed:    true,
				Description: "Unix timestamp of the key's last use, zero when it has never been used.",
			},
		},
	}
}

// portainerAPIKey is the portainer.APIKey object. The digest is deliberately
// not mapped: it is the stored hash of the key and is of no use in a plan.
type portainerAPIKey struct {
	ID          int    `json:"id"`
	UserID      int    `json:"userId"`
	Description string `json:"description"`
	Prefix      string `json:"prefix"`
	DateCreated int    `json:"dateCreated"`
	LastUsed    int    `json:"lastUsed"`
}

func resourceUserAPIKeyCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	userID := d.Get("user_id").(int)

	payload := map[string]interface{}{
		"description": d.Get("description").(string),
		"password":    d.Get("password").(string),
	}

	var response struct {
		RawAPIKey string          `json:"rawAPIKey"`
		APIKey    portainerAPIKey `json:"apiKey"`
	}
	// Portainer accepts this call only from a session, never from an API key,
	// and only for the calling user's own account. Saying so here turns an
	// opaque 401 "Auth not supported" into something actionable.
	if client.APIKey != "" {
		return diag.FromErr(fmt.Errorf(
			"cannot create an API key for user %d: Portainer only accepts this call from a session, so the provider has to be configured with api_user and api_password rather than api_key - and the key can only be created for that same user", userID))
	}

	url := fmt.Sprintf("%s/users/%d/tokens", client.Endpoint, userID)
	if err := doJSON(ctx, client, http.MethodPost, url, payload, &response); err != nil {
		return diag.FromErr(fmt.Errorf("failed to create an API key for user %d: %w", userID, err))
	}
	if response.APIKey.ID == 0 {
		return diag.FromErr(fmt.Errorf("failed to create an API key for user %d: Portainer returned no key ID", userID))
	}

	// rawAPIKey is returned only here and can never be read back, so it is
	// stored now or lost.
	if err := setFields(d, map[string]interface{}{
		"raw_api_key":  response.RawAPIKey,
		"prefix":       response.APIKey.Prefix,
		"date_created": response.APIKey.DateCreated,
		"last_used":    response.APIKey.LastUsed,
	}); err != nil {
		return diag.FromErr(err)
	}

	d.SetId(strconv.Itoa(response.APIKey.ID))
	return nil
}

func resourceUserAPIKeyRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	userID := d.Get("user_id").(int)

	// Portainer has no endpoint for a single key, so the user's keys are listed
	// and matched by ID.
	var keys []portainerAPIKey
	url := fmt.Sprintf("%s/users/%d/tokens", client.Endpoint, userID)
	if err := doJSON(ctx, client, http.MethodGet, url, nil, &keys); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to list the API keys of user %d: %w", userID, err))
	}

	for _, key := range keys {
		if strconv.Itoa(key.ID) != d.Id() {
			continue
		}
		if err := setFields(d, map[string]interface{}{
			"description":  key.Description,
			"prefix":       key.Prefix,
			"date_created": key.DateCreated,
			"last_used":    key.LastUsed,
		}); err != nil {
			return diag.FromErr(err)
		}
		return nil
	}

	// Revoked outside Terraform.
	d.SetId("")
	return nil
}

func resourceUserAPIKeyDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	userID := d.Get("user_id").(int)

	url := fmt.Sprintf("%s/users/%d/tokens/%s", client.Endpoint, userID, d.Id())
	if err := doJSON(ctx, client, http.MethodDelete, url, nil, nil); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to revoke API key %s of user %d: %w", d.Id(), userID, err))
	}

	d.SetId("")
	return nil
}
