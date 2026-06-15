package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceCloudCredentialsRead_HappyPath verifies list+filter on
// /cloud/credentials by name and populates cloud_provider from `provider`.
func TestDataSourceCloudCredentialsRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/cloud/credentials", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": 1, "name": "aws-old", "provider": "amazon"},
		{"id": 9, "name": "my-gcp", "provider": "googlecloud"},
	}))

	ds := dataSourceCloudCredentials()
	d := ds.TestResourceData()
	_ = d.Set("name", "my-gcp")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "9" {
		t.Errorf("expected ID %q, got %q", "9", d.Id())
	}
	if got := d.Get("cloud_provider"); got != "googlecloud" {
		t.Errorf("cloud_provider: expected %q, got %v", "googlecloud", got)
	}
}

// TestDataSourceCloudCredentialsRead_NotFound verifies error on missing name.
func TestDataSourceCloudCredentialsRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/cloud/credentials", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": 1, "name": "other", "provider": "amazon"},
	}))

	ds := dataSourceCloudCredentials()
	d := ds.TestResourceData()
	_ = d.Set("name", "missing")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error when cloud credential not found, got nil")
	}
}

// TestDataSourceCloudCredentialsRead_HTTPError verifies non-200 status is
// surfaced.
func TestDataSourceCloudCredentialsRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/cloud/credentials", RespondString(http.StatusUnauthorized,
		"application/json", `{"message":"unauthorized"}`))

	ds := dataSourceCloudCredentials()
	d := ds.TestResourceData()
	_ = d.Set("name", "x")

	if err := rcRead(ds, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 401, got nil")
	}
}
