package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/validation"
)

// AlertingNotificationChannel represents a notification channel within alerting settings.
type AlertingNotificationChannel struct {
	ID      int                    `json:"id,omitempty"`
	Name    string                 `json:"name,omitempty"`
	Type    string                 `json:"type,omitempty"`
	Enabled bool                   `json:"enabled,omitempty"`
	Config  map[string]interface{} `json:"config,omitempty"`
}

// AlertingSettings represents the Portainer alerting settings object.
type AlertingSettings struct {
	ID                   int                           `json:"id,omitempty"`
	Name                 string                        `json:"name,omitempty"`
	Enabled              bool                          `json:"enabled"`
	IsInternal           bool                          `json:"isInternal,omitempty"`
	URL                  string                        `json:"url,omitempty"`
	PortainerURL         string                        `json:"portainerURL,omitempty"`
	Status               string                        `json:"status,omitempty"`
	Uptime               string                        `json:"uptime,omitempty"`
	CreatedAt            string                        `json:"createdAt,omitempty"`
	CreatedBy            string                        `json:"createdBy,omitempty"`
	NotificationChannels []AlertingNotificationChannel `json:"notificationChannels,omitempty"`
}

// AlertingUpdatePayload is the body sent for PUT /observability/alerting/settings.
type AlertingUpdatePayload struct {
	AlertingSettings AlertingSettings `json:"AlertingSettings"`
}

func resourceAlertingSettings() *schema.Resource {
	return &schema.Resource{
		Create: resourcePortainerAlertingSettingsCreate,
		Read:   resourcePortainerAlertingSettingsRead,
		Update: resourcePortainerAlertingSettingsUpdate,
		Delete: resourcePortainerAlertingSettingsDelete,
		Importer: &schema.ResourceImporter{
			State: schema.ImportStatePassthrough,
		},
		Schema: map[string]*schema.Schema{
			"enabled": {
				Type:        schema.TypeBool,
				Required:    true,
				Description: "Whether alerting is enabled.",
			},
			"name": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Name of the alerting settings entry.",
			},
			"url": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "URL of the external AlertManager instance. Leave empty for internal.",
			},
			"portainer_url": {
				Type:        schema.TypeString,
				Optional:    true,
				Computed:    true,
				Description: "Portainer URL used by AlertManager for callbacks.",
			},
			"is_internal": {
				Type:        schema.TypeBool,
				Computed:    true,
				Description: "Whether this uses the internal AlertManager.",
			},
			"status": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Connection status (disabled, connected, disconnected, error).",
			},
			"uptime": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Uptime of the AlertManager.",
			},
			"created_at": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "Timestamp when the settings were created.",
			},
			"created_by": {
				Type:        schema.TypeString,
				Computed:    true,
				Description: "User who created the settings.",
			},
			"notification_channels": {
				Type:        schema.TypeList,
				Optional:    true,
				Computed:    true,
				Description: "List of notification channels.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"channel_id": {
							Type:        schema.TypeInt,
							Optional:    true,
							Computed:    true,
							Description: "Notification channel identifier.",
						},
						"name": {
							Type:        schema.TypeString,
							Optional:    true,
							Computed:    true,
							Description: "Name of the notification channel.",
						},
						"type": {
							Type:         schema.TypeString,
							Optional:     true,
							Computed:     true,
							Description:  "Type of the notification channel.",
							ValidateFunc: validation.StringInSlice([]string{"slack", "webhook", "teams", "discord", "email", "pagerduty", "opsgenie"}, false),
						},
						"enabled": {
							Type:        schema.TypeBool,
							Optional:    true,
							Computed:    true,
							Description: "Whether the notification channel is enabled.",
						},
						"config": {
							Type:        schema.TypeMap,
							Optional:    true,
							Computed:    true,
							Sensitive:   true,
							Description: "Configuration key-value pairs for the notification channel.",
							Elem:        &schema.Schema{Type: schema.TypeString},
						},
					},
				},
			},
		},
	}
}

func resourcePortainerAlertingSettingsCreate(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	payload := buildAlertingSettingsPayload(d)

	jsonPayload, err := json.Marshal(AlertingUpdatePayload{AlertingSettings: payload})
	if err != nil {
		return fmt.Errorf("failed to marshal alerting settings payload: %w", err)
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/observability/alerting/settings", client.Endpoint), bytes.NewBuffer(jsonPayload))
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
		return fmt.Errorf("failed to create/update alerting settings: %s", string(body))
	}

	// Parse the response to get the ID
	var result map[string]interface{}
	if err := json.Unmarshal(body, &result); err == nil {
		if id, ok := result["id"].(float64); ok {
			d.SetId(fmt.Sprintf("%d", int(id)))
			return resourcePortainerAlertingSettingsRead(d, meta)
		}
	}

	d.SetId("portainer-alerting-settings")
	return resourcePortainerAlertingSettingsRead(d, meta)
}

