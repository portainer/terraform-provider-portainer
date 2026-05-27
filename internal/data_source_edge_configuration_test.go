package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceEdgeConfigurationRead_HappyPath verifies the list+filter on
// /edge_configurations matches by name and populates both type and category.
// Note: this data source uses lowercase JSON keys (`id`, `name`, `type`,
// `category`), not the title-case style of the older edge data sources.
func TestDataSourceEdgeConfigurationRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_configurations", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": 1, "name": "other", "type": 0, "category": "general"},
		{"id": 22, "name": "my-cfg", "type": 1, "category": "secret"},
	}))

	ds := dataSourceEdgeConfiguration()
	d := ds.TestResourceData()
	_ = d.Set("name", "my-cfg")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "22" {
		t.Errorf("expected ID %q, got %q", "22", d.Id())
	}
	if got := d.Get("type"); got != 1 {
		t.Errorf("type: expected 1, got %v", got)
	}
	if got := d.Get("category"); got != "secret" {
		t.Errorf("category: expected %q, got %v", "secret", got)
	}
}

// TestDataSourceEdgeConfigurationRead_NotFound verifies error on missing name.
func TestDataSourceEdgeConfigurationRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_configurations", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	ds := dataSourceEdgeConfiguration()
	d := ds.TestResourceData()
	_ = d.Set("name", "missing")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error when edge configuration not found, got nil")
	}
}

// TestDataSourceEdgeConfigurationRead_HTTPError verifies non-200 status is
// surfaced.
func TestDataSourceEdgeConfigurationRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_configurations", RespondString(http.StatusInternalServerError,
		"application/json", `{"message":"boom"}`))

	ds := dataSourceEdgeConfiguration()
	d := ds.TestResourceData()
	_ = d.Set("name", "x")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
