package internal

import (
	"net/http"
	"testing"
)

func TestKubernetesConfigMapsUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/2/kubernetes/api/v1/namespaces/prod/configmaps/cm1",
		RespondString(http.StatusOK, "", ""))
	mock.On("POST", "/endpoints/2/kubernetes/api/v1/namespaces/prod/configmaps",
		RespondString(http.StatusCreated, "application/json", `{}`))

	r := resourceKubernetesConfigMaps()
	d := r.TestResourceData()
	d.SetId("2:prod:cm1")
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("namespace", "prod")
	_ = d.Set("manifest", `{"metadata":{"name":"cm1"}}`)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/2/kubernetes/api/v1/namespaces/prod/configmaps/cm1") == nil {
		t.Error("expected DELETE during update")
	}
	if mock.FindRequest("POST", "/endpoints/2/kubernetes/api/v1/namespaces/prod/configmaps") == nil {
		t.Error("expected POST during update")
	}
}

func TestKubernetesConfigMapsDelete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/endpoints/1/kubernetes/api/v1/namespaces/default/configmaps/x",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesConfigMaps()
	d := r.TestResourceData()
	d.SetId("1:default:x")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500 delete")
	}
}

func TestKubernetesConfigMapsDelete_404IsSuccess(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/endpoints/1/kubernetes/api/v1/namespaces/default/configmaps/gone",
		RespondString(http.StatusNotFound, "application/json", `{"message":"nf"}`))

	r := resourceKubernetesConfigMaps()
	d := r.TestResourceData()
	d.SetId("1:default:gone")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("expected 404 delete to succeed, got %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected cleared ID, got %q", d.Id())
	}
}

func TestKubernetesConfigMapsRead_NoOp(t *testing.T) {
	r := resourceKubernetesConfigMaps()
	d := r.TestResourceData()
	d.SetId("1:ns:keep")
	if err := rcRead(r, d, nil); err != nil {
		t.Fatalf("Read should be no-op, got %v", err)
	}
	if d.Id() != "1:ns:keep" {
		t.Errorf("Read should not change ID, got %q", d.Id())
	}
}

func TestKubernetesConfigMapsCreate_MissingMetadata(t *testing.T) {
	r := resourceKubernetesConfigMaps()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", `{"kind":"ConfigMap"}`)

	if err := rcCreate(r, d, &APIClient{}); err == nil {
		t.Fatal("expected error for missing metadata")
	}
}

func TestKubernetesConfigMapsCreate_MissingMetadataName(t *testing.T) {
	r := resourceKubernetesConfigMaps()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", `{"metadata":{}}`)

	if err := rcCreate(r, d, &APIClient{}); err == nil {
		t.Fatal("expected error for missing metadata.name")
	}
}
