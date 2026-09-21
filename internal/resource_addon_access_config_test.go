package internal

import (
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// =========================================================================
// Add-on access and configuration (Business Edition).
// =========================================================================

// TestAddonAccess_SendsPolicyObjects pins the access payload: Portainer stores
// the grants as {"<id>": {"RoleId": n}} objects keyed by identifier, not as
// the lists a Terraform block naturally suggests.
func TestAddonAccess_SendsPolicyObjects(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/addons/portal-template/access", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": "portal-template", "enabled": true,
	}))
	mock.On("GET", "/addons/portal-template", respondAddon("installed"))

	r := resourceAddonAccess()
	d := r.TestResourceData()
	_ = d.Set("addon_id", "portal-template")
	_ = d.Set("enabled", true)
	_ = d.Set("user_access", []interface{}{map[string]interface{}{"user_id": 3, "role_id": 2}})
	_ = d.Set("team_access", []interface{}{map[string]interface{}{"team_id": 5, "role_id": 1}})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload struct {
		Enabled bool                         `json:"enabled"`
		Users   map[string]map[string]int    `json:"userAccessPolicies"`
		Teams   map[string]map[string]int    `json:"teamAccessPolicies"`
		Extra   map[string]map[string]string `json:"-"`
	}
	if err := mock.FindRequest("PUT", "/addons/portal-template/access").DecodeJSON(&payload); err != nil {
		t.Fatalf("access payload is not JSON: %v", err)
	}
	if !payload.Enabled {
		t.Error("enabled must be sent")
	}
	if got := payload.Users["3"]["RoleId"]; got != 2 {
		t.Errorf("user 3: expected RoleId 2, got %d", got)
	}
	if got := payload.Teams["5"]["RoleId"]; got != 1 {
		t.Errorf("team 5: expected RoleId 1, got %d", got)
	}
}

// TestAddonAccess_EmptySetRevokes covers what an empty block has to mean.
// Portainer leaves the stored policies alone when the field is omitted, so an
// authoritative resource must send an explicit empty object instead.
func TestAddonAccess_EmptySetRevokes(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/addons/portal-template/access", RespondJSON(http.StatusOK, map[string]interface{}{"id": "portal-template"}))
	mock.On("GET", "/addons/portal-template", respondAddon("installed"))

	r := resourceAddonAccess()
	d := r.TestResourceData()
	_ = d.Set("addon_id", "portal-template")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("PUT", "/addons/portal-template/access").DecodeJSON(&payload); err != nil {
		t.Fatalf("access payload is not JSON: %v", err)
	}
	for _, key := range []string{"userAccessPolicies", "teamAccessPolicies"} {
		policies, ok := payload[key].(map[string]interface{})
		if !ok {
			t.Fatalf("%s must be an object, got %T", key, payload[key])
		}
		if len(policies) != 0 {
			t.Errorf("%s: expected an empty object to revoke every grant, got %v", key, policies)
		}
	}
}

// TestAddonAccess_DeleteLeavesEnabledAlone pins the destroy semantics: giving
// up managing who may use an addon is not the same decision as switching the
// addon off, so `enabled` stays out of the revoke payload.
func TestAddonAccess_DeleteLeavesEnabledAlone(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/addons/portal-template/access", RespondJSON(http.StatusOK, map[string]interface{}{"id": "portal-template"}))

	r := resourceAddonAccess()
	d := r.TestResourceData()
	_ = d.Set("addon_id", "portal-template")
	d.SetId("portal-template")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("PUT", "/addons/portal-template/access").DecodeJSON(&payload); err != nil {
		t.Fatalf("revoke payload is not JSON: %v", err)
	}
	if _, ok := payload["enabled"]; ok {
		t.Error("destroying the access resource must not switch the addon off")
	}
}

// TestAddonAccess_ReadKeepsGrantsWithoutRecord guards the non-admin read: the
// policies live on the admin-only record, and a response that never carried
// them must not be taken as proof the grants are gone.
func TestAddonAccess_ReadKeepsGrantsWithoutRecord(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/addons/portal-template", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": "portal-template", "enabled": true, "lifecycleStatus": "installed",
	}))

	r := resourceAddonAccess()
	d := r.TestResourceData()
	d.SetId("portal-template")
	_ = d.Set("user_access", []interface{}{map[string]interface{}{"user_id": 3, "role_id": 2}})

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("user_access").(*schema.Set).Len(); got != 1 {
		t.Errorf("expected the configured grant to survive a read without the record, got %d", got)
	}
}

func addonConfigResource(t *testing.T, entries ...map[string]interface{}) (*schema.Resource, *schema.ResourceData) {
	t.Helper()
	r := resourceAddonConfig()
	d := r.TestResourceData()
	_ = d.Set("addon_id", "portal-template")
	list := make([]interface{}, 0, len(entries))
	for _, e := range entries {
		list = append(list, e)
	}
	_ = d.Set("entry", list)
	return r, d
}

