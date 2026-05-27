package internal

import (
	"net/http"
	"testing"
)

// TestEdgeGenerateKeyCreate_HappyPath verifies that Create posts to
// /endpoints/edge/generate-key, stores the returned edgeKey, and sets a
// fixed ID.
func TestEdgeGenerateKeyCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/edge/generate-key", RespondJSON(http.StatusOK, map[string]interface{}{
		"edgeKey": "abc123-the-key",
	}))

	r := resourcePortainerEdgeGenerateKey()
	d := r.TestResourceData()

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "portainer-generated-edge-key" {
		t.Errorf("expected fixed ID, got %q", d.Id())
	}
	if got := d.Get("edge_key"); got != "abc123-the-key" {
		t.Errorf("edge_key: expected %q, got %v", "abc123-the-key", got)
	}

	// Verify the request body shape: {"edgeKey":""}.
	post := mock.FindRequest("POST", "/endpoints/edge/generate-key")
	if post == nil {
		t.Fatal("expected POST /endpoints/edge/generate-key")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode POST body: %v", err)
	}
	if v, ok := payload["edgeKey"]; !ok || v != "" {
		t.Errorf("expected payload {edgeKey: \"\"}, got %v", payload)
	}
}

// TestEdgeGenerateKeyCreate_HTTPError verifies that a 4xx surfaces an error
// and leaves edge_key empty.
func TestEdgeGenerateKeyCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/edge/generate-key", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"forbidden"}`,
	))

	r := resourcePortainerEdgeGenerateKey()
	d := r.TestResourceData()

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if got := d.Get("edge_key"); got != "" {
		t.Errorf("expected empty edge_key after error, got %v", got)
	}
}

// TestEdgeGenerateKeyRead_NoOp verifies Read is schema.Noop.
func TestEdgeGenerateKeyRead_NoOp(t *testing.T) {
	mock := NewMockServer(t)

	r := resourcePortainerEdgeGenerateKey()
	d := r.TestResourceData()
	d.SetId("portainer-generated-edge-key")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected zero requests for Noop Read, got %d", len(mock.Requests()))
	}
}

// TestEdgeGenerateKeyDelete_RemoveFromState verifies Delete uses
// schema.RemoveFromState semantics: ID is cleared, no API call made.
func TestEdgeGenerateKeyDelete_RemoveFromState(t *testing.T) {
	mock := NewMockServer(t)

	r := resourcePortainerEdgeGenerateKey()
	d := r.TestResourceData()
	d.SetId("portainer-generated-edge-key")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected zero requests for RemoveFromState Delete, got %d", len(mock.Requests()))
	}
}
