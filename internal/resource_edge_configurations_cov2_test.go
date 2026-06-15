package internal

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// terraformConfigForEdgeConfig builds a *terraform.ResourceConfig from a raw
// attribute map so resource.Diff (and thus CustomizeDiff) can be driven without
// a live Terraform run.
func terraformConfigForEdgeConfig(raw map[string]interface{}) *terraform.ResourceConfig {
	return terraform.NewResourceConfigRaw(raw)
}

// TestEdgeConfigsCov2_Create_HappyPath exercises the multipart create path.
// The POST returns an empty body so the resolver lists configs and diffs
// against the pre-create snapshot to determine the new ID.
func TestEdgeConfigsCov2_Create_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	dir := t.TempDir()
	fp := filepath.Join(dir, "cfg.txt")
	if err := os.WriteFile(fp, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write tmp: %v", err)
	}

	// Pre-create listing: no same-name config yet.
	listCalls := 0
	mock.On("GET", "/edge_configurations", func(w http.ResponseWriter, r *http.Request) {
		listCalls++
		if listCalls == 1 {
			RespondJSON(http.StatusOK, []map[string]interface{}{})(w, r)
			return
		}
		// Post-create listing now includes the new config.
		RespondJSON(http.StatusOK, []map[string]interface{}{
			{"id": 12, "name": "cfg", "type": 1, "created": 5000},
		})(w, r)
	})

	// POST returns empty body -> triggers list-based resolution.
	mock.On("POST", "/edge_configurations", RespondString(http.StatusOK, "", ""))

	r := resourcePortainerEdgeConfigurations()
	d := r.TestResourceData()
	_ = d.Set("name", "cfg")
	_ = d.Set("type", "general")
	_ = d.Set("category", "configuration")
	_ = d.Set("base_dir", "")
	_ = d.Set("edge_group_ids", []interface{}{1, 2})
	_ = d.Set("file_path", fp)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "12" {
		t.Errorf("expected ID %q, got %q", "12", d.Id())
	}
	if d.Get("file_sha256").(string) == "" {
		t.Error("expected file_sha256 to be populated after create")
	}
}

// TestEdgeConfigsCov2_Create_IDFromResponse covers the branch where the POST
// response already carries a non-zero ID (no list-based resolution needed).
func TestEdgeConfigsCov2_Create_IDFromResponse(t *testing.T) {
	mock := NewMockServer(t)

	dir := t.TempDir()
	fp := filepath.Join(dir, "cfg.txt")
	if err := os.WriteFile(fp, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write tmp: %v", err)
	}

	mock.On("GET", "/edge_configurations", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/edge_configurations", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":   77,
		"name": "cfg",
		"type": 1,
	}))

	r := resourcePortainerEdgeConfigurations()
	d := r.TestResourceData()
	_ = d.Set("name", "cfg")
	_ = d.Set("type", "general")
	_ = d.Set("edge_group_ids", []interface{}{1})
	_ = d.Set("file_path", fp)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "77" {
		t.Errorf("expected ID %q, got %q", "77", d.Id())
	}
}

