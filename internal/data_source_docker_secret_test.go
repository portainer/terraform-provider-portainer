package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceDockerSecretRead_HappyPath matches a secret by Spec.Name.
func TestDataSourceDockerSecretRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/8/docker/secrets", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"ID": "sec-aaa", "Spec": map[string]interface{}{"Name": "other"}},
		{"ID": "sec-bbb", "Spec": map[string]interface{}{"Name": "db-password"}},
	}))

	ds := dataSourceDockerSecret()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 8)
	_ = d.Set("name", "db-password")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "sec-bbb" {
		t.Errorf("expected ID %q, got %q", "sec-bbb", d.Id())
	}
}

// TestDataSourceDockerSecretRead_NotFound errors out if no secret matches.
func TestDataSourceDockerSecretRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/8/docker/secrets", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"ID": "sec-aaa", "Spec": map[string]interface{}{"Name": "other"}},
	}))

	ds := dataSourceDockerSecret()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 8)
	_ = d.Set("name", "missing")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error for missing docker secret, got nil")
	}
}

// TestDataSourceDockerSecretRead_HTTPError surfaces non-2xx as an error.
func TestDataSourceDockerSecretRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/8/docker/secrets", RespondString(
		http.StatusBadGateway, "application/json",
		`{"message":"upstream"}`,
	))

	ds := dataSourceDockerSecret()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 8)
	_ = d.Set("name", "db-password")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 502, got nil")
	}
}
