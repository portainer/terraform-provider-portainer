package internal

import (
	"net/http"
	"strings"
	"testing"
)

func TestDataSourceAddonCatalog(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/addons/catalog", RespondJSON(http.StatusOK, map[string]interface{}{
		"addons": []map[string]interface{}{{
			"id": "portal-template", "displayName": "Portal", "serviceUrl": "http://portal:8080",
			"healthCheckPath": "/readyz", "helmChartRepository": "oci://ghcr.io/portainer/charts/portal-template",
			"tlsVerify": true, "machineAccess": []string{"deploy"},
		}},
	}))

	ds := dataSourceAddonCatalog()
	d := ds.TestResourceData()
	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	for field, want := range map[string]interface{}{
		"addons.0.id": "portal-template", "addons.0.display_name": "Portal",
		"addons.0.service_url": "http://portal:8080", "addons.0.health_check_path": "/readyz",
		"addons.0.tls_verify": true, "addons.0.machine_access.0": "deploy",
	} {
		if got := d.Get(field); got != want {
			t.Errorf("%s: got %v, want %v", field, got, want)
		}
	}
	if d.Id() != "portainer-addon-catalog" {
		t.Errorf("id: got %q", d.Id())
	}
}

// TestDataSourceAddonCatalog_NoCopy verifies the 503 Portainer answers when it
// has no catalog at all fails the read instead of reporting an empty catalog.
func TestDataSourceAddonCatalog_NoCopy(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/addons/catalog", RespondString(http.StatusServiceUnavailable, "", "catalog unavailable"))

	ds := dataSourceAddonCatalog()
	d := ds.TestResourceData()
	err := rcRead(ds, d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "503") {
		t.Fatalf("expected the 503 to surface, got %v", err)
	}
}

func TestAddonCatalogRefresh(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/addons/catalog/refresh", RespondString(http.StatusNoContent, "", ""))

	r := resourceAddonCatalogRefresh()
	d := r.TestResourceData()
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if !strings.HasPrefix(d.Id(), "addon-catalog-refresh-") {
		t.Errorf("id: got %q", d.Id())
	}
	if mock.FindRequest("POST", "/addons/catalog/refresh") == nil {
		t.Error("expected POST /addons/catalog/refresh")
	}

	forbidden := NewMockServer(t)
	forbidden.On("POST", "/addons/catalog/refresh", RespondString(http.StatusForbidden, "", "admins only"))
	d2 := r.TestResourceData()
	if err := rcCreate(r, d2, forbidden.Client()); err == nil {
		t.Error("expected an error on 403")
	}
}