func resourcePortainerAlertingSettingsRead(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	// If we have a numeric ID, read the specific settings entry
	settingsID := d.Id()
	url := fmt.Sprintf("%s/observability/alerting/settings", client.Endpoint)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	setAlertingAuthHeaders(req, client)

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		d.SetId("")
		return nil
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("failed to read alerting settings, status: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// The GET /observability/alerting/settings returns an array.
	// Try to find our settings entry by ID.
	var settingsList []AlertingSettings
	if err := json.Unmarshal(body, &settingsList); err != nil {
		// Try single object in case the response format differs
		var single AlertingSettings
		if err2 := json.Unmarshal(body, &single); err2 != nil {
			return fmt.Errorf("failed to decode alerting settings response: %w", err)
		}
		settingsList = []AlertingSettings{single}
	}

	// Find matching settings entry
	var settings *AlertingSettings
	for i := range settingsList {
		if fmt.Sprintf("%d", settingsList[i].ID) == settingsID {
			settings = &settingsList[i]
			break
		}
	}
	if settings == nil && len(settingsList) > 0 {
		// If ID is the generic one or not found, use the first entry
		settings = &settingsList[0]
		d.SetId(fmt.Sprintf("%d", settings.ID))
	}
	if settings == nil {
		d.SetId("")
		return nil
	}

	_ = d.Set("enabled", settings.Enabled)
	_ = d.Set("name", settings.Name)
	_ = d.Set("url", settings.URL)
	_ = d.Set("portainer_url", settings.PortainerURL)
	_ = d.Set("is_internal", settings.IsInternal)
	_ = d.Set("status", settings.Status)
	_ = d.Set("uptime", settings.Uptime)
	_ = d.Set("created_at", settings.CreatedAt)
	_ = d.Set("created_by", settings.CreatedBy)

	channels := make([]map[string]interface{}, 0, len(settings.NotificationChannels))
	for _, ch := range settings.NotificationChannels {
		cfg := make(map[string]interface{})
		for k, v := range ch.Config {
			cfg[k] = fmt.Sprintf("%v", v)
		}
		channels = append(channels, map[string]interface{}{
			"channel_id": ch.ID,
			"name":       ch.Name,
			"type":       ch.Type,
			"enabled":    ch.Enabled,
			"config":     cfg,
		})
	}
	_ = d.Set("notification_channels", channels)

	return nil
}

func resourcePortainerAlertingSettingsUpdate(d *schema.ResourceData, meta interface{}) error {
	return resourcePortainerAlertingSettingsCreate(d, meta)
}

func resourcePortainerAlertingSettingsDelete(d *schema.ResourceData, meta interface{}) error {
	client := meta.(*APIClient)

	// On delete, disable alerting by sending enabled=false
	disabledSettings := AlertingSettings{
		Enabled: false,
	}

	// Preserve the ID if it's numeric
	settingsID := d.Id()
	if settingsID != "" && settingsID != "portainer-alerting-settings" {
		var id int
		if _, err := fmt.Sscanf(settingsID, "%d", &id); err == nil {
			disabledSettings.ID = id
		}
	}

	jsonPayload, err := json.Marshal(AlertingUpdatePayload{AlertingSettings: disabledSettings})
	if err != nil {
		return err
	}

	req, err := http.NewRequest("PUT", fmt.Sprintf("%s/observability/alerting/settings", client.Endpoint), bytes.NewBuffer(jsonPayload))
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
		return fmt.Errorf("failed to disable alerting settings: %s", string(body))
	}

	d.SetId("")
	return nil
}

func buildAlertingSettingsPayload(d *schema.ResourceData) AlertingSettings {
	settings := AlertingSettings{
		Enabled:      d.Get("enabled").(bool),
		Name:         d.Get("name").(string),
		URL:          d.Get("url").(string),
		PortainerURL: d.Get("portainer_url").(string),
	}

	// Preserve ID for updates
	settingsID := d.Id()
	if settingsID != "" && settingsID != "portainer-alerting-settings" {
		var id int
		if _, err := fmt.Sscanf(settingsID, "%d", &id); err == nil {
			settings.ID = id
		}
	}

	if v, ok := d.GetOk("notification_channels"); ok {
		for _, raw := range v.([]interface{}) {
			ch := raw.(map[string]interface{})
			channel := AlertingNotificationChannel{
				ID:      ch["channel_id"].(int),
				Name:    ch["name"].(string),
				Type:    ch["type"].(string),
				Enabled: ch["enabled"].(bool),
			}
			if cfg, ok := ch["config"]; ok {
				channel.Config = make(map[string]interface{})
				for k, v := range cfg.(map[string]interface{}) {
					channel.Config[k] = v
				}
			}
			settings.NotificationChannels = append(settings.NotificationChannels, channel)
		}
	}

	return settings
}

func setAlertingAuthHeaders(req *http.Request, client *APIClient) {
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	}
}
