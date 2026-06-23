package internal

import (
	"net/http"
	"testing"
)

func TestKubernetesCronJobUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/2/kubernetes/apis/batch/v1/namespaces/prod/cronjobs/cj1",
		RespondString(http.StatusOK, "", ""))
	mock.On("POST", "/endpoints/2/kubernetes/apis/batch/v1/namespaces/prod/cronjobs",
		RespondString(http.StatusCreated, "application/json", `{}`))

	r := resourceKubernetesCronJob()
	d := r.TestResourceData()
	d.SetId("2:prod:cj1")
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("namespace", "prod")
	_ = d.Set("manifest", `{"metadata":{"name":"cj1"}}`)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/2/kubernetes/apis/batch/v1/namespaces/prod/cronjobs/cj1") == nil {
		t.Error("expected DELETE during update")
	}
	if mock.FindRequest("POST", "/endpoints/2/kubernetes/apis/batch/v1/namespaces/prod/cronjobs") == nil {
		t.Error("expected POST during update")
	}
}

func TestKubernetesCronJobDelete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/endpoints/1/kubernetes/apis/batch/v1/namespaces/default/cronjobs/x",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesCronJob()
	d := r.TestResourceData()
	d.SetId("1:default:x")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500 delete")
	}
}

func TestKubernetesCronJobDelete_404IsSuccess(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/endpoints/1/kubernetes/apis/batch/v1/namespaces/default/cronjobs/gone",
		RespondString(http.StatusNotFound, "application/json", `{"message":"nf"}`))

	r := resourceKubernetesCronJob()
	d := r.TestResourceData()
	d.SetId("1:default:gone")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("expected 404 delete to succeed, got %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected cleared ID, got %q", d.Id())
	}
}

func TestKubernetesCronJobRead_NoOp(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/kubernetes/apis/batch/v1/namespaces/ns/cronjobs/keep",
		RespondString(http.StatusOK, "application/json", "{}"))

	r := resourceKubernetesCronJob()
	d := r.TestResourceData()
	d.SetId("1:ns:keep")
	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should be no-op, got %v", err)
	}
	if d.Id() != "1:ns:keep" {
		t.Errorf("Read should not change ID, got %q", d.Id())
	}
}

func TestKubernetesCronJobCreate_InvalidManifest(t *testing.T) {
	r := resourceKubernetesCronJob()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", "[unterminated")

	if err := rcCreate(r, d, &APIClient{}); err == nil {
		t.Fatal("expected error for invalid manifest")
	}
}

func TestKubernetesCronJobCreate_MissingMetadata(t *testing.T) {
	r := resourceKubernetesCronJob()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", `{"kind":"CronJob"}`)

	if err := rcCreate(r, d, &APIClient{}); err == nil {
		t.Fatal("expected error for missing metadata")
	}
}

func TestKubernetesCronJobParseID_Malformed(t *testing.T) {
	endpointID, namespace, name := parseCronJobID("bad")
	if endpointID != 0 || namespace != "" || name != "" {
		t.Errorf("expected zero values, got (%d, %q, %q)", endpointID, namespace, name)
	}
}
