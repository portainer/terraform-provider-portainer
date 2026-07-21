package internal

import (
	"context"
	"testing"
)

// TestDockerVolumeCov3_Import_HappyPath verifies the importer parses the
// "<endpoint_id>-<volume_name>" composite ID into the endpoint_id and name
// attributes and normalizes the resource ID back to the composite form.
func TestDockerVolumeCov3_Import_HappyPath(t *testing.T) {
	r := resourceDockerVolume()
	d := r.TestResourceData()
	d.SetId("7-my-vol")

	results, err := r.Importer.StateContext(context.Background(), d, nil)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 imported state, got %d", len(results))
	}
	imported := results[0]
	if got := imported.Get("endpoint_id"); got != 7 {
		t.Errorf("endpoint_id: expected 7, got %v", got)
	}
	if got := imported.Get("name"); got != "my-vol" {
		t.Errorf("name: expected %q, got %v", "my-vol", got)
	}
	if imported.Id() != "7-my-vol" {
		t.Errorf("expected ID %q, got %q", "7-my-vol", imported.Id())
	}
}

// TestDockerVolumeCov3_Import_BadFormat rejects an ID that has no numeric
// endpoint segment / separator.
func TestDockerVolumeCov3_Import_BadFormat(t *testing.T) {
	r := resourceDockerVolume()
	d := r.TestResourceData()
	d.SetId("invalidformat")

	if _, err := r.Importer.StateContext(context.Background(), d, nil); err == nil {
		t.Fatal("expected error for malformed import ID, got nil")
	}
}

// TestDockerVolumeCov3_Import_BadEndpointID rejects a non-numeric endpoint
// segment.
func TestDockerVolumeCov3_Import_BadEndpointID(t *testing.T) {
	r := resourceDockerVolume()
	d := r.TestResourceData()
	d.SetId("abc-my-vol")

	if _, err := r.Importer.StateContext(context.Background(), d, nil); err == nil {
		t.Fatal("expected error for non-numeric endpoint ID, got nil")
	}
}
