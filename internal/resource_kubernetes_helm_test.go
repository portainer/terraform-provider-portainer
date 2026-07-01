package internal

import (
	"net/http"
	"testing"
)

// TestKubernetesHelmCreate_HappyPath verifies that Create POSTs to
// /endpoints/{envID}/kubernetes/helm with chart/name/namespace/repo/values
// and sets composite ID "<envID>:<namespace>:<name>".
func TestKubernetesHelmCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/3/kubernetes/helm", RespondJSON(http.StatusCreated, map[string]interface{}{}))
	mock.On("GET", "/endpoints/3/kubernetes/helm/my-nginx", RespondJSON(http.StatusOK, map[string]interface{}{
		"chartReference": map[string]interface{}{
			"chartPath": "nginx",
			"repoURL":   "https://charts.bitnami.com/bitnami",
		},
	}))

	r := resourceKubernetesHelm()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 3)
	_ = d.Set("chart", "nginx")
	_ = d.Set("name", "my-nginx")
	_ = d.Set("namespace", "web")
	_ = d.Set("repo", "https://charts.bitnami.com/bitnami")
	_ = d.Set("values", "replicaCount: 2\n")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "3:web:my-nginx" {
		t.Errorf("expected ID %q, got %q", "3:web:my-nginx", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/3/kubernetes/helm")
	if post == nil {
		t.Fatal("expected POST recorded")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["chart"] != "nginx" {
		t.Errorf("chart: expected nginx, got %v", payload["chart"])
	}
	if payload["name"] != "my-nginx" {
		t.Errorf("name: expected my-nginx, got %v", payload["name"])
	}
	if payload["namespace"] != "web" {
		t.Errorf("namespace: expected web, got %v", payload["namespace"])
	}
	if payload["repo"] != "https://charts.bitnami.com/bitnami" {
		t.Errorf("repo: expected bitnami URL, got %v", payload["repo"])
	}
	if payload["values"] != "replicaCount: 2\n" {
		t.Errorf("values: unexpected, got %v", payload["values"])
	}
}

// TestKubernetesHelmCreate_HTTPError verifies HTTP error propagates.
func TestKubernetesHelmCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/kubernetes/helm",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"helm install failed"}`))

	r := resourceKubernetesHelm()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("chart", "broken")
	_ = d.Set("name", "broken-release")
	_ = d.Set("namespace", "default")
	_ = d.Set("repo", "https://example.com/charts")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestKubernetesHelmDelete_HappyPath verifies DELETE to
// /endpoints/{envID}/kubernetes/helm/{release}?namespace={ns} clears the ID
// when the API returns 204.
func TestKubernetesHelmDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/3/kubernetes/helm/my-nginx",
		RespondString(http.StatusNoContent, "", ""))

	r := resourceKubernetesHelm()
	d := r.TestResourceData()
	d.SetId("3:web:my-nginx")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
	del := mock.FindRequest("DELETE", "/endpoints/3/kubernetes/helm/my-nginx")
	if del == nil {
		t.Fatal("expected DELETE recorded")
	}
	if got := del.Query; got != "namespace=web" {
		t.Errorf("expected query namespace=web, got %q", got)
	}
}

// TestKubernetesHelmRead_Noop verifies Read is a no-op.
func TestKubernetesHelmRead_Noop(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/3/kubernetes/helm/my-nginx",
		RespondJSON(http.StatusOK, map[string]interface{}{"chartReference": map[string]interface{}{"chartPath": "nginx", "repoURL": "https://charts.bitnami.com/bitnami"}}))

	r := resourceKubernetesHelm()
	d := r.TestResourceData()
	d.SetId("3:web:my-nginx")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should be a no-op, got error: %v", err)
	}
}

func TestKubernetesHelmRead_404ClearsID(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/kubernetes/helm/gone",
		RespondString(http.StatusNotFound, "application/json", "{\"message\":\"not found\"}"))

	r := resourceKubernetesHelm()
	d := r.TestResourceData()
	d.SetId("1:default:gone")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read on 404 should not error, got %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared on 404, got %q", d.Id())
	}
}
