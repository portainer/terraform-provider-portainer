package internal

import (
	"context"
	"testing"
)

// TestDockerSecretCov3_Import_HappyPath verifies the importer parses the
// "<endpoint_id>-<secret_id>" composite ID: endpoint_id becomes an attribute
// and the bare secret ID becomes the resource ID.
func TestDockerSecretCov3_Import_HappyPath(t *testing.T) {
	r := resourceDockerSecret()
	d := r.TestResourceData()
	d.SetId("3-sec_1")

	results, err := r.Importer.StateContext(context.Background(), d, nil)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 imported state, got %d", len(results))
	}
	imported := results[0]
	if got := imported.Get("endpoint_id"); got != 3 {
		t.Errorf("endpoint_id: expected 3, got %v", got)
	}
	if imported.Id() != "sec_1" {
		t.Errorf("expected secret ID %q, got %q", "sec_1", imported.Id())
	}
}

// TestDockerSecretCov3_Import_BadFormat rejects an ID with no numeric endpoint
// segment / separator.
func TestDockerSecretCov3_Import_BadFormat(t *testing.T) {
	r := resourceDockerSecret()
	d := r.TestResourceData()
	d.SetId("invalidformat")

	if _, err := r.Importer.StateContext(context.Background(), d, nil); err == nil {
		t.Fatal("expected error for malformed import ID, got nil")
	}
}

// TestDockerSecretCov3_Import_BadEndpointID rejects a non-numeric endpoint
// segment.
func TestDockerSecretCov3_Import_BadEndpointID(t *testing.T) {
	r := resourceDockerSecret()
	d := r.TestResourceData()
	d.SetId("abc-sec_1")

	if _, err := r.Importer.StateContext(context.Background(), d, nil); err == nil {
		t.Fatal("expected error for non-numeric endpoint ID, got nil")
	}
}