// TestEdgeConfigsCov2_Create_HTTPError covers the >=400 branch of Create.
func TestEdgeConfigsCov2_Create_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	dir := t.TempDir()
	fp := filepath.Join(dir, "cfg.txt")
	if err := os.WriteFile(fp, []byte("payload"), 0o600); err != nil {
		t.Fatalf("write tmp: %v", err)
	}

	mock.On("GET", "/edge_configurations", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/edge_configurations", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad"}`))

	r := resourcePortainerEdgeConfigurations()
	d := r.TestResourceData()
	_ = d.Set("name", "cfg")
	_ = d.Set("type", "general")
	_ = d.Set("edge_group_ids", []interface{}{1})
	_ = d.Set("file_path", fp)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestEdgeConfigsCov2_Create_OpenFileError covers the os.Open failure branch.
func TestEdgeConfigsCov2_Create_OpenFileError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_configurations", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourcePortainerEdgeConfigurations()
	d := r.TestResourceData()
	_ = d.Set("name", "cfg")
	_ = d.Set("type", "general")
	_ = d.Set("edge_group_ids", []interface{}{1})
	_ = d.Set("file_path", "/nonexistent/path/does-not-exist.txt")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error opening missing file, got nil")
	}
}

// TestEdgeConfigsCov2_Read_HappyPath covers Read populating state, including
// the unknown-type fallback to the numeric string form.
func TestEdgeConfigsCov2_Read_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_configurations/9", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":           9,
		"name":         "cfg",
		"type":         1,
		"category":     "secret",
		"baseDir":      "/data",
		"edgeGroupIDs": []int{4, 5},
	}))

	r := resourcePortainerEdgeConfigurations()
	d := r.TestResourceData()
	d.SetId("9")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("name"); got != "cfg" {
		t.Errorf("name: got %v", got)
	}
	if got := d.Get("type"); got != "general" {
		t.Errorf("type: expected mapped 'general', got %v", got)
	}
	if got := d.Get("category"); got != "secret" {
		t.Errorf("category: got %v", got)
	}
	if got := d.Get("base_dir"); got != "/data" {
		t.Errorf("base_dir: got %v", got)
	}
}

// TestEdgeConfigsCov2_Read_UnknownTypeFallback covers the else branch where the
// numeric type has no string mapping and is stored as a string.
func TestEdgeConfigsCov2_Read_UnknownTypeFallback(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_configurations/3", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":   3,
		"name": "cfg",
		"type": 42,
	}))

	r := resourcePortainerEdgeConfigurations()
	d := r.TestResourceData()
	d.SetId("3")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("type"); got != "42" {
		t.Errorf("type: expected numeric-string fallback '42', got %v", got)
	}
}

// TestEdgeConfigsCov2_Read_404ClearsID covers the drift branch.
func TestEdgeConfigsCov2_Read_404ClearsID(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_configurations/55", RespondString(http.StatusNotFound, "application/json", `{"message":"nope"}`))

	r := resourcePortainerEdgeConfigurations()
	d := r.TestResourceData()
	d.SetId("55")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404, got: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestEdgeConfigsCov2_Read_HTTPError covers the >=400 (non-404) error branch.
func TestEdgeConfigsCov2_Read_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_configurations/55", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourcePortainerEdgeConfigurations()
	d := r.TestResourceData()
	d.SetId("55")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestEdgeConfigsCov2_Update_HappyPath covers the PUT path with a real file and
// the chained Read.
func TestEdgeConfigsCov2_Update_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	dir := t.TempDir()
	fp := filepath.Join(dir, "cfg.txt")
	if err := os.WriteFile(fp, []byte("updated"), 0o600); err != nil {
		t.Fatalf("write tmp: %v", err)
	}

	mock.On("PUT", "/edge_configurations/7", RespondString(http.StatusOK, "", ""))
	mock.On("GET", "/edge_configurations/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":   7,
		"name": "cfg",
		"type": 1,
	}))

	r := resourcePortainerEdgeConfigurations()
	d := r.TestResourceData()
	d.SetId("7")
	_ = d.Set("name", "cfg")
	_ = d.Set("type", "general")
	_ = d.Set("edge_group_ids", []interface{}{1})
	_ = d.Set("file_path", fp)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("PUT", "/edge_configurations/7") == nil {
		t.Error("expected PUT /edge_configurations/7")
	}
	if d.Get("file_sha256").(string) == "" {
		t.Error("expected file_sha256 populated after update")
	}
}

// TestEdgeConfigsCov2_Update_HTTPError covers the >=400 branch of Update.
func TestEdgeConfigsCov2_Update_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	dir := t.TempDir()
	fp := filepath.Join(dir, "cfg.txt")
	if err := os.WriteFile(fp, []byte("x"), 0o600); err != nil {
		t.Fatalf("write tmp: %v", err)
	}

	mock.On("PUT", "/edge_configurations/7", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad"}`))

	r := resourcePortainerEdgeConfigurations()
	d := r.TestResourceData()
	d.SetId("7")
	_ = d.Set("name", "cfg")
	_ = d.Set("type", "general")
	_ = d.Set("edge_group_ids", []interface{}{1})
	_ = d.Set("file_path", fp)

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
}

// TestEdgeConfigsCov2_Update_OpenFileError covers the os.Open failure branch.
func TestEdgeConfigsCov2_Update_OpenFileError(t *testing.T) {
	mock := NewMockServer(t)

	r := resourcePortainerEdgeConfigurations()
	d := r.TestResourceData()
	d.SetId("7")
	_ = d.Set("name", "cfg")
	_ = d.Set("type", "general")
	_ = d.Set("edge_group_ids", []interface{}{1})
	_ = d.Set("file_path", "/nonexistent/missing.txt")

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error opening missing file, got nil")
	}
}

