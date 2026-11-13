package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// kubernetes_prune and kubernetes_force_apply (Portainer 3.0) travel in
// DeployerOptions and are sent only when configured, so a configuration that
// leaves them out sends exactly what it did before.

func mockEdgeStackK8s(mock *MockServer, kubernetesOptions map[string]interface{}) {
	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	stack := map[string]interface{}{"Id": 71, "Name": "k8s-edge", "DeploymentType": 1, "EdgeGroups": []int{1}}
	if kubernetesOptions != nil {
		stack["KubernetesOptions"] = kubernetesOptions
	}
	mock.On("GET", "/edge_stacks/71", RespondJSON(http.StatusOK, stack))
}

func TestEdgeStackCreate_KubernetesOptions_String(t *testing.T) {
	mock := NewMockServer(t)
	mockEdgeStackK8s(mock, map[string]interface{}{"Prune": false, "ForceApply": true})
	mock.On("POST", "/edge_stacks/create/string", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 71}))

	r := resourceEdgeStack()
	d := dataWithRawConfig(r, map[string]string{
		"name": "k8s-edge", "deployment_type": "1", "edge_groups.#": "1", "edge_groups.0": "1",
		"stack_file_content": "apiVersion: v1", "kubernetes_prune": "false", "kubernetes_force_apply": "true",
	}, map[string]cty.Value{"kubernetes_prune": cty.False, "kubernetes_force_apply": cty.True})
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	opts, ok := decodeBody(t, mock, "POST", "/edge_stacks/create/string")["deployerOptions"].(map[string]interface{})
	if !ok {
		t.Fatal("expected deployerOptions in the payload")
	}
	if got, ok := opts["KubernetesPrune"]; !ok || got != false {
		t.Errorf("KubernetesPrune: expected false to be sent, got %v (present %v)", got, ok)
	}
	if opts["KubernetesForceApply"] != true {
		t.Errorf("KubernetesForceApply: expected true, got %v", opts["KubernetesForceApply"])
	}
	if _, ok := opts["Prune"]; ok {
		t.Error("the Docker Prune option must not be sent")
	}
	if d.Get("kubernetes_prune").(bool) || !d.Get("kubernetes_force_apply").(bool) {
		t.Errorf("state: expected prune=false force_apply=true from KubernetesOptions, got %v %v",
			d.Get("kubernetes_prune"), d.Get("kubernetes_force_apply"))
	}
}

func TestEdgeStackCreate_KubernetesOptions_OmittedWhenUnset(t *testing.T) {
	mock := NewMockServer(t)
	mockEdgeStackK8s(mock, nil)
	mock.On("POST", "/edge_stacks/create/string", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 71}))

	r := resourceEdgeStack()
	d := dataWithRawConfig(r, map[string]string{
		"name": "k8s-edge", "deployment_type": "1", "edge_groups.#": "1", "edge_groups.0": "1",
		"stack_file_content": "apiVersion: v1",
	}, map[string]cty.Value{"kubernetes_prune": cty.NullVal(cty.Bool)})
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, ok := decodeBody(t, mock, "POST", "/edge_stacks/create/string")["deployerOptions"]; ok {
		t.Error("deployerOptions must not be sent when no Kubernetes option is configured")
	}
}

func TestEdgeStackCreate_KubernetesOptions_NotSentForCompose(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/edge_stacks/create/string", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 72}))
	mock.On("GET", "/edge_stacks/72", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 72, "Name": "c", "DeploymentType": 0}))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	_ = d.Set("name", "c")
	_ = d.Set("deployment_type", 0)
	_ = d.Set("edge_groups", []interface{}{1})
	_ = d.Set("stack_file_content", "services: {}")
	_ = d.Set("kubernetes_prune", true)
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if _, ok := decodeBody(t, mock, "POST", "/edge_stacks/create/string")["deployerOptions"]; ok {
		t.Error("deployerOptions must not be sent for a Docker Compose edge stack")
	}
}

func TestEdgeStackCreate_KubernetesOptions_File(t *testing.T) {
	mock := NewMockServer(t)
	mockEdgeStackK8s(mock, nil)
	mock.On("POST", "/edge_stacks/create/file", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 71}))

	path := filepath.Join(t.TempDir(), "app.yml")
	if err := os.WriteFile(path, []byte("apiVersion: v1"), 0o600); err != nil {
		t.Fatal(err)
	}
	r := resourceEdgeStack()
	d := r.TestResourceData()
	_ = d.Set("name", "k8s-edge")
	_ = d.Set("deployment_type", 1)
	_ = d.Set("edge_groups", []interface{}{1})
	_ = d.Set("stack_file_path", path)
	_ = d.Set("kubernetes_force_apply", true)
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	req := mock.FindRequest("POST", "/edge_stacks/create/file")
	if req == nil {
		t.Fatal("expected POST /edge_stacks/create/file")
	}
	fields, _, _ := parseMultipart(t, req)
	var opts map[string]interface{}
	if err := json.Unmarshal([]byte(fields["DeployerOptions"]), &opts); err != nil {
		t.Fatalf("DeployerOptions form field: %v (%q)", err, fields["DeployerOptions"])
	}
	if opts["KubernetesForceApply"] != true {
		t.Errorf("KubernetesForceApply: expected true, got %v", opts["KubernetesForceApply"])
	}
	if _, ok := opts["KubernetesPrune"]; ok {
		t.Error("KubernetesPrune must not be sent when unset")
	}
	// Not returned (Portainer 2.45): the configured value stays.
	if !d.Get("kubernetes_force_apply").(bool) {
		t.Error("kubernetes_force_apply: expected the configured value to stay in state")
	}
}

