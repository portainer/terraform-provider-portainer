package internal

import (
	"net/http"
	"testing"
)

// TestEdgeJobCov3_Create_NoSourceError covers the final error branch of Create
// when neither file_content nor file_path is provided. Driving the handler
// directly bypasses the schema's ExactlyOneOf validation, so the code reaches
// the terminal "either file_content or file_path must be provided" error after
// the existing-name lookup returns no match.
func TestEdgeJobCov3_Create_NoSourceError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_jobs", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceEdgeJob()
	d := r.TestResourceData()
	_ = d.Set("name", "nosource")
	_ = d.Set("cron_expression", "0 * * * *")
	_ = d.Set("edge_groups", []interface{}{1})
	_ = d.Set("endpoints", []interface{}{10})

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when neither file_content nor file_path set, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestEdgeJobCov3_Create_FilePathOpenError covers the os.Open failure branch of
// the file_path create path: after the existing-name lookup returns no match,
// opening a non-existent file must surface as an error.
func TestEdgeJobCov3_Create_FilePathOpenError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_jobs", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceEdgeJob()
	d := r.TestResourceData()
	_ = d.Set("name", "badfile")
	_ = d.Set("cron_expression", "0 * * * *")
	_ = d.Set("edge_groups", []interface{}{1})
	_ = d.Set("endpoints", []interface{}{10})
	_ = d.Set("file_path", "/nonexistent/path/does-not-exist.sh")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error opening missing file, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