// TestEdgeConfigsCov2_Delete_HappyPath covers a 200 delete.
func TestEdgeConfigsCov2_Delete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/edge_configurations/7", RespondString(http.StatusOK, "", ""))

	r := resourcePortainerEdgeConfigurations()
	d := r.TestResourceData()
	d.SetId("7")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after delete, got %q", d.Id())
	}
}

// TestEdgeConfigsCov2_Delete_404 covers the 404 branch (treated as success).
func TestEdgeConfigsCov2_Delete_404(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/edge_configurations/7", RespondString(http.StatusNotFound, "application/json", `{"message":"gone"}`))

	r := resourcePortainerEdgeConfigurations()
	d := r.TestResourceData()
	d.SetId("7")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow 404, got: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestEdgeConfigsCov2_Delete_HTTPError covers the >=400 (non-404) error branch.
func TestEdgeConfigsCov2_Delete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/edge_configurations/7", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourcePortainerEdgeConfigurations()
	d := r.TestResourceData()
	d.SetId("7")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestEdgeConfigsCov2_CustomizeDiffFileHash_SetsNewHash covers
// customizeDiffEdgeConfigurationFileHash via the resource's Diff machinery:
// with a real file at file_path and no prior file_sha256 in state, the diff
// must include the computed hash for file_sha256 (the SetNew branch).
func TestEdgeConfigsCov2_CustomizeDiffFileHash_SetsNewHash(t *testing.T) {
	dir := t.TempDir()
	fp := filepath.Join(dir, "cfg.txt")
	if err := os.WriteFile(fp, []byte("abc"), 0o600); err != nil {
		t.Fatalf("write tmp: %v", err)
	}

	r := resourcePortainerEdgeConfigurations()

	cfg := terraformConfigForEdgeConfig(map[string]interface{}{
		"name":           "cfg",
		"type":           "general",
		"edge_group_ids": []interface{}{1},
		"file_path":      fp,
	})

	diff, err := r.Diff(context.Background(), nil, cfg, nil)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if diff == nil {
		t.Fatal("expected a non-nil diff")
	}
	want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if got := diff.Attributes["file_sha256"]; got == nil || got.New != want {
		t.Errorf("expected file_sha256 diff New=%q, got %+v", want, got)
	}
}

// TestEdgeConfigsCov2_CustomizeDiffFileHash_MissingFile covers the error branch
// where sha256File fails because file_path points at a missing file.
func TestEdgeConfigsCov2_CustomizeDiffFileHash_MissingFile(t *testing.T) {
	r := resourcePortainerEdgeConfigurations()

	cfg := terraformConfigForEdgeConfig(map[string]interface{}{
		"name":           "cfg",
		"type":           "general",
		"edge_group_ids": []interface{}{1},
		"file_path":      "/nonexistent/missing-config.txt",
	})

	if _, err := r.Diff(context.Background(), nil, cfg, nil); err == nil {
		t.Fatal("expected diff error for missing file_path, got nil")
	}
}

// TestEdgeConfigsCov2_ListEdgeConfigurations covers the listEdgeConfigurations
// helper happy path directly.
func TestEdgeConfigsCov2_ListEdgeConfigurations(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_configurations", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": 1, "name": "a"},
		{"id": 2, "name": "b"},
	}))

	configs, err := listEdgeConfigurations(context.Background(), mock.Client())
	if err != nil {
		t.Fatalf("listEdgeConfigurations: %v", err)
	}
	if len(configs) != 2 {
		t.Fatalf("expected 2 configs, got %d", len(configs))
	}
	if configs[0].Name != "a" || configs[1].ID != 2 {
		t.Errorf("unexpected configs: %+v", configs)
	}
}

// TestEdgeConfigsCov2_ListEdgeConfigurations_DecodeError covers the decode
// failure branch of listEdgeConfigurations.
func TestEdgeConfigsCov2_ListEdgeConfigurations_DecodeError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_configurations", RespondString(http.StatusOK, "application/json", `not-json`))

	if _, err := listEdgeConfigurations(context.Background(), mock.Client()); err == nil {
		t.Fatal("expected decode error, got nil")
	}
}
