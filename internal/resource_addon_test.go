package internal

import (
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// =========================================================================
// Add-on store (Business Edition). These tests pin the wire format against
// the published BE OpenAPI contract: there is no live BE instance to run
// them against, so what they verify is the shape of every request and how
// each response is folded into state.
// =========================================================================

// respondAddon is the inspect response for an installed addon, with the
// admin-only record present.
func respondAddon(lifecycle string) http.HandlerFunc {
	return RespondJSON(http.StatusOK, map[string]interface{}{
		"id":                     "portal-template",
		"displayName":            "Portal",
		"description":            "A portal",
		"shortDescription":       "Portal",
		"icon":                   "portal.svg",
		"path":                   "/portal",
		"enabled":                true,
		"healthStatus":           "healthy",
		"healthMessage":          "",
		"lifecycleStatus":        lifecycle,
		"lifecycleStatusMessage": "",
		"record": map[string]interface{}{
			"chartVersion":        "1.2.3",
			"chartPath":           "portainer/charts/portal",
			"helmChartRepository": "oci://ghcr.io/portainer/charts/portal-template",
			"registryId":          3,
			"imageRegistryId":     4,
			"availableVersions":   []string{"1.3.0", "1.2.3"},
			"upgradeAvailable":    true,
			"versionCheckError":   "",
		},
	})
}

func addonResourceData(t *testing.T) (*schema.Resource, *schema.ResourceData) {
	t.Helper()
	r := resourceAddon()
	d := r.TestResourceData()
	_ = d.Set("addon_id", "portal-template")
	return r, d
}

// TestAddonInstall_SendsCatalogPayload pins the install payload: camelCase
// keys, values decoded from JSON into a real object rather than passed as a
// string, and imageRegistryId always present.
func TestAddonInstall_SendsCatalogPayload(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/addons/portal-template", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": "portal-template", "enabled": true, "lifecycleStatus": "installed",
	}))
	mock.On("GET", "/addons/portal-template", respondAddon("installed"))

	r, d := addonResourceData(t)
	_ = d.Set("chart", "portainer/charts/portal")
	_ = d.Set("version", "1.2.3")
	_ = d.Set("registry_id", 3)
	_ = d.Set("image_registry_id", 4)
	_ = d.Set("values", `{"ingress":{"enabled":true},"replicas":2}`)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	req := mock.FindRequest("POST", "/addons/portal-template")
	if req == nil {
		t.Fatal("no install request was sent")
	}
	var payload map[string]interface{}
	if err := req.DecodeJSON(&payload); err != nil {
		t.Fatalf("install payload is not JSON: %v", err)
	}
	for key, want := range map[string]interface{}{
		"chart":           "portainer/charts/portal",
		"version":         "1.2.3",
		"registryId":      float64(3),
		"imageRegistryId": float64(4),
	} {
		if payload[key] != want {
			t.Errorf("%s: expected %v, got %v", key, want, payload[key])
		}
	}
	values, ok := payload["values"].(map[string]interface{})
	if !ok {
		t.Fatalf("values must be sent as a JSON object, got %T", payload["values"])
	}
	if values["replicas"] != float64(2) {
		t.Errorf("values.replicas: expected 2, got %v", values["replicas"])
	}

	if d.Id() != "portal-template" {
		t.Errorf("expected the addon id as the resource ID, got %q", d.Id())
	}
	if got := d.Get("chart_version"); got != "1.2.3" {
		t.Errorf("chart_version: expected 1.2.3, got %v", got)
	}
	if got := d.Get("upgrade_available"); got != true {
		t.Errorf("upgrade_available: expected true, got %v", got)
	}
}

// TestAddonInstall_AlwaysSendsImageRegistry covers the one payload field that
// has to be sent even at zero: omitting it tells Portainer to keep whatever
// registry the addon is installed with, which is not what an absent attribute
// means in a Terraform configuration.
func TestAddonInstall_AlwaysSendsImageRegistry(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/addons/portal-template", RespondJSON(http.StatusOK, map[string]interface{}{"id": "portal-template"}))
	mock.On("GET", "/addons/portal-template", respondAddon("installed"))

	r, d := addonResourceData(t)
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("POST", "/addons/portal-template").DecodeJSON(&payload); err != nil {
		t.Fatalf("install payload is not JSON: %v", err)
	}
	if _, ok := payload["imageRegistryId"]; !ok {
		t.Error("imageRegistryId must be sent even when unset, to mean 'no credentials'")
	}
	// The optional fields, in contrast, stay out of the payload entirely.
	for _, key := range []string{"chart", "version", "registryId", "values"} {
		if _, ok := payload[key]; ok {
			t.Errorf("%s must be omitted when not configured, got %v", key, payload[key])
		}
	}
}

// TestAddonRead_UninstalledLeavesState covers the drift case that a 404 does
// not cover: an addon uninstalled outside Terraform stays in the catalog, and
// only its empty lifecycle status says the release is gone.
func TestAddonRead_UninstalledLeavesState(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/addons/portal-template", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": "portal-template", "displayName": "Portal", "lifecycleStatus": "",
	}))

	r, d := addonResourceData(t)
	d.SetId("portal-template")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Error("an addon with no release must be dropped from state so Terraform reinstalls it")
	}
}

// TestAddonRead_TolerantOfMissingRecord guards the non-admin case: the record
// is admin-only and simply absent, which must not fail the read.
func TestAddonRead_TolerantOfMissingRecord(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/addons/portal-template", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": "portal-template", "lifecycleStatus": "installed", "enabled": true,
	}))

	r, d := addonResourceData(t)
	d.SetId("portal-template")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("a response without the admin-only record must still read: %v", err)
	}
	if d.Id() != "portal-template" {
		t.Error("the resource must stay in state")
	}
}

// TestAddonUninstall_PassesFlagsAsQuery pins the uninstall flags, which
// Portainer takes as query parameters rather than a body.
func TestAddonUninstall_PassesFlagsAsQuery(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/addons/portal-template", RespondJSON(http.StatusOK, map[string]interface{}{"id": "portal-template"}))

	r, d := addonResourceData(t)
	d.SetId("portal-template")
	_ = d.Set("force_uninstall", true)
	_ = d.Set("prune_config_on_uninstall", true)

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	req := mock.FindRequest("DELETE", "/addons/portal-template")
	if req == nil {
		t.Fatal("no uninstall request was sent")
	}
	if !strings.Contains(req.Query, "force=true") || !strings.Contains(req.Query, "pruneConfig=true") {
		t.Errorf("expected both uninstall flags in the query, got %q", req.Query)
	}
}

// TestAddonValues_RejectsNonObject keeps a JSON array or scalar from reaching
// the server, where it would come back as an opaque decode failure.
func TestAddonValues_RejectsNonObject(t *testing.T) {
	if _, errs := validateJSONObject(`{"a":1}`, "values"); len(errs) != 0 {
		t.Errorf("a JSON object must validate, got %v", errs)
	}
	if _, errs := validateJSONObject(`[1,2]`, "values"); len(errs) == 0 {
		t.Error("a JSON array must be rejected: Portainer expects an object")
	}
	if _, errs := validateJSONObject(`not json`, "values"); len(errs) == 0 {
		t.Error("invalid JSON must be rejected")
	}
}
