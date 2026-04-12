package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// SilenceMatcher represents a matcher within an AlertManager silence.
type SilenceMatcher struct {
	Name    string `json:"name"`
	Value   string `json:"value"`
	IsRegex bool   `json:"isRegex"`
	IsEqual bool   `json:"isEqual,omitempty"`
}

// PostableSilence represents the silence object sent to the AlertManager.
type PostableSilence struct {
	ID        string           `json:"id,omitempty"`
	Comment   string           `json:"comment"`
	CreatedBy string           `json:"createdBy"`
	StartsAt  string           `json:"startsAt"`
	EndsAt    string           `json:"endsAt"`
	Matchers  []SilenceMatcher `json:"matchers"`
}

// CreateSilencePayload is the body sent for POST /observability/alerting/silence.
type CreateSilencePayload struct {
	AlertManagerURL string          `json:"alertManagerURL"`
	Silence         PostableSilence `json:"silence"`
}

func resourceAlertingSilence() *schema.Resource {
	return &schema.Resource{
		Create: resourcePortainerAlertingSilenceCreate,
		Read:   resourcePortainerAlertingSilenceRead,
		Delete: resourcePortainerAlertingSilenceDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"alert_manager_url": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "URL of the AlertManager instance.",
			},
			"comment": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Comment explaining the reason for the silence.",
			},
			"created_by": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Name of the user creating the silence.",
			},
			"starts_at": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Start time of the silence in RFC3339 format.",
			},
			"ends_at": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "End time of the silence in RFC3339 format.",
			},
			"matchers": {
				Type:        schema.TypeList,
				Required:    true,
				ForceNew:    true,
				Description: "List of matchers to determine which alerts are silenced.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"name": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Label name to match.",
						},
						"value": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Label value to match.",
						},
						"is_regex": {
							Type:        schema.TypeBool,
							Required:    true,
							Description: "Whether the value is a regular expression.",
						},
						"is_equal": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     true,
							Description: "Whether to match for equality (true) or inequality (false).",
						},
					},
				},
			},
		},
	}
}

func resourcePortainerAlertingSilenceCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	silence := PostableSilence{
		Comment:   d.Get("comment").(string),
		CreatedBy: d.Get("created_by").(string),
		StartsAt:  d.Get("starts_at").(string),
		EndsAt:    d.Get("ends_at").(string),
	}

	if v, ok := d.GetOk("matchers"); ok {
		for _, raw := range v.([]interface{}) {
			m := raw.(map[string]interface{})
			silence.Matchers = append(silence.Matchers, SilenceMatcher{
				Name:    m["name"].(string),
				Value:   m["value"].(string),
				IsRegex: m["is_regex"].(bool),
				IsEqual: m["is_equal"].(bool),
			})
		}
	}

	payload := CreateSilencePayload{
		AlertManagerURL: d.Get("alert_manager_url").(string),
		Silence:         silence,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal silence payload: %w", err)
	}

	req, err := http.NewRequest("POST", fmt.Sprintf("%s/observability/alerting/silence", client.Endpoint), bytes.NewBuffer(jsonPayload))
	if err != nil {
		return err
	}
	setAlertingAuthHeaders(req, client)
	req.Header.Set("Content-Type", "application/json")

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("failed to create alert silence: %s", string(body))
	}

	// Parse response to get the silence ID
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err != nil {
		return fmt.Errorf("failed to decode create silence response: %w", err)
	}

	if silenceID, ok := result["silenceID"].(string); ok && silenceID != "" {
		d.SetId(silenceID)
	} else if id, ok := result["id"].(string); ok && id != "" {
		d.SetId(id)
	} else {
		return fmt.Errorf("silence created but no ID returned in response: %s", string(body))
	}

	return resourcePortainerAlertingSilenceRead(d, meta)
}

func resourcePortainerAlertingSilenceRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	// Use GET /observability/alerting/alerts?status=silenced to verify the silence exists
	req, err := http.NewRequest("GET", fmt.Sprintf("%s/observability/alerting/alerts", client.Endpoint), nil)
	if err != nil {
		return err
	}
	setAlertingAuthHeaders(req, client)

	q := req.URL.Query()
	q.Set("status", "silenced")
	req.URL.RawQuery = q.Encode()

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		// If we cannot read silenced alerts, keep the resource state as-is
		return nil
	}

	// The silence exists as long as we have an ID and can reach the server.
	// The alerts endpoint returns silenced alerts, not silences themselves.
	// We retain the stored attributes since silences are immutable (ForceNew on all fields).
	return nil
}

func resourcePortainerAlertingSilenceDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	alertManagerURL := d.Get("alert_manager_url").(string)

	deleteURL := fmt.Sprintf("%s/observability/alerting/silence/%s", client.Endpoint, d.Id())

	req, err := http.NewRequest("DELETE", deleteURL, nil)
	if err != nil {
		return err
	}
	setAlertingAuthHeaders(req, client)

	q := req.URL.Query()
	q.Set("alertManagerURL", alertManagerURL)
	req.URL.RawQuery = q.Encode()

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 && resp.StatusCode != 404 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("failed to delete alert silence %s: %s", d.Id(), string(body))
	}

	d.SetId("")
	return nil
}
