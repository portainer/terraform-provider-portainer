package internal

import (
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// TestEdgeStackCov4_Create_FilePath_DryRun covers the stack_file_path create
// branch with dryrun=true. In that mode the endpoint gets a "?dryrun=true"
// query string and — because the deploy is only simulated — Create must NOT set
// an ID or chain into Read; it returns nil after the POST. This exercises the
// dryrun query-string construction and the `if !dryrun` false path.
func TestEdgeStackCov4_Create_FilePath_DryRun(t *testing.T) {
	mock := NewMockServer(t)

	// Duplicate-detection list is empty so Create proceeds to the file branch.
	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	// The dispatcher matches on path only; the "?dryrun=true" query is ignored
	// for routing but still built by the resource under test.
	mock.On("POST", "/edge_stacks/create/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   0,
		"Name": "dry",
	}))

	dir := t.TempDir()
	stackPath := filepath.Join(dir, "docker-compose.yml")
	if err := os.WriteFile(stackPath, []byte("version: \"3\"\n"), 0o600); err != nil {
		t.Fatalf("write temp stack file: %v", err)
	}

	r := resourceEdgeStack()
	d := r.TestResourceData()
	_ = d.Set("name", "dry")
	_ = d.Set("deployment_type", 0)
	_ = d.Set("edge_groups", []interface{}{1})
	_ = d.Set("stack_file_path", stackPath)
	_ = d.Set("dryrun", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create (dryrun) failed: %v", err)
	}

	post := mock.FindRequest("POST", "/edge_stacks/create/file")
	if post == nil {
		t.Fatal("expected POST /edge_stacks/create/file")
	}
	if post.Query != "dryrun=true" {
		t.Errorf("expected query %q, got %q", "dryrun=true", post.Query)
	}
	// dryrun must not persist an ID nor chain into Read.
	if d.Id() != "" {
		t.Errorf("expected empty ID on dryrun, got %q", d.Id())
	}
	if mock.FindRequest("GET", "/edge_stacks/0") != nil {
		t.Error("dryrun must not chain into Read")
	}
}
