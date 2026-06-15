package internal

import (
	"net/http"
	"testing"
)

const serviceAccountManifestJSON = `{
  "apiVersion": "v1",
  "kind": "ServiceAccount",
  "metadata": {"name": "deployer"}
}`

// TestKubernetesServiceAccountCreate_HappyPath verifies POST and ID.
func TestKubernetesServiceAccountCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/api/v1/namespaces/default/serviceaccounts",
		RespondJSON(http.StatusCreated, map[string]interface{}{"kind": "ServiceAccount"}))

	r := resourceKubernetesServiceAccounts()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", serviceAccountManifestJSON)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "1:default:deployer" {
		t.Errorf("expected ID %q, got %q", "1:default:deployer", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/1/kubernetes/api/v1/namespaces/default/serviceaccounts")
	if post == nil {
		t.Fatal("expected POST request to be recorded")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["kind"] != "ServiceAccount" {
		t.Errorf("payload.kind: expected %q, got %v", "ServiceAccount", payload["kind"])
	}
}

// TestKubernetesServiceAccountCreate_MissingMetadata verifies fail-fast.
func TestKubernetesServiceAccountCreate_MissingMetadata(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceKubernetesServiceAccounts()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", `{"apiVersion":"v1","kind":"ServiceAccount"}`)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when metadata missing, got nil")
	}
}

// TestKubernetesServiceAccountCreate_HTTPError verifies HTTP error surfaces.
func TestKubernetesServiceAccountCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/api/v1/namespaces/default/serviceaccounts",
		RespondString(http.StatusBadRequest, "application/json", `{"message":"bad"}`))

	r := resourceKubernetesServiceAccounts()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("manifest", serviceAccountManifestJSON)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
}

// TestKubernetesServiceAccountDelete_HappyPath verifies DELETE.
func TestKubernetesServiceAccountDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/kubernetes/api/v1/namespaces/default/serviceaccounts/deployer",
		RespondString(http.StatusNoContent, "", ""))

	r := resourceKubernetesServiceAccounts()
	d := r.TestResourceData()
	d.SetId("1:default:deployer")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoints/1/kubernetes/api/v1/namespaces/default/serviceaccounts/deployer") == nil {
		t.Error("expected DELETE request to be recorded")
	}
}

// TestKubernetesServiceAccountParseID verifies ID parsing.
func TestKubernetesServiceAccountParseID(t *testing.T) {
	endpointID, namespace, name := parseServiceAccountsID("4:apps:sa1")
	if endpointID != 4 || namespace != "apps" || name != "sa1" {
		t.Errorf("expected (4, apps, sa1), got (%d, %q, %q)", endpointID, namespace, name)
	}
}
