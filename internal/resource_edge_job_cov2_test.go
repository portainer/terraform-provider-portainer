package internal

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// TestEdgeJobCov2_Update_HappyPath drives Update directly (PUT /edge_jobs/{id}),
// including the file_content branch that adds fileContent to the payload.
func TestEdgeJobCov2_Update_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/edge_jobs/7", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceEdgeJob()
	d := r.TestResourceData()
	d.SetId("7")
	_ = d.Set("name", "nightly")
	_ = d.Set("cron_expression", "0 3 * * *")
	_ = d.Set("edge_groups", []interface{}{1})
	_ = d.Set("endpoints", []interface{}{10})
	_ = d.Set("recurring", true)
	_ = d.Set("file_content", "echo hi")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/edge_jobs/7")
	if put == nil {
		t.Fatal("expected PUT /edge_jobs/7")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode PUT body: %v", err)
	}
	if got := payload["name"]; got != "nightly" {
		t.Errorf("payload.name: got %v", got)
	}
	if got := payload["cronExpression"]; got != "0 3 * * *" {
		t.Errorf("payload.cronExpression: got %v", got)
	}
	if got := payload["fileContent"]; got != "echo hi" {
		t.Errorf("payload.fileContent: got %v", got)
	}
}

// TestEdgeJobCov2_Update_HTTPError covers the non-200 branch of Update.
func TestEdgeJobCov2_Update_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/edge_jobs/7", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad"}`))

	r := resourceEdgeJob()
	d := r.TestResourceData()
	d.SetId("7")
	_ = d.Set("name", "nightly")
	_ = d.Set("cron_expression", "0 3 * * *")
	_ = d.Set("edge_groups", []interface{}{1})
	_ = d.Set("endpoints", []interface{}{10})

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
}

// TestEdgeJobCov2_Read_HTTPError covers the non-404 error branch of Read.
func TestEdgeJobCov2_Read_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_jobs/7", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceEdgeJob()
	d := r.TestResourceData()
	d.SetId("7")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestEdgeJobCov2_Delete_HTTPError covers the non-204 branch of Delete.
func TestEdgeJobCov2_Delete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/edge_jobs/7", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceEdgeJob()
	d := r.TestResourceData()
	d.SetId("7")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on non-204 delete, got nil")
	}
}

// TestEdgeJobCov2_Create_FromFilePath covers the multipart file_path create
// branch (POST /edge_jobs/create/file).
func TestEdgeJobCov2_Create_FromFilePath(t *testing.T) {
	mock := NewMockServer(t)

	dir := t.TempDir()
	fp := filepath.Join(dir, "job.sh")
	if err := os.WriteFile(fp, []byte("#!/bin/sh\necho run\n"), 0o600); err != nil {
		t.Fatalf("write tmp: %v", err)
	}

	mock.On("GET", "/edge_jobs", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/edge_jobs/create/file", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 88,
	}))

	r := resourceEdgeJob()
	d := r.TestResourceData()
	_ = d.Set("name", "fromfile")
	_ = d.Set("cron_expression", "0 * * * *")
	_ = d.Set("edge_groups", []interface{}{1})
	_ = d.Set("endpoints", []interface{}{10})
	_ = d.Set("recurring", false)
	_ = d.Set("file_path", fp)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "88" {
		t.Errorf("expected ID %q, got %q", "88", d.Id())
	}
	if mock.FindRequest("POST", "/edge_jobs/create/file") == nil {
		t.Error("expected POST /edge_jobs/create/file")
	}
}

// TestEdgeJobCov2_Create_FromFilePath_HTTPError covers the non-200 branch of
// the multipart file_path create path.
func TestEdgeJobCov2_Create_FromFilePath_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	dir := t.TempDir()
	fp := filepath.Join(dir, "job.sh")
	if err := os.WriteFile(fp, []byte("echo x"), 0o600); err != nil {
		t.Fatalf("write tmp: %v", err)
	}

	mock.On("GET", "/edge_jobs", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/edge_jobs/create/file", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad"}`))

	r := resourceEdgeJob()
	d := r.TestResourceData()
	_ = d.Set("name", "fromfile")
	_ = d.Set("cron_expression", "0 * * * *")
	_ = d.Set("edge_groups", []interface{}{1})
	_ = d.Set("endpoints", []interface{}{10})
	_ = d.Set("file_path", fp)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
}

// TestEdgeJobCov2_Create_ListError covers the findExistingEdgeJobByName failure
// path (the list GET returns a non-200), surfaced as a create error.
func TestEdgeJobCov2_Create_ListError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_jobs", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceEdgeJob()
	d := r.TestResourceData()
	_ = d.Set("name", "x")
	_ = d.Set("cron_expression", "0 * * * *")
	_ = d.Set("edge_groups", []interface{}{1})
	_ = d.Set("endpoints", []interface{}{10})
	_ = d.Set("file_content", "echo hi")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when listing existing jobs fails, got nil")
	}
}

// TestEdgeJobCov2_FindExistingByName covers findExistingEdgeJobByName directly:
// a match returns its ID; a non-match returns 0.
func TestEdgeJobCov2_FindExistingByName(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_jobs", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 3, "Name": "alpha"},
		{"Id": 4, "Name": "beta"},
	}))

	client := mock.Client()

	id, err := findExistingEdgeJobByName(context.Background(), client, "beta")
	if err != nil {
		t.Fatalf("findExistingEdgeJobByName: %v", err)
	}
	if id != 4 {
		t.Errorf("expected id 4, got %d", id)
	}

	id, err = findExistingEdgeJobByName(context.Background(), client, "missing")
	if err != nil {
		t.Fatalf("findExistingEdgeJobByName: %v", err)
	}
	if id != 0 {
		t.Errorf("expected id 0 for missing name, got %d", id)
	}
}
