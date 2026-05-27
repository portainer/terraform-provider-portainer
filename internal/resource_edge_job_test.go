package internal

import (
	"net/http"
	"testing"
)

// TestEdgeJobCreate_FromFileContent exercises the JSON-based create path
// using inline file_content. The resource POSTs to /edge_jobs/create/string.
func TestEdgeJobCreate_FromFileContent(t *testing.T) {
	mock := NewMockServer(t)

	// findExistingEdgeJobByName lists all jobs first.
	mock.On("GET", "/edge_jobs", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	mock.On("POST", "/edge_jobs/create/string", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 11,
	}))

	r := resourceEdgeJob()
	d := r.TestResourceData()
	_ = d.Set("name", "cleanup")
	_ = d.Set("cron_expression", "0 * * * *")
	_ = d.Set("edge_groups", []interface{}{1, 2})
	_ = d.Set("endpoints", []interface{}{10})
	_ = d.Set("recurring", true)
	_ = d.Set("file_content", "#!/bin/bash\necho hello")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "11" {
		t.Errorf("expected ID %q, got %q", "11", d.Id())
	}

	post := mock.FindRequest("POST", "/edge_jobs/create/string")
	if post == nil {
		t.Fatal("expected POST to /edge_jobs/create/string")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode POST body: %v", err)
	}
	if got := payload["name"]; got != "cleanup" {
		t.Errorf("payload.name: expected %q, got %v", "cleanup", got)
	}
	if got := payload["cronExpression"]; got != "0 * * * *" {
		t.Errorf("payload.cronExpression: expected %q, got %v", "0 * * * *", got)
	}
	if got := payload["recurring"]; got != true {
		t.Errorf("payload.recurring: expected true, got %v", got)
	}
	if got := payload["fileContent"]; got != "#!/bin/bash\necho hello" {
		t.Errorf("payload.fileContent: unexpected: %v", got)
	}
}

// TestEdgeJobCreate_ExistingNameTriggersUpdate verifies that if a job with the
// same name already exists, Create switches to Update (PUT) instead.
func TestEdgeJobCreate_ExistingNameTriggersUpdate(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_jobs", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 33, "Name": "cleanup"},
	}))

	mock.On("PUT", "/edge_jobs/33", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceEdgeJob()
	d := r.TestResourceData()
	_ = d.Set("name", "cleanup")
	_ = d.Set("cron_expression", "0 * * * *")
	_ = d.Set("edge_groups", []interface{}{1})
	_ = d.Set("endpoints", []interface{}{10})
	_ = d.Set("file_content", "#!/bin/bash\necho updated")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "33" {
		t.Errorf("expected adopted ID %q, got %q", "33", d.Id())
	}
	if mock.FindRequest("PUT", "/edge_jobs/33") == nil {
		t.Error("expected PUT /edge_jobs/33 to be sent")
	}
}

// TestEdgeJobRead_HappyPath verifies the GET response is mapped to state.
// Note: Endpoints in the response is a map keyed by string ID — the resource
// flattens it into a list of integer IDs.
func TestEdgeJobRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_jobs/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"Name":           "nightly",
		"CronExpression": "0 2 * * *",
		"EdgeGroups":     []int{1, 2},
		"Endpoints": map[string]interface{}{
			"5": map[string]interface{}{"LogsStatus": 0},
		},
		"Recurring": true,
	}))

	r := resourceEdgeJob()
	d := r.TestResourceData()
	d.SetId("7")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("name"); got != "nightly" {
		t.Errorf("name: expected %q, got %v", "nightly", got)
	}
	if got := d.Get("cron_expression"); got != "0 2 * * *" {
		t.Errorf("cron_expression: expected %q, got %v", "0 2 * * *", got)
	}
	if got := d.Get("recurring"); got != true {
		t.Errorf("recurring: expected true, got %v", got)
	}
}

// TestEdgeJobRead_404_ClearsID verifies drift detection.
func TestEdgeJobRead_404_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_jobs/99", RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`))

	r := resourceEdgeJob()
	d := r.TestResourceData()
	d.SetId("99")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestEdgeJobDelete_HappyPath verifies DELETE is sent.
func TestEdgeJobDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/edge_jobs/7", RespondString(http.StatusNoContent, "", ""))

	r := resourceEdgeJob()
	d := r.TestResourceData()
	d.SetId("7")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if mock.FindRequest("DELETE", "/edge_jobs/7") == nil {
		t.Error("expected DELETE /edge_jobs/7 to be sent")
	}
}

// TestEdgeJobCreate_HTTPError verifies POST 4xx propagates as an error.
// The file_path branch (multipart upload) is intentionally not exercised here —
// it would require writing a temp file; it shares the same response-decode
// logic as the file_content branch.
func TestEdgeJobCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_jobs", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	mock.On("POST", "/edge_jobs/create/string", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad cron"}`))

	r := resourceEdgeJob()
	d := r.TestResourceData()
	_ = d.Set("name", "bad")
	_ = d.Set("cron_expression", "not-a-cron")
	_ = d.Set("edge_groups", []interface{}{1})
	_ = d.Set("endpoints", []interface{}{1})
	_ = d.Set("file_content", "echo hi")

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
