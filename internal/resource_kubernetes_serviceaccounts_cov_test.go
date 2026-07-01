package internal

import (
	"net/http"
	"testing"
)

func TestKubernetesServiceAccountUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/2/kubernetes/api/v1/namespaces/prod/serviceaccounts/sa1",
		RespondString(http.StatusOK, "", ""))
	mock.On("POST", "/endpoints/2/kubernetes/api/v1/namespaces/prod/serviceaccounts",
		RespondString(http.StatusCreated, "application/json", `{}`))

	r := resourceKubernetesServiceAccounts()
	d := r.TestResourceData()
	d.SetId("2:prod:sa1")
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("namespace", "prod")
	_ = d.Set("manifest", `{"metadata":{"name":"sa1"}}`)

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/2/kubernetes/api/v1/namespaces/prod/serviceaccounts/sa1") == nil {
		t.Error("expected DELETE during update")
	}
	if mock.FindRequest("POST", "/endpoints/2/kubernetes/api/v1/namespaces/prod/serviceaccounts") == nil {
		t.Error("expected POST during update")
	}
}

func TestKubernetesServiceAccountDelete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/endpoints/1/kubernetes/api/v1/namespaces/default/serviceaccounts/x",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesServiceAccounts()
	d := r.TestResourceData()
	d.SetId("1:default:x")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500 delete")
	}
}

func TestKubernetesServiceAccountDelete_404IsSuccess(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/endpoints/1/kubernetes/api/v1/namespaces/default/serviceaccounts/gone",
		RespondString(http.StatusNotFound, "application/json", `{"message":"nf"}`))

	r := resourceKubernetesServiceAccounts()
	d := r.TestResourceData()
	d.SetId("1:default:gone")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("expected 404 delete to succeed, got %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected cleared ID, got %q", d.Id())
	}
}

func TestKubernetesServiceAccountRead_NoOp(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/kubernetes/api/v1/namespaces/ns/serviceaccounts/keep",
		RespondString(http.StatusOK, "application/json", "{}"))

	r := resourceKubernetesServiceAccounts()
	d := r.TestResourceData()
	d.SetId("1:ns:keep")
	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should be no-op, got %v", err)
	}
	if d.Id() != "1:ns:keep" {
		t.Errorf("Read should not change ID, got %q", d.Id())
	}
}

func TestKubernetesServiceAccountCreate_InvalidManifest(t *testing.T) {
	r := resourceKubernetesServiceAccounts()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", "[unterminated")

	if err := rcCreate(r, d, &APIClient{}); err == nil {
		t.Fatal("expected error for invalid manifest")
	}
}

func TestKubernetesServiceAccountCreate_MissingMetadataName(t *testing.T) {
	r := resourceKubernetesServiceAccounts()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", `{"metadata":{}}`)

	if err := rcCreate(r, d, &APIClient{}); err == nil {
		t.Fatal("expected error for missing metadata.name")
	}
}

func TestKubernetesServiceAccountParseID_Malformed(t *testing.T) {
	endpointID, namespace, name := parseServiceAccountsID("bad")
	if endpointID != 0 || namespace != "" || name != "" {
		t.Errorf("expected zero values, got (%d, %q, %q)", endpointID, namespace, name)
	}
}

func TestKubernetesServiceAccountRead_404ClearsID(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/kubernetes/api/v1/namespaces/ns/serviceaccounts/gone",
		RespondString(http.StatusNotFound, "application/json", "{\"message\":\"not found\"}"))

	r := resourceKubernetesServiceAccounts()
	d := r.TestResourceData()
	d.SetId("1:ns:gone")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read on 404 should not error, got %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared on 404, got %q", d.Id())
	}
}
