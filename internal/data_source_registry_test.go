package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceRegistryRead_HappyPath finds a registry by name and exposes
// its URL/type as computed fields.
func TestDataSourceRegistryRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "docker-hub", "URL": "https://index.docker.io", "Type": 3},
		{"Id": 42, "Name": "harbor", "URL": "https://harbor.example.com", "Type": 6},
	}))

	ds := dataSourceRegistry()
	d := ds.TestResourceData()
	_ = d.Set("name", "harbor")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "42" {
		t.Errorf("expected ID %q, got %q", "42", d.Id())
	}
	if got := d.Get("url"); got != "https://harbor.example.com" {
		t.Errorf("url: expected harbor URL, got %v", got)
	}
	if got := d.Get("type"); got != 6 {
		t.Errorf("type: expected 6, got %v", got)
	}
}

// TestDataSourceRegistryRead_NotFound errors out if no registry matches.
func TestDataSourceRegistryRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "docker-hub", "URL": "https://index.docker.io", "Type": 3},
	}))

	ds := dataSourceRegistry()
	d := ds.TestResourceData()
	_ = d.Set("name", "missing")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error for missing registry, got nil")
	}
}

// TestDataSourceRegistryRead_HTTPError propagates non-2xx.
func TestDataSourceRegistryRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	ds := dataSourceRegistry()
	d := ds.TestResourceData()
	_ = d.Set("name", "harbor")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
