package internal

import (
	"encoding/json"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

const deploymentPath = "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments/web"

// updatedDeployment returns ResourceData for the application resource with
// before in state and after as the new manifest, as an apply sees it.
func updatedDeployment(t *testing.T, before, after string) (*schema.Resource, *schema.ResourceData) {
	t.Helper()
	r := resourceKubernetesApplication()
	state := &terraform.InstanceState{ID: "1:default:web", Attributes: map[string]string{
		"id": "1:default:web", "endpoint_id": "1", "namespace": "default", "manifest": before,
	}}
	diff := &terraform.InstanceDiff{Attributes: map[string]*terraform.ResourceAttrDiff{
		"manifest": {Old: before, New: after},
	}}
	d, err := schema.InternalMap(r.SchemaMap()).Data(state, diff)
	if err != nil {
		t.Fatalf("Data failed: %v", err)
	}
	return r, d
}

// TestK8sUpdateManifest_PatchesInPlace: a changed manifest is a merge patch on
// the existing object - a rollout, not a deleted and recreated Deployment.
// Fields dropped from the manifest are nulled, unchanged replicas are left to
// whatever scales the workload.
func TestK8sUpdateManifest_PatchesInPlace(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PATCH", deploymentPath, RespondJSON(http.StatusOK, map[string]interface{}{}))

	r, d := updatedDeployment(t,
		"kind: Deployment\nmetadata:\n  name: web\n  labels:\n    app: web\n    tier: old\nspec:\n  replicas: 2\n  template:\n    spec:\n      containers:\n        - name: web\n          image: nginx:1.29\n",
		"kind: Deployment\nmetadata:\n  name: web\n  labels:\n    app: web\nspec:\n  replicas: 2\n  template:\n    spec:\n      containers:\n        - name: web\n          image: nginx:1.30\n",
	)
	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	for _, req := range mock.Requests() {
		if req.Method == http.MethodDelete || req.Method == http.MethodPost {
			t.Fatalf("an in-place update must not recreate the object, got %s %s", req.Method, req.Path)
		}
	}
	req := mock.FindRequest("PATCH", deploymentPath)
	if req == nil {
		t.Fatal("expected a PATCH of the deployment")
	}
	if ct := req.Headers.Get("Content-Type"); ct != "application/merge-patch+json" {
		t.Errorf("expected a JSON merge patch, got Content-Type %q", ct)
	}
	var patch map[string]interface{}
	if err := json.Unmarshal(req.Body, &patch); err != nil {
		t.Fatalf("patch is not JSON: %v", err)
	}
	labels := patch["metadata"].(map[string]interface{})["labels"].(map[string]interface{})
	if v, ok := labels["tier"]; !ok || v != nil {
		t.Errorf("a label removed from the manifest must be nulled, got labels %v", labels)
	}
	spec := patch["spec"].(map[string]interface{})
	if _, ok := spec["replicas"]; ok {
		t.Errorf("unchanged replicas must not be sent, got spec %v", spec)
	}
	if d.Id() != "1:default:web" {
		t.Errorf("the ID must stay, got %q", d.Id())
	}
}

// TestK8sUpdateManifest_Recreates covers the changes an object cannot take in
// place: they fall back to delete and create, as every update did before.
func TestK8sUpdateManifest_Recreates(t *testing.T) {
	const before = "kind: Deployment\nmetadata:\n  name: web\nspec:\n  selector:\n    matchLabels:\n      app: web\n"
	cases := map[string]struct {
		after   string
		patchRC int
	}{
		"renamed":           {"kind: Deployment\nmetadata:\n  name: web2\n", 0},
		"immutable changed": {"kind: Deployment\nmetadata:\n  name: web\nspec:\n  selector:\n    matchLabels:\n      app: other\n", http.StatusUnprocessableEntity},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			if tc.patchRC != 0 {
				mock.On("PATCH", deploymentPath, RespondJSON(tc.patchRC, map[string]interface{}{"message": "field is immutable"}))
			}
			mock.On("DELETE", deploymentPath, RespondJSON(http.StatusOK, map[string]interface{}{}))
			mock.On("POST", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments", RespondJSON(http.StatusCreated, map[string]interface{}{}))

			r, d := updatedDeployment(t, before, tc.after)
			if err := rcUpdate(r, d, mock.Client()); err != nil {
				t.Fatalf("Update failed: %v", err)
			}
			if mock.FindRequest("DELETE", deploymentPath) == nil || mock.FindRequest("POST", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments") == nil {
				t.Errorf("expected delete and create, got %d requests", len(mock.Requests()))
			}
		})
	}
}

