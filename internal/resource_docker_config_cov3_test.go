package internal

import (
	"context"
	"net/http"
	"testing"
)

// TestDockerConfigCov3_Import_HappyPath verifies the importer parses the
// "<endpoint_id>-<config_id>" composite ID: endpoint_id becomes an attribute
// and the bare config ID becomes the resource ID.
func TestDockerConfigCov3_Import_HappyPath(t *testing.T) {
	r := resourceDockerConfig()
	d := r.TestResourceData()
	d.SetId("5-cfg123")

	results, err := r.Importer.StateContext(context.Background(), d, nil)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 imported state, got %d", len(results))
	}
	imported := results[0]
	if got := imported.Get("endpoint_id"); got != 5 {
		t.Errorf("endpoint_id: expected 5, got %v", got)
	}
	if imported.Id() != "cfg123" {
		t.Errorf("expected config ID %q, got %q", "cfg123", imported.Id())
	}
}

// TestDockerConfigCov3_Import_BadFormat rejects an ID with no numeric endpoint
// segment / separator.
func TestDockerConfigCov3_Import_BadFormat(t *testing.T) {
	r := resourceDockerConfig()
	d := r.TestResourceData()
	d.SetId("invalidformat")

	if _, err := r.Importer.StateContext(context.Background(), d, nil); err == nil {
		t.Fatal("expected error for malformed import ID, got nil")
	}
}

// TestDockerConfigCov3_Import_BadEndpointID rejects a non-numeric endpoint
// segment.
func TestDockerConfigCov3_Import_BadEndpointID(t *testing.T) {
	r := resourceDockerConfig()
	d := r.TestResourceData()
	d.SetId("abc-cfg123")

	if _, err := r.Importer.StateContext(context.Background(), d, nil); err == nil {
		t.Fatal("expected error for non-numeric endpoint ID, got nil")
	}
}

// TestDockerConfigCov3_Read_HTTPError verifies that a non-200/non-404 response
// on Read is surfaced as an error (the ID is left untouched).
func TestDockerConfigCov3_Read_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/configs/abc", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDockerConfig()
	d := r.TestResourceData()
	d.SetId("abc")
	_ = d.Set("endpoint_id", 1)

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
