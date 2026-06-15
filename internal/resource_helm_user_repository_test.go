package internal

import (
	"net/http"
	"testing"
)

// TestHelmUserRepositoryCreate_HappyPath verifies POST to
// /users/{userID}/helm/repositories with the URL, ID derived from response.
func TestHelmUserRepositoryCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/users/7/helm/repositories", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":     11,
		"URL":    "https://charts.bitnami.com/bitnami",
		"UserId": 7,
	}))

	r := resourceHelmUserRepository()
	d := r.TestResourceData()
	_ = d.Set("user_id", 7)
	_ = d.Set("url", "https://charts.bitnami.com/bitnami")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "11" {
		t.Errorf("expected ID %q, got %q", "11", d.Id())
	}
	if got := d.Get("url"); got != "https://charts.bitnami.com/bitnami" {
		t.Errorf("url: expected bitnami, got %v", got)
	}

	post := mock.FindRequest("POST", "/users/7/helm/repositories")
	if post == nil {
		t.Fatal("expected POST recorded")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["url"] != "https://charts.bitnami.com/bitnami" {
		t.Errorf("payload.url: expected bitnami, got %v", payload["url"])
	}
}

// TestHelmUserRepositoryCreate_HTTPError verifies HTTP 4xx surfaces.
func TestHelmUserRepositoryCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/users/7/helm/repositories",
		RespondString(http.StatusBadRequest, "application/json", `{"message":"bad url"}`))

	r := resourceHelmUserRepository()
	d := r.TestResourceData()
	_ = d.Set("user_id", 7)
	_ = d.Set("url", "https://example.com/charts")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestHelmUserRepositoryRead_HappyPath verifies Read GETs the user's
// repositories list and finds the matching repo by ID, populating url.
func TestHelmUserRepositoryRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users/7/helm/repositories", RespondJSON(http.StatusOK, map[string]interface{}{
		"UserRepositories": []map[string]interface{}{
			{"Id": 11, "URL": "https://charts.bitnami.com/bitnami", "UserId": 7},
			{"Id": 12, "URL": "https://other.example.com", "UserId": 7},
		},
	}))

	r := resourceHelmUserRepository()
	d := r.TestResourceData()
	_ = d.Set("user_id", 7)
	d.SetId("11")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("url"); got != "https://charts.bitnami.com/bitnami" {
		t.Errorf("url: expected bitnami, got %v", got)
	}
	if got := d.Get("user_id"); got != 7 {
		t.Errorf("user_id: expected 7, got %v", got)
	}
}

// TestHelmUserRepositoryRead_NotFound_ClearsID verifies Read clears ID when
// the repository is missing from the user's list.
func TestHelmUserRepositoryRead_NotFound_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/users/7/helm/repositories", RespondJSON(http.StatusOK, map[string]interface{}{
		"UserRepositories": []map[string]interface{}{},
	}))

	r := resourceHelmUserRepository()
	d := r.TestResourceData()
	_ = d.Set("user_id", 7)
	d.SetId("99")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared when not found, got %q", d.Id())
	}
}

// TestHelmUserRepositoryDelete_HappyPath verifies DELETE to
// /users/{userID}/helm/repositories/{repoID}.
func TestHelmUserRepositoryDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/users/7/helm/repositories/11", RespondString(http.StatusNoContent, "", ""))

	r := resourceHelmUserRepository()
	d := r.TestResourceData()
	_ = d.Set("user_id", 7)
	d.SetId("11")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
	if mock.FindRequest("DELETE", "/users/7/helm/repositories/11") == nil {
		t.Fatal("expected DELETE recorded")
	}
}