// TestK8sUpdateManifest_PatchError: an error other than a rejected change is
// reported and leaves the object alone.
func TestK8sUpdateManifest_PatchError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PATCH", deploymentPath, RespondJSON(http.StatusForbidden, map[string]interface{}{"message": "forbidden"}))

	r, d := updatedDeployment(t, "kind: Deployment\nmetadata:\n  name: web\n", "kind: Deployment\nmetadata:\n  name: web\n  labels:\n    a: b\n")
	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected the failed PATCH to be reported")
	}
	if mock.FindRequest("DELETE", deploymentPath) != nil {
		t.Error("a failed PATCH must not delete the object")
	}
}

func TestK8sMergePatch(t *testing.T) {
	t.Run("secret key removed from stringData", func(t *testing.T) {
		patch := k8sMergePatch(
			map[string]interface{}{"kind": "Secret", "stringData": map[string]interface{}{"a": "1", "b": "2"}},
			map[string]interface{}{"kind": "Secret", "stringData": map[string]interface{}{"a": "1"}},
		)
		data := patch["data"].(map[string]interface{})
		if v, ok := data["b"]; !ok || v != nil {
			t.Errorf("the removed key must be nulled in data, got %v", patch)
		}
		if data["a"] != "MQ==" {
			t.Errorf("stringData must be sent base64-encoded in data, got %v", patch)
		}
	})
	t.Run("replicas changed", func(t *testing.T) {
		patch := k8sMergePatch(
			map[string]interface{}{"kind": "Deployment", "spec": map[string]interface{}{"replicas": 2}},
			map[string]interface{}{"kind": "Deployment", "spec": map[string]interface{}{"replicas": 3}},
		)
		if patch["spec"].(map[string]interface{})["replicas"] != 3 {
			t.Errorf("changed replicas must be sent, got %v", patch)
		}
	})
	t.Run("lists replaced as a whole", func(t *testing.T) {
		patch := k8sMergePatch(
			map[string]interface{}{"kind": "Role", "rules": []interface{}{"x", "y"}},
			map[string]interface{}{"kind": "Role", "rules": []interface{}{"x"}},
		)
		if rules := patch["rules"].([]interface{}); len(rules) != 1 {
			t.Errorf("expected the new list, got %v", patch)
		}
	})
}

// TestK8sUpdateManifest_RecreateWaitsForDeletion: a deleted object can linger
// while its finalizers run; it is only created again once it is gone, or the
// create fails with "object is being deleted".
func TestK8sUpdateManifest_RecreateWaitsForDeletion(t *testing.T) {
	k8sDeletePollInterval = time.Millisecond
	t.Cleanup(func() { k8sDeletePollInterval = time.Second })

	mock := NewMockServer(t)
	var gets atomic.Int32
	mock.On("GET", deploymentPath, func(w http.ResponseWriter, r *http.Request) {
		if gets.Add(1) < 3 {
			RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
			return
		}
		RespondJSON(http.StatusNotFound, map[string]interface{}{})(w, r)
	})
	mock.On("DELETE", deploymentPath, RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("POST", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments", RespondJSON(http.StatusCreated, map[string]interface{}{}))

	r, d := updatedDeployment(t, "kind: Deployment\nmetadata:\n  name: web\n", "kind: Deployment\nmetadata:\n  name: web2\n")
	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	requests := mock.Requests()
	order := make([]string, 0, len(requests))
	for _, req := range requests {
		order = append(order, req.Method)
	}
	if got := strings.Join(order, " "); got != "DELETE GET GET GET POST" {
		t.Errorf("expected the create to wait for the deletion, got %s", got)
	}
}
