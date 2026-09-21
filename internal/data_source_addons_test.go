package internal

import (
	"net/http"
	"strings"
	"testing"
)

// TestDataSourceAddons_ListsCatalogAndEnvironment covers the listing plus the
// environment lookup it folds in, and the catalog error that tells an admin
// the listing is a cached copy.
func TestDataSourceAddons_ListsCatalogAndEnvironment(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/addons", RespondJSON(http.StatusOK, map[string]interface{}{
		"catalogError": "registry unreachable",
		"addons": []map[string]interface{}{
			{
				"id": "portal-template", "displayName": "Portal", "enabled": true,
				"lifecycleStatus": "installed", "healthStatus": "healthy",
				"record": map[string]interface{}{
					"chartVersion": "1.2.3", "upgradeAvailable": true,
					"availableVersions": []string{"1.3.0", "1.2.3"},
				},
			},
			// No record: this is what a non-admin sees, and it must not fail.
			{"id": "other", "displayName": "Other", "lifecycleStatus": ""},
		},
	}))
	mock.On("GET", "/addons/environment", RespondJSON(http.StatusOK, map[string]interface{}{
		"environmentId": 7,
	}))

	ds := dataSourceAddons()
	d := ds.TestResourceData()

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("environment_id"); got != 7 {
		t.Errorf("environment_id: expected 7, got %v", got)
	}
	if got := d.Get("catalog_error"); got != "registry unreachable" {
		t.Errorf("catalog_error: expected the server's explanation, got %v", got)
	}

	addons := d.Get("addons").([]interface{})
	if len(addons) != 2 {
		t.Fatalf("expected two addons, got %d", len(addons))
	}
	first := addons[0].(map[string]interface{})
	if first["chart_version"] != "1.2.3" || first["upgrade_available"] != true {
		t.Errorf("record fields were not read: %+v", first)
	}
	second := addons[1].(map[string]interface{})
	if second["id"] != "other" || second["chart_version"] != "" {
		t.Errorf("an addon without the admin-only record must still be listed: %+v", second)
	}
}

// TestDataSourceAddons_SwitcherView pins the one query parameter the endpoint
// takes, which is also what makes the listing readable by a non-admin.
func TestDataSourceAddons_SwitcherView(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/addons", RespondJSON(http.StatusOK, map[string]interface{}{"addons": []interface{}{}}))
	mock.On("GET", "/addons/environment", RespondJSON(http.StatusOK, map[string]interface{}{"environmentId": 1}))

	ds := dataSourceAddons()
	d := ds.TestResourceData()
	_ = d.Set("view", "switcher")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	req := mock.FindRequest("GET", "/addons")
	if req == nil || !strings.Contains(req.Query, "view=switcher") {
		t.Errorf("expected the switcher view in the query, got %q", req.Query)
	}
}

// TestDataSourceAddonChartSource_QueriesRegistryAndChart pins the pre-install
// reachability check, whose whole point is the query it sends.
func TestDataSourceAddonChartSource_QueriesRegistryAndChart(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/addons/portal-template/chart-source", RespondJSON(http.StatusOK, map[string]interface{}{
		"reachable": true, "reason": "", "tlsVerify": true,
		"versions": []string{"1.3.0", "1.2.3"},
	}))

	ds := dataSourceAddonChartSource()
	d := ds.TestResourceData()
	_ = d.Set("addon_id", "portal-template")
	_ = d.Set("registry_id", 3)
	_ = d.Set("chart", "portainer/charts/portal")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	req := mock.FindRequest("GET", "/addons/portal-template/chart-source")
	if req == nil {
		t.Fatal("no chart-source request was sent")
	}
	if !strings.Contains(req.Query, "registryId=3") {
		t.Errorf("expected registryId in the query, got %q", req.Query)
	}
	if !strings.Contains(req.Query, "chart=portainer%2Fcharts%2Fportal") {
		t.Errorf("expected the chart path escaped in the query, got %q", req.Query)
	}

	if got := d.Get("reachable"); got != true {
		t.Errorf("reachable: expected true, got %v", got)
	}
	if got := d.Get("versions").([]interface{}); len(got) != 2 || got[0] != "1.3.0" {
		t.Errorf("versions: expected the published list newest-first, got %v", got)
	}
}

// TestAddonRepair_PostsAndRecordsOutcome covers the repair action and the
// lifecycle fields it reports back.
func TestAddonRepair_PostsAndRecordsOutcome(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/addons/portal-template/repair", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": "portal-template", "enabled": true, "chartVersion": "1.2.3",
		"lifecycleStatus": "installed", "lifecycleStatusMessage": "",
	}))

	r := resourceAddonRepair()
	d := r.TestResourceData()
	_ = d.Set("addon_id", "portal-template")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if mock.FindRequest("POST", "/addons/portal-template/repair") == nil {
		t.Fatal("no repair request was sent")
	}
	if got := d.Get("lifecycle_status"); got != "installed" {
		t.Errorf("lifecycle_status: expected installed, got %v", got)
	}
	if got := d.Get("chart_version"); got != "1.2.3" {
		t.Errorf("chart_version: expected 1.2.3, got %v", got)
	}
	if !strings.HasPrefix(d.Id(), "portal-template-repair-") {
		t.Errorf("the resource ID should name the addon and the run, got %q", d.Id())
	}
}
