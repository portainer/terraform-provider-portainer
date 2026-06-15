package internal

import (
	"net/http"
	"testing"
)

const cronJobManifestJSON = `{
  "apiVersion": "batch/v1",
  "kind": "CronJob",
  "metadata": {"name": "nightly"},
  "spec": {"schedule": "0 0 * * *"}
}`

// TestKubernetesCronJobCreate_HappyPath verifies that Create POSTs to the
// cronjobs endpoint and builds the composite ID.
func TestKubernetesCronJobCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/2/kubernetes/apis/batch/v1/namespaces/prod/cronjobs", RespondJSON(http.StatusCreated, map[string]interface{}{
		"kind": "CronJob",
	}))

	r := resourceKubernetesCronJob()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("namespace", "prod")
	_ = d.Set("manifest", cronJobManifestJSON)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "2:prod:nightly" {
		t.Errorf("expected ID %q, got %q", "2:prod:nightly", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/2/kubernetes/apis/batch/v1/namespaces/prod/cronjobs")
	if post == nil {
		t.Fatal("expected POST request to be recorded")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["kind"] != "CronJob" {
		t.Errorf("payload.kind: expected %q, got %v", "CronJob", payload["kind"])
	}
}

// TestKubernetesCronJobCreate_MissingMetadataName verifies fail-fast when
// metadata.name is missing.
func TestKubernetesCronJobCreate_MissingMetadataName(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesCronJob()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", `{"kind":"CronJob","metadata":{}}`)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when metadata.name missing, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestKubernetesCronJobCreate_HTTPError verifies HTTP error surfaces.
func TestKubernetesCronJobCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/apis/batch/v1/namespaces/default/cronjobs", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"bad request"}`,
	))

	r := resourceKubernetesCronJob()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", cronJobManifestJSON)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
}

// TestKubernetesCronJobDelete_HappyPath verifies DELETE is sent and ID cleared.
func TestKubernetesCronJobDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/2/kubernetes/apis/batch/v1/namespaces/prod/cronjobs/nightly", RespondString(
		http.StatusNoContent, "", "",
	))

	r := resourceKubernetesCronJob()
	d := r.TestResourceData()
	d.SetId("2:prod:nightly")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/2/kubernetes/apis/batch/v1/namespaces/prod/cronjobs/nightly") == nil {
		t.Error("expected DELETE request to be recorded")
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestKubernetesCronJobParseID verifies ID parsing.
func TestKubernetesCronJobParseID(t *testing.T) {
	endpointID, namespace, name := parseCronJobID("7:cron:weekly")
	if endpointID != 7 || namespace != "cron" || name != "weekly" {
		t.Errorf("expected (7, cron, weekly), got (%d, %q, %q)", endpointID, namespace, name)
	}
}
