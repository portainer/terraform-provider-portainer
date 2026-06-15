package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceDockerConfigRead_HappyPath lists configs at the env and
// matches by Spec.Name, exposing the API config ID.
func TestDataSourceDockerConfigRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/3/docker/configs", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"ID": "cfg-abc", "Spec": map[string]interface{}{"Name": "other"}},
		{"ID": "cfg-xyz", "Spec": map[string]interface{}{"Name": "app-config"}},
	}))

	ds := dataSourceDockerConfig()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 3)
	_ = d.Set("name", "app-config")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "cfg-xyz" {
		t.Errorf("expected ID %q, got %q", "cfg-xyz", d.Id())
	}
}

// TestDataSourceDockerConfigRead_NotFound errors out if no config matches.
func TestDataSourceDockerConfigRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/3/docker/configs", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"ID": "cfg-abc", "Spec": map[string]interface{}{"Name": "other"}},
	}))

	ds := dataSourceDockerConfig()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 3)
	_ = d.Set("name", "missing")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error for missing docker config, got nil")
	}
}

// TestDataSourceDockerConfigRead_HTTPError surfaces non-2xx as an error.
func TestDataSourceDockerConfigRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/3/docker/configs", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	ds := dataSourceDockerConfig()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 3)
	_ = d.Set("name", "app-config")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
