package internal

import (
	"net/http"
	"testing"
)

const jobManifestJSON = `{
  "apiVersion": "batch/v1",
  "kind": "Job",
  "metadata": {"name": "backup"},
  "spec": {"template": {"spec": {"containers": [{"name": "c", "image": "busybox"}]}}}
}`

// TestKubernetesJobCreate_HappyPath verifies that Create POSTs to the jobs
// endpoint and builds the composite ID.
func TestKubernetesJobCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/2/kubernetes/apis/batch/v1/namespaces/prod/jobs", RespondJSON(http.StatusCreated, map[string]interface{}{
		"kind": "Job",
	}))

	r := resourceKubernetesJob()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("namespace", "prod")
	_ = d.Set("manifest", jobManifestJSON)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "2:prod:backup" {
		t.Errorf("expected ID %q, got %q", "2:prod:backup", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/2/kubernetes/apis/batch/v1/namespaces/prod/jobs")
	if post == nil {
		t.Fatal("expected POST request to be recorded")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["kind"] != "Job" {
		t.Errorf("payload.kind: expected %q, got %v", "Job", payload["kind"])
	}
}

// TestKubernetesJobCreate_MissingMetadataName verifies fail-fast when
// metadata.name is missing.
func TestKubernetesJobCreate_MissingMetadataName(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesJob()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", `{"kind":"Job","metadata":{}}`)

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error when metadata.name missing, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestKubernetesJobCreate_HTTPError verifies HTTP error surfaces.
func TestKubernetesJobCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/apis/batch/v1/namespaces/default/jobs", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	r := resourceKubernetesJob()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", jobManifestJSON)

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestKubernetesJobDelete_HappyPath verifies DELETE is sent and ID cleared.
func TestKubernetesJobDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/2/kubernetes/apis/batch/v1/namespaces/prod/jobs/backup", RespondString(
		http.StatusNoContent, "", "",
	))

	r := resourceKubernetesJob()
	d := r.TestResourceData()
	d.SetId("2:prod:backup")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/2/kubernetes/apis/batch/v1/namespaces/prod/jobs/backup") == nil {
		t.Error("expected DELETE request to be recorded")
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestKubernetesJobParseID verifies ID parsing.
func TestKubernetesJobParseID(t *testing.T) {
	endpointID, namespace, name := parseJobID("5:batch:job1")
	if endpointID != 5 || namespace != "batch" || name != "job1" {
		t.Errorf("expected (5, batch, job1), got (%d, %q, %q)", endpointID, namespace, name)
	}
}
