package internal

import (
	"net/http"
	"testing"
)

func TestKubernetesServiceUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/2/kubernetes/api/v1/namespaces/prod/services/svc1",
		RespondString(http.StatusOK, "", ""))
	mock.On("POST", "/endpoints/2/kubernetes/api/v1/namespaces/prod/services",
		RespondString(http.StatusCreated, "application/json", `{}`))

	r := resourceKubernetesService()
	d := r.TestResourceData()
	d.SetId("2:prod:svc1")
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("namespace", "prod")
	_ = d.Set("manifest", `{"metadata":{"name":"svc1"}}`)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/2/kubernetes/api/v1/namespaces/prod/services/svc1") == nil {
		t.Error("expected DELETE during update")
	}
	if mock.FindRequest("POST", "/endpoints/2/kubernetes/api/v1/namespaces/prod/services") == nil {
		t.Error("expected POST during update")
	}
}

func TestKubernetesServiceDelete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/endpoints/1/kubernetes/api/v1/namespaces/default/services/x",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesService()
	d := r.TestResourceData()
	d.SetId("1:default:x")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500 delete")
	}
}

func TestKubernetesServiceDelete_404IsSuccess(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/endpoints/1/kubernetes/api/v1/namespaces/default/services/gone",
		RespondString(http.StatusNotFound, "application/json", `{"message":"nf"}`))

	r := resourceKubernetesService()
	d := r.TestResourceData()
	d.SetId("1:default:gone")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("expected 404 delete to succeed, got %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected cleared ID, got %q", d.Id())
	}
}

func TestKubernetesServiceRead_NoOp(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/kubernetes/api/v1/namespaces/ns/services/keep",
		RespondString(http.StatusOK, "application/json", "{}"))

	r := resourceKubernetesService()
	d := r.TestResourceData()
	d.SetId("1:ns:keep")
	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should be no-op, got %v", err)
	}
	if d.Id() != "1:ns:keep" {
		t.Errorf("Read should not change ID, got %q", d.Id())
	}
}

func TestKubernetesServiceCreate_InvalidManifest(t *testing.T) {
	r := resourceKubernetesService()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", "[unterminated")

	if err := rcCreate(r, d, &APIClient{}); err == nil {
		t.Fatal("expected error for invalid manifest")
	}
}

func TestKubernetesServiceCreate_MissingMetadata(t *testing.T) {
	r := resourceKubernetesService()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", `{"kind":"Service"}`)

	if err := rcCreate(r, d, &APIClient{}); err == nil {
		t.Fatal("expected error for missing metadata")
	}
}

func TestKubernetesServiceParseID_Malformed(t *testing.T) {
	endpointID, namespace, name := parseServiceID("bad")
	if endpointID != 0 || namespace != "" || name != "" {
		t.Errorf("expected zero values, got (%d, %q, %q)", endpointID, namespace, name)
	}
}