// TestAddonConfig_CreateReplacesWholeSet pins create: taking ownership of an
// addon's configuration replaces it, which is the single PUT.
func TestAddonConfig_CreateReplacesWholeSet(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/addons/portal-template/config", RespondJSON(http.StatusOK, map[string]interface{}{"entries": []interface{}{}}))
	mock.On("GET", "/addons/portal-template/config", RespondJSON(http.StatusOK, map[string]interface{}{
		"entries": []map[string]interface{}{
			{"key": "BASE_DOMAIN", "value": "apps.example.com", "sensitive": false},
		},
	}))

	r, d := addonConfigResource(t,
		map[string]interface{}{"key": "BASE_DOMAIN", "value": "apps.example.com", "sensitive": false},
	)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload struct {
		Entries []addonConfigEntry `json:"entries"`
	}
	if err := mock.FindRequest("PUT", "/addons/portal-template/config").DecodeJSON(&payload); err != nil {
		t.Fatalf("config payload is not JSON: %v", err)
	}
	if len(payload.Entries) != 1 || payload.Entries[0].Key != "BASE_DOMAIN" {
		t.Fatalf("expected the configured entry in the payload, got %+v", payload.Entries)
	}
	if payload.Entries[0].Value != "apps.example.com" {
		t.Errorf("value: expected apps.example.com, got %q", payload.Entries[0].Value)
	}
}

// TestAddonConfig_UpdateTouchesOnlyChangedKeys is the reason update does not
// simply PUT the whole set again: a key added outside Terraform would be
// destroyed on every apply. Changed keys are PATCHed, removed keys DELETEd,
// and an untouched key is left alone.
func TestAddonConfig_UpdateTouchesOnlyChangedKeys(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PATCH", "/addons/portal-template/config/BASE_DOMAIN", RespondJSON(http.StatusOK, map[string]interface{}{"entries": []interface{}{}}))
	mock.On("DELETE", "/addons/portal-template/config/OLD_KEY", RespondJSON(http.StatusOK, map[string]interface{}{"entries": []interface{}{}}))
	mock.On("GET", "/addons/portal-template/config", RespondJSON(http.StatusOK, map[string]interface{}{"entries": []interface{}{}}))

	r := resourceAddonConfig()

	// The update path is driven by GetChange, which only reports a change when
	// the resource data carries a real diff. Building one through the SDK's own
	// Diff is the only way to exercise it: a value written with Set on
	// TestResourceData is not a change as far as GetChange is concerned.
	seed := r.TestResourceData()
	seed.SetId("portal-template")
	_ = seed.Set("addon_id", "portal-template")
	_ = seed.Set("entry", []interface{}{
		map[string]interface{}{"key": "BASE_DOMAIN", "value": "old.example.com", "sensitive": false},
		map[string]interface{}{"key": "KEEP", "value": "same", "sensitive": false},
		map[string]interface{}{"key": "OLD_KEY", "value": "gone", "sensitive": false},
	})
	state := seed.State()

	config := terraform.NewResourceConfigRaw(map[string]interface{}{
		"addon_id": "portal-template",
		"entry": []interface{}{
			map[string]interface{}{"key": "BASE_DOMAIN", "value": "new.example.com", "sensitive": false},
			map[string]interface{}{"key": "KEEP", "value": "same", "sensitive": false},
		},
	})
	diff, err := r.Diff(context.Background(), state, config, nil)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	d2, err := schema.InternalMap(r.Schema).Data(state, diff)
	if err != nil {
		t.Fatalf("building resource data from the diff failed: %v", err)
	}

	if err := rcUpdate(r, d2, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if mock.FindRequest("PATCH", "/addons/portal-template/config/BASE_DOMAIN") == nil {
		t.Error("a changed entry must be PATCHed")
	}
	if mock.FindRequest("DELETE", "/addons/portal-template/config/OLD_KEY") == nil {
		t.Error("a removed entry must be DELETEd")
	}
	if mock.FindRequest("PATCH", "/addons/portal-template/config/KEEP") != nil {
		t.Error("an unchanged entry must not be rewritten")
	}
	if mock.FindRequest("PUT", "/addons/portal-template/config") != nil {
		t.Error("update must not replace the whole configuration")
	}
}

// TestAddonConfig_ReadKeepsWithheldSecret covers the sensitive entry:
// Portainer does not return its value, and blanking it in state would make
// every subsequent plan show a spurious change.
func TestAddonConfig_ReadKeepsWithheldSecret(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/addons/portal-template/config", RespondJSON(http.StatusOK, map[string]interface{}{
		"entries": []map[string]interface{}{
			{"key": "API_TOKEN", "value": "", "sensitive": true},
		},
	}))

	r, d := addonConfigResource(t,
		map[string]interface{}{"key": "API_TOKEN", "value": "s3cret", "sensitive": true},
	)
	d.SetId("portal-template")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	entries := d.Get("entry").(*schema.Set).List()
	if len(entries) != 1 {
		t.Fatalf("expected one entry, got %d", len(entries))
	}
	entry := entries[0].(map[string]interface{})
	if entry["value"] != "s3cret" {
		t.Errorf("a withheld sensitive value must keep what was configured, got %q", entry["value"])
	}
}

// TestAddonConfig_DeleteClearsEverything pins destroy to the bulk endpoint
// rather than a per-key sweep.
func TestAddonConfig_DeleteClearsEverything(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/addons/portal-template/config", RespondJSON(http.StatusOK, map[string]interface{}{"entries": []interface{}{}}))

	r, d := addonConfigResource(t)
	d.SetId("portal-template")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/addons/portal-template/config") == nil {
		t.Error("destroy must clear the addon's configuration")
	}
	if d.Id() != "" {
		t.Error("the resource must be removed from state")
	}
}