func TestEdgeStackCreate_KubernetesOptions_Repository(t *testing.T) {
	mock := NewMockServer(t)
	mockEdgeStackK8s(mock, map[string]interface{}{"ForceApply": false})
	mock.On("POST", "/edge_stacks/create/repository", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 71}))

	r := resourceEdgeStack()
	d := dataWithRawConfig(r, map[string]string{
		"name": "k8s-edge", "deployment_type": "1", "edge_groups.#": "1", "edge_groups.0": "1",
		"repository_url": "https://github.com/acme/edge.git", "kubernetes_prune": "true",
	}, map[string]cty.Value{"kubernetes_prune": cty.True})
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	opts, ok := decodeBody(t, mock, "POST", "/edge_stacks/create/repository")["deployerOptions"].(map[string]interface{})
	if !ok || opts["KubernetesPrune"] != true {
		t.Errorf("expected deployerOptions.KubernetesPrune = true, got %v", opts)
	}
	// A nil Prune (pruning on) is not read back, so the configured value stays.
	if !d.Get("kubernetes_prune").(bool) {
		t.Error("kubernetes_prune: expected the configured true to stay in state")
	}
}

func TestEdgeStackUpdate_KubernetesOptions(t *testing.T) {
	for _, tc := range []struct {
		name, path string
		attrs      map[string]string
	}{
		{"string", "/edge_stacks/71", map[string]string{"stack_file_content": "apiVersion: v1"}},
		{"git", "/edge_stacks/71/git", map[string]string{"repository_url": "https://github.com/acme/edge.git"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockServer(t)
			mockEdgeStackK8s(mock, map[string]interface{}{"Prune": false, "ForceApply": false})
			mock.On("PUT", tc.path, RespondJSON(http.StatusOK, map[string]interface{}{"Id": 71}))

			r := resourceEdgeStack()
			attrs := map[string]string{
				"name": "k8s-edge", "deployment_type": "1", "edge_groups.#": "1", "edge_groups.0": "1",
				"kubernetes_prune": "false", "kubernetes_force_apply": "true",
			}
			for k, v := range tc.attrs {
				attrs[k] = v
			}
			d := dataWithRawConfig(r, attrs, map[string]cty.Value{"kubernetes_prune": cty.False})
			d.SetId("71")
			if err := rcUpdate(r, d, mock.Client()); err != nil {
				t.Fatalf("Update failed: %v", err)
			}
			opts, ok := decodeBody(t, mock, "PUT", tc.path)["deployerOptions"].(map[string]interface{})
			if !ok {
				t.Fatal("expected deployerOptions in the update payload")
			}
			if got, ok := opts["KubernetesPrune"]; !ok || got != false {
				t.Errorf("KubernetesPrune: expected false to be sent, got %v (present %v)", got, ok)
			}
			if _, ok := opts["KubernetesForceApply"]; ok {
				t.Error("KubernetesForceApply must not be sent when the configuration leaves it out")
			}
			if d.Get("kubernetes_force_apply").(bool) {
				t.Error("kubernetes_force_apply: expected the value Portainer returned (false)")
			}
		})
	}
}

func TestEdgeStackDiff_KubernetesOptionsRejectedForCompose(t *testing.T) {
	config := func(deploymentType int) *terraform.ResourceConfig {
		return terraform.NewResourceConfigRaw(map[string]interface{}{
			"name": "c", "deployment_type": deploymentType, "edge_groups": []interface{}{1},
			"stack_file_content": "x", "kubernetes_force_apply": true,
		})
	}
	r := resourceEdgeStack()
	if _, err := r.Diff(context.Background(), nil, config(0), nil); err == nil ||
		!strings.Contains(err.Error(), "kubernetes_force_apply only applies") {
		t.Errorf("expected kubernetes_force_apply to be rejected for Docker Compose, got %v", err)
	}
	if _, err := r.Diff(context.Background(), nil, config(1), nil); err != nil {
		t.Errorf("unexpected error for a Kubernetes edge stack: %v", err)
	}
}
