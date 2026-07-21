package internal

import (
	"net/http"
	"testing"
)

// TestDockerNodeCov2_Update_JWTAuth covers the JWT (Authorization: Bearer)
// authentication branch of the update handler by clearing the API key and
// setting a JWT token on the client.
func TestDockerNodeCov2_Update_JWTAuth(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/nodes/n1/update", RespondJSON(http.StatusOK, map[string]interface{}{}))

	client := mock.Client()
	client.APIKey = ""
	client.JWTToken = "jwt-token"

	r := resourceDockerNode()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("node_id", "n1")
	_ = d.Set("version", 3)

	if err := rcCreate(r, d, client); err != nil {
		t.Fatalf("Update via JWT failed: %v", err)
	}
	if d.Id() != "1-n1" {
		t.Errorf("expected ID %q, got %q", "1-n1", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/1/docker/nodes/n1/update")
	if post == nil {
		t.Fatal("expected POST to node update endpoint")
	}
	if got := post.Headers.Get("Authorization"); got != "Bearer jwt-token" {
		t.Errorf("Authorization header: expected %q, got %q", "Bearer jwt-token", got)
	}
}

// TestDockerNodeCov2_Update_NoAuth covers the branch where neither an API key
// nor a JWT token is configured: the handler must return an error before any
// HTTP request is issued.
func TestDockerNodeCov2_Update_NoAuth(t *testing.T) {
	mock := NewMockServer(t)

	client := mock.Client()
	client.APIKey = ""
	client.JWTToken = ""

	r := resourceDockerNode()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("node_id", "n1")
	_ = d.Set("version", 1)

	if err := rcCreate(r, d, client); err == nil {
		t.Fatal("expected error when no auth method is configured, got nil")
	}
}

// TestDockerNodeCov2_Read_NoAuth covers the no-auth error branch of Read.
func TestDockerNodeCov2_Read_NoAuth(t *testing.T) {
	mock := NewMockServer(t)

	client := mock.Client()
	client.APIKey = ""
	client.JWTToken = ""

	r := resourceDockerNode()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("node_id", "n1")

	if err := rcRead(r, d, client); err == nil {
		t.Fatal("expected error when no auth method is configured, got nil")
	}
}

// TestDockerNodeCov2_Read_DecodeError covers the JSON decode-error branch of
// Read: a 200 response whose body is not valid JSON must surface an error.
func TestDockerNodeCov2_Read_DecodeError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/nodes/n1", RespondString(
		http.StatusOK, "application/json", `{ not valid json`,
	))

	r := resourceDockerNode()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("node_id", "n1")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected decode error on malformed JSON, got nil")
	}
}

// TestDockerNodeCov2_Delete_NoAuth covers the no-auth error branch of Delete.
func TestDockerNodeCov2_Delete_NoAuth(t *testing.T) {
	mock := NewMockServer(t)

	client := mock.Client()
	client.APIKey = ""
	client.JWTToken = ""

	r := resourceDockerNode()
	d := r.TestResourceData()
	d.SetId("1-n1")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("node_id", "n1")

	if err := rcDelete(r, d, client); err == nil {
		t.Fatal("expected error when no auth method is configured, got nil")
	}
}
