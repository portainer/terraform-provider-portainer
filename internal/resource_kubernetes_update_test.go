package internal

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/hashicorp/terraform-plugin-framework/types"
)

const deploymentPath = "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments/web"

// updateDeployment runs k8sUpdateManifest for the deployment "web" as the
// application resource does, with delete and create sent to the mock.
func updateDeployment(t *testing.T, mock *MockServer, before, after string) error {
	t.Helper()
	r := &k8sManifestResource{kind: k8sKindApplication, client: mock.Client()}
	ctx := context.Background()
	return k8sUpdateManifest(ctx, r.client, mock.URL+deploymentPath, "web", "deployment web", before, after, false,
		func() error { return r.deleteObject(ctx, "1:default:web", time.Minute) },
		func() error {
			_, err := r.createObject(ctx, k8sManifestValues{
				EndpointID: types.Int64Value(1), Namespace: types.StringValue("default"), Manifest: types.StringValue(after),
			}, time.Minute)
			return err
		})
}

// TestK8sUpdateManifest_PatchesInPlace: a changed manifest is a merge patch on
// the existing object - a rollout, not a deleted and recreated Deployment.
// Fields dropped from the manifest are nulled, unchanged replicas are left to
// whatever scales the workload.
func TestK8sUpdateManifest_PatchesInPlace(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("PATCH", deploymentPath, RespondJSON(http.StatusOK, map[string]interface{}{}))

	err := updateDeployment(t, mock,
		"kind: Deployment\nmetadata:\n  name: web\n  labels:\n    app: web\n    tier: old\nspec:\n  replicas: 2\n  template:\n    spec:\n      containers:\n        - name: web\n          image: nginx:1.29\n",
		"kind: Deployment\nmetadata:\n  name: web\n  labels:\n    app: web\nspec:\n  replicas: 2\n  template:\n    spec:\n      containers:\n        - name: web\n          image: nginx:1.30\n",
	)
	if err != nil {
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
}

// TestK8sUpdateManifest_Recreates covers the changes an object cannot take in
// place: they fall back to delete and create, as every update did before.
func TestK8sUpdateManifest_Recreates(t *testing.T) {
	k8sDeletePollInterval = time.Millisecond
	t.Cleanup(func() { k8sDeletePollInterval = time.Second })

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

			if err := updateDeployment(t, mock, before, tc.after); err != nil {
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
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("PATCH", deploymentPath, RespondJSON(http.StatusForbidden, map[string]interface{}{"message": "forbidden"}))

	err := updateDeployment(t, mock, "kind: Deployment\nmetadata:\n  name: web\n", "kind: Deployment\nmetadata:\n  name: web\n  labels:\n    a: b\n")
	if err == nil || !strings.Contains(err.Error(), "failed to update deployment web") {
		t.Fatalf("expected the failed PATCH to be reported, got %v", err)
	}
	if mock.FindRequest("DELETE", deploymentPath) != nil {
		t.Error("a failed PATCH must not delete the object")
	}
}

// TestK8sUpdateManifest_DeleteErrorAborts: when the delete of a recreate
// fails, nothing is created.
func TestK8sUpdateManifest_DeleteErrorAborts(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("DELETE", deploymentPath, RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	err := updateDeployment(t, mock, "kind: Deployment\nmetadata:\n  name: web\n", "kind: Deployment\nmetadata:\n  name: web2\n")
	if err == nil || !strings.Contains(err.Error(), "failed to delete Job") {
		t.Fatalf("expected the failed delete to be reported, got %v", err)
	}
	if mock.FindRequest("POST", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments") != nil {
		t.Error("a failed delete must not be followed by a create")
	}
}

func TestK8sUpdateManifest_IdentityChangedRecreates(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	calls := []string{}
	err := k8sUpdateManifest(context.Background(), mock.Client(), mock.URL+deploymentPath, "web", "deployment web",
		"kind: Deployment\nmetadata:\n  name: web\n", "kind: Deployment\nmetadata:\n  name: web\n", true,
		func() error { calls = append(calls, "delete"); return errors.New("stop") },
		func() error { calls = append(calls, "create"); return nil })
	if err == nil || strings.Join(calls, ",") != "delete" {
		t.Errorf("a changed namespace/type must recreate the object, got calls %v err %v", calls, err)
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("no PATCH expected, got %d requests", len(mock.Requests()))
	}
}

func TestK8sMergePatch(t *testing.T) {
	t.Parallel()
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

	if err := updateDeployment(t, mock, "kind: Deployment\nmetadata:\n  name: web\n", "kind: Deployment\nmetadata:\n  name: web2\n"); err != nil {
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
