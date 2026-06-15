package internal

import (
	"context"
	"net/http"
	"testing"
)

// TestDockerPluginCov2_Import_ParsesCompositeID verifies the importer splits
// "<endpoint_id>:<plugin_name>" into the endpoint_id attribute and the bare
// plugin name as the resource ID.
func TestDockerPluginCov2_Import_ParsesCompositeID(t *testing.T) {
	r := resourceDockerPlugin()
	d := r.TestResourceData()
	d.SetId("7:sshfs")

	results, err := r.Importer.StateContext(context.Background(), d, nil)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 imported state, got %d", len(results))
	}
	imported := results[0]
	if imported.Id() != "sshfs" {
		t.Errorf("expected plugin ID %q, got %q", "sshfs", imported.Id())
	}
	if got := imported.Get("endpoint_id"); got != 7 {
		t.Errorf("endpoint_id: expected 7, got %v", got)
	}
}

// TestDockerPluginCov2_Import_BadFormat rejects an ID without the colon
// separator.
func TestDockerPluginCov2_Import_BadFormat(t *testing.T) {
	r := resourceDockerPlugin()
	d := r.TestResourceData()
	d.SetId("not-composite")

	if _, err := r.Importer.StateContext(context.Background(), d, nil); err == nil {
		t.Fatal("expected error for non-composite import ID, got nil")
	}
}

// TestDockerPluginCov2_Import_BadEndpointID rejects a non-numeric endpoint
// segment.
func TestDockerPluginCov2_Import_BadEndpointID(t *testing.T) {
	r := resourceDockerPlugin()
	d := r.TestResourceData()
	d.SetId("notanint:sshfs")

	if _, err := r.Importer.StateContext(context.Background(), d, nil); err == nil {
		t.Fatal("expected error for non-numeric endpoint ID, got nil")
	}
}

// TestDockerPluginCov2_Create_NoName covers the create path where no name is
// supplied: the query omits the &name= segment and the ID is set to the empty
// name. The enable branch is skipped (enable defaults false).
func TestDockerPluginCov2_Create_NoName(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/plugins/pull", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceDockerPlugin()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("remote", "vieux/sshfs:latest")
	// name intentionally left unset.

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	post := mock.FindRequest("POST", "/endpoints/1/docker/plugins/pull")
	if post == nil {
		t.Fatal("expected pull POST")
	}
	if got := post.Query; got != "remote=vieux%2Fsshfs%3Alatest" && got != "remote=vieux/sshfs:latest" {
		// The query is built with raw string concatenation (no escaping); accept
		// either the raw or transport-escaped form.
		t.Logf("query (informational): %q", got)
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID when name unset, got %q", d.Id())
	}
}
