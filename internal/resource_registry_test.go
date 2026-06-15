package internal

import (
	"net/http"
	"testing"
)

// TestRegistryCreate_HappyPath verifies Create lists registries (to detect
// dupes), POSTs the create payload, and then chains into Read via the
// inspect endpoint.
func TestRegistryCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	// findRegistryByName lists all registries first.
	mock.On("GET", "/registries", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	// Create response carries the new ID in "Id" (capitalized).
	mock.On("POST", "/registries", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":             5,
		"Name":           "dockerhub",
		"URL":            "https://index.docker.io",
		"Type":           3,
		"Authentication": false,
	}))

	// Chained Read after Create.
	mock.On("GET", "/registries/5", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":             5,
		"Name":           "dockerhub",
		"URL":            "https://index.docker.io",
		"BaseURL":        "",
		"Type":           3,
		"Authentication": false,
		"Username":       "",
	}))

	r := resourceRegistry()
	d := r.TestResourceData()
	_ = d.Set("name", "dockerhub")
	_ = d.Set("url", "https://index.docker.io")
	_ = d.Set("type", 3)
	_ = d.Set("authentication", false)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "5" {
		t.Errorf("expected ID %q, got %q", "5", d.Id())
	}
	if got := d.Get("name"); got != "dockerhub" {
		t.Errorf("name: expected %q, got %v", "dockerhub", got)
	}
	if got := d.Get("url"); got != "https://index.docker.io" {
		t.Errorf("url: expected %q, got %v", "https://index.docker.io", got)
	}
	if got := d.Get("type"); got != 3 {
		t.Errorf("type: expected 3, got %v", got)
	}

	post := mock.FindRequest("POST", "/registries")
	if post == nil {
		t.Fatal("expected a POST to /registries")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode POST body: %v", err)
	}
	if got := payload["name"]; got != "dockerhub" {
		t.Errorf("payload.name: expected %q, got %v", "dockerhub", got)
	}
	if got := payload["type"]; got != float64(3) {
		t.Errorf("payload.type: expected 3, got %v", got)
	}
}

// TestRegistryRead_HappyPath verifies Read populates state from the
// inspect endpoint payload.
func TestRegistryRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":             42,
		"Name":           "ghcr",
		"URL":            "ghcr.io",
		"BaseURL":        "",
		"Type":           8,
		"Authentication": true,
		"Username":       "robot",
		"Github": map[string]interface{}{
			"UseOrganisation":  true,
			"OrganisationName": "myorg",
		},
	}))

	r := resourceRegistry()
	d := r.TestResourceData()
	d.SetId("42")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("name"); got != "ghcr" {
		t.Errorf("name: expected %q, got %v", "ghcr", got)
	}
	if got := d.Get("type"); got != 8 {
		t.Errorf("type: expected 8, got %v", got)
	}
	if got := d.Get("authentication"); got != true {
		t.Errorf("authentication: expected true, got %v", got)
	}
	if got := d.Get("github_use_organisation"); got != true {
		t.Errorf("github_use_organisation: expected true, got %v", got)
	}
	if got := d.Get("github_organisation_name"); got != "myorg" {
		t.Errorf("github_organisation_name: expected %q, got %v", "myorg", got)
	}
}

// TestRegistryRead_404ClearsID verifies that a 404 from the inspect endpoint
// silently clears the resource ID for drift detection.
func TestRegistryRead_404ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries/99", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"registry not found"}`,
	))

	r := resourceRegistry()
	d := r.TestResourceData()
	d.SetId("99")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404 and clear ID, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after 404, got %q", d.Id())
	}
}

// TestRegistryUpdate_HappyPath verifies the SDK PUT call is sent and
// then chains into Read.
func TestRegistryUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/registries/12", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   12,
		"Name": "updated",
	}))
	mock.On("GET", "/registries/12", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":             12,
		"Name":           "updated",
		"URL":            "https://example.com",
		"Type":           3,
		"Authentication": false,
	}))

	r := resourceRegistry()
	d := r.TestResourceData()
	d.SetId("12")
	_ = d.Set("name", "updated")
	_ = d.Set("url", "https://example.com")
	_ = d.Set("type", 3)
	_ = d.Set("authentication", false)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if mock.FindRequest("PUT", "/registries/12") == nil {
		t.Error("expected PUT /registries/12 to be sent")
	}
	if got := d.Get("name"); got != "updated" {
		t.Errorf("name: expected %q, got %v", "updated", got)
	}
}

// TestRegistryDelete_HappyPath verifies the SDK DELETE call is sent.
func TestRegistryDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/registries/7", RespondString(http.StatusNoContent, "", ""))

	r := resourceRegistry()
	d := r.TestResourceData()
	d.SetId("7")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if mock.FindRequest("DELETE", "/registries/7") == nil {
		t.Error("expected DELETE /registries/7 to be sent")
	}
}

// TestRegistryCreate_HTTPError verifies that an HTTP 4xx/5xx response on
// create is surfaced as an error rather than silently succeeding.
func TestRegistryCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	// List succeeds (no existing registry).
	mock.On("GET", "/registries", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	// Create fails.
	mock.On("POST", "/registries", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"invalid type"}`,
	))

	r := resourceRegistry()
	d := r.TestResourceData()
	_ = d.Set("name", "bad")
	_ = d.Set("url", "https://example.com")
	_ = d.Set("type", 3)
	_ = d.Set("authentication", false)

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
