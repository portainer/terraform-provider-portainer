package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceHelmReleaseHistoryRead_HappyPath verifies the GET to
// /endpoints/{id}/kubernetes/helm/{name}/history decodes the (slightly nested)
// Helm history response into the flat `revisions` list shape, including the
// chart name+version concatenation.
func TestDataSourceHelmReleaseHistoryRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/4/kubernetes/helm/myapp/history", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"name":       "myapp",
			"namespace":  "default",
			"version":    1,
			"appVersion": "1.0.0",
			"info": map[string]interface{}{
				"status":        "superseded",
				"description":   "Install complete",
				"last_deployed": "2024-01-01T00:00:00Z",
			},
			"chart": map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":    "myapp",
					"version": "0.1.0",
				},
			},
		},
		{
			"name":       "myapp",
			"namespace":  "default",
			"version":    2,
			"appVersion": "1.1.0",
			"info": map[string]interface{}{
				"status":        "deployed",
				"description":   "Upgrade complete",
				"last_deployed": "2024-02-01T00:00:00Z",
			},
			"chart": map[string]interface{}{
				"metadata": map[string]interface{}{
					"name":    "myapp",
					"version": "0.2.0",
				},
			},
		},
	}))

	ds := dataSourceHelmReleaseHistory()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 4)
	_ = d.Set("release_name", "myapp")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Id(); got != "helm-history-4-myapp" {
		t.Errorf("unexpected ID: %q", got)
	}

	revisions := d.Get("revisions").([]interface{})
	if len(revisions) != 2 {
		t.Fatalf("expected 2 revisions, got %d", len(revisions))
	}

	first := revisions[0].(map[string]interface{})
	if first["revision"] != 1 {
		t.Errorf("revisions[0].revision: got %v", first["revision"])
	}
	if first["status"] != "superseded" {
		t.Errorf("revisions[0].status: got %v", first["status"])
	}
	if first["chart"] != "myapp-0.1.0" {
		t.Errorf("revisions[0].chart: got %v", first["chart"])
	}
	if first["app_version"] != "1.0.0" {
		t.Errorf("revisions[0].app_version: got %v", first["app_version"])
	}
	if first["updated"] != "2024-01-01T00:00:00Z" {
		t.Errorf("revisions[0].updated: got %v", first["updated"])
	}

	second := revisions[1].(map[string]interface{})
	if second["revision"] != 2 {
		t.Errorf("revisions[1].revision: got %v", second["revision"])
	}
	if second["status"] != "deployed" {
		t.Errorf("revisions[1].status: got %v", second["status"])
	}
	if second["chart"] != "myapp-0.2.0" {
		t.Errorf("revisions[1].chart: got %v", second["chart"])
	}
}

// TestDataSourceHelmReleaseHistoryRead_NamespaceQueryParam verifies that when
// namespace is set, it is appended as a query parameter (the code constructs
// the URL by concatenation, not via url.Values).
func TestDataSourceHelmReleaseHistoryRead_NamespaceQueryParam(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/2/kubernetes/helm/app/history", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	ds := dataSourceHelmReleaseHistory()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("release_name", "app")
	_ = d.Set("namespace", "prod")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	req := mock.FindRequest("GET", "/endpoints/2/kubernetes/helm/app/history")
	if req == nil {
		t.Fatal("expected GET to history endpoint")
	}
	if req.Query != "namespace=prod" {
		t.Errorf("expected query 'namespace=prod', got %q", req.Query)
	}
}

// TestDataSourceHelmReleaseHistoryRead_HTTPError verifies that a non-2xx
// response is surfaced as an error.
func TestDataSourceHelmReleaseHistoryRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/kubernetes/helm/missing/history", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"release not found"}`,
	))

	ds := dataSourceHelmReleaseHistory()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("release_name", "missing")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 404, got nil")
	}
}
