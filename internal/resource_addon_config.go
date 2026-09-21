package internal

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"sort"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// addonConfigEntry is one stored configuration entry of an addon, in the
// shape Portainer both accepts and returns.
type addonConfigEntry struct {
	Key       string `json:"key"`
	Value     string `json:"value"`
	Sensitive bool   `json:"sensitive"`
}

func resourceAddonConfig() *schema.Resource {
	return &schema.Resource{
		CreateContext: resourceAddonConfigCreate,
		ReadContext:   resourceAddonConfigRead,
		UpdateContext: resourceAddonConfigUpdate,
		DeleteContext: resourceAddonConfigDelete,

		Importer: &schema.ResourceImporter{
			StateContext: schema.ImportStatePassthroughContext,
		},

		Schema: map[string]*schema.Schema{
			"addon_id": {
				Type:        schema.TypeString,
				Required:    true,
				ForceNew:    true,
				Description: "Catalog identifier of the addon whose stored configuration this resource owns, for example `portal-template`.",
			},
			"entry": {
				Type:        schema.TypeSet,
				Required:    true,
				Description: "Configuration entries of the addon. The set is authoritative: an entry removed from the configuration is deleted from the addon.",
				Elem: &schema.Resource{
					Schema: map[string]*schema.Schema{
						"key": {
							Type:        schema.TypeString,
							Required:    true,
							Description: "Name of the configuration entry, for example `BASE_DOMAIN`.",
						},
						"value": {
							Type:     schema.TypeString,
							Required: true,
							// Entries marked sensitive are not returned by
							// Portainer, and any entry may legitimately hold a
							// credential, so the value never lands in a plan.
							Sensitive:   true,
							Description: "Value of the configuration entry.",
						},
						"sensitive": {
							Type:        schema.TypeBool,
							Optional:    true,
							Default:     false,
							Description: "Whether Portainer should treat the value as a secret, which keeps it out of API responses and makes clients display it carefully.",
						},
					},
				},
			},
		},
	}
}

// addonConfigEntries reads an entry set out of the resource data, keyed for
// comparison between old and new.
func addonConfigEntries(raw interface{}) map[string]addonConfigEntry {
	entries := map[string]addonConfigEntry{}
	set, ok := raw.(*schema.Set)
	if !ok {
		return entries
	}
	for _, item := range set.List() {
		m, ok := item.(map[string]interface{})
		if !ok {
			continue
		}
		key, _ := m["key"].(string)
		if key == "" {
			continue
		}
		value, _ := m["value"].(string)
		sensitive, _ := m["sensitive"].(bool)
		entries[key] = addonConfigEntry{Key: key, Value: value, Sensitive: sensitive}
	}
	return entries
}

func addonConfigURL(client *APIClient, addonID string) string {
	return addonURL(client, addonID) + "/config"
}

// sortedKeys keeps the order of the entries sent to Portainer stable, so a
// re-apply that changes nothing produces an identical request.
func sortedKeys(entries map[string]addonConfigEntry) []string {
	keys := make([]string, 0, len(entries))
	for key := range entries {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func resourceAddonConfigCreate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	addonID := d.Get("addon_id").(string)

	// Create replaces the addon's configuration outright, which is what taking
	// ownership of it means. Updates go entry by entry instead, so a key added
	// outside Terraform is not silently destroyed on every apply.
	entries := addonConfigEntries(d.Get("entry"))
	list := make([]addonConfigEntry, 0, len(entries))
	for _, key := range sortedKeys(entries) {
		list = append(list, entries[key])
	}
	payload := map[string]interface{}{"entries": list}

	if err := doJSON(ctx, client, http.MethodPut, addonConfigURL(client, addonID), payload, nil); err != nil {
		return diag.FromErr(fmt.Errorf("failed to write the configuration of addon %s: %w", addonID, err))
	}

	d.SetId(addonID)
	return resourceAddonConfigRead(ctx, d, meta)
}

func resourceAddonConfigUpdate(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)
	addonID := d.Id()

	oldRaw, newRaw := d.GetChange("entry")
	before := addonConfigEntries(oldRaw)
	after := addonConfigEntries(newRaw)

	for _, key := range sortedKeys(after) {
		entry := after[key]
		if existing, ok := before[key]; ok && existing == entry {
			continue
		}
		endpoint := fmt.Sprintf("%s/%s", addonConfigURL(client, addonID), url.PathEscape(key))
		payload := map[string]interface{}{"value": entry.Value, "sensitive": entry.Sensitive}
		if err := doJSON(ctx, client, http.MethodPatch, endpoint, payload, nil); err != nil {
			return diag.FromErr(fmt.Errorf("failed to set configuration entry %q of addon %s: %w", key, addonID, err))
		}
	}

	for _, key := range sortedKeys(before) {
		if _, ok := after[key]; ok {
			continue
		}
		endpoint := fmt.Sprintf("%s/%s", addonConfigURL(client, addonID), url.PathEscape(key))
		if err := doJSON(ctx, client, http.MethodDelete, endpoint, nil, nil); err != nil && !isAPINotFound(err) {
			return diag.FromErr(fmt.Errorf("failed to remove configuration entry %q of addon %s: %w", key, addonID, err))
		}
	}

	return resourceAddonConfigRead(ctx, d, meta)
}

func resourceAddonConfigRead(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	var response struct {
		Entries []addonConfigEntry `json:"entries"`
	}
	if err := doJSON(ctx, client, http.MethodGet, addonConfigURL(client, d.Id()), nil, &response); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to read the configuration of addon %s: %w", d.Id(), err))
	}

	// Portainer withholds the value of a sensitive entry. Taking the response
	// at face value would blank it in state and make the next plan look like a
	// change, so the configured value is kept whenever none came back.
	known := addonConfigEntries(d.Get("entry"))
	entries := make([]interface{}, 0, len(response.Entries))
	for _, entry := range response.Entries {
		if entry.Value == "" {
			if previous, ok := known[entry.Key]; ok {
				entry.Value = previous.Value
			}
		}
		entries = append(entries, map[string]interface{}{
			"key":       entry.Key,
			"value":     entry.Value,
			"sensitive": entry.Sensitive,
		})
	}

	if err := setFields(d, map[string]interface{}{
		"addon_id": d.Id(),
		"entry":    entries,
	}); err != nil {
		return diag.FromErr(err)
	}
	return nil
}

func resourceAddonConfigDelete(ctx context.Context, d *schema.ResourceData, meta interface{}) diag.Diagnostics {
	client := meta.(*APIClient)

	if err := doJSON(ctx, client, http.MethodDelete, addonConfigURL(client, d.Id()), nil, nil); err != nil {
		if isAPINotFound(err) {
			d.SetId("")
			return nil
		}
		return diag.FromErr(fmt.Errorf("failed to clear the configuration of addon %s: %w", d.Id(), err))
	}

	d.SetId("")
	return nil
}
