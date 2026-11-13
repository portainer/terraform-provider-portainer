package internal

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// kubernetes_prune and kubernetes_force_apply (Portainer 3.0) are sent only
// when configured, so a configuration that leaves them out sends exactly what
// it did before and works unchanged against Portainer 2.45.

func mockK8sStringStack(mock *MockServer, id string, option map[string]interface{}) {
	mockEmptyStackList(mock)
	mock.On("POST", "/stacks/create/kubernetes/string", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 61}))
	mock.On("PUT", "/stacks/"+id, RespondJSON(http.StatusOK, map[string]interface{}{"Id": 61}))
	stack := map[string]interface{}{
		"Id": 61, "Name": "k8s", "Status": 1, "Type": 3, "EndpointId": 2, "namespace": "default",
	}
	if option != nil {
		stack["Option"] = option
	}
	mock.On("GET", "/stacks/"+id, RespondJSON(http.StatusOK, stack))
	mock.On("GET", "/stacks/"+id+"/file", RespondJSON(http.StatusOK, map[string]interface{}{"StackFileContent": "apiVersion: v1"}))
}

func k8sStringStackAttrs() map[string]string {
	return map[string]string{
		"deployment_type": "kubernetes", "method": "string", "name": "k8s", "endpoint_id": "2",
		"namespace": "default", "stack_file_content": "apiVersion: v1", "active": "true",
	}
}

func TestStackCreate_KubernetesOptions_SentWhenConfigured(t *testing.T) {
	mock := NewMockServer(t)
	mockK8sStringStack(mock, "61", map[string]interface{}{"KubernetesPrune": false, "KubernetesForceApply": true})

	r := resourcePortainerStack()
	attrs := k8sStringStackAttrs()
	attrs["kubernetes_prune"] = "false"
	attrs["kubernetes_force_apply"] = "true"
	d := dataWithRawConfig(r, attrs, map[string]cty.Value{
		"kubernetes_prune":       cty.False,
		"kubernetes_force_apply": cty.True,
	})
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	body := decodeBody(t, mock, "POST", "/stacks/create/kubernetes/string")
	if got, ok := body["prune"]; !ok || got != false {
		t.Errorf("prune: expected false to be sent, got %v (present %v)", got, ok)
	}
	if got := body["forceApply"]; got != true {
		t.Errorf("forceApply: expected true, got %v", got)
	}
	if d.Get("kubernetes_prune").(bool) || !d.Get("kubernetes_force_apply").(bool) {
		t.Errorf("state: expected prune=false force_apply=true, got %v %v", d.Get("kubernetes_prune"), d.Get("kubernetes_force_apply"))
	}
}

func TestStackCreate_KubernetesOptions_OmittedWhenUnset(t *testing.T) {
	mock := NewMockServer(t)
	// Portainer 2.45 returns no Kubernetes options at all.
	mockK8sStringStack(mock, "61", map[string]interface{}{"prune": false})

	r := resourcePortainerStack()
	d := dataWithRawConfig(r, k8sStringStackAttrs(), map[string]cty.Value{
		"kubernetes_prune": cty.NullVal(cty.Bool),
	})
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	body := decodeBody(t, mock, "POST", "/stacks/create/kubernetes/string")
	for _, key := range []string{"prune", "forceApply"} {
		if _, ok := body[key]; ok {
			t.Errorf("%s must not be sent when unset, got %v", key, body[key])
		}
	}
}

func TestStackCreate_KubernetesOptions_RepositoryAndURL(t *testing.T) {
	for _, tc := range []struct {
		method, path string
		attrs        map[string]string
	}{
		{"repository", "/stacks/create/kubernetes/repository", map[string]string{"repository_url": "https://github.com/acme/k8s.git", "file_path_in_repository": "app.yml"}},
		{"url", "/stacks/create/kubernetes/url", map[string]string{"manifest_url": "https://example.com/app.yml"}},
	} {
		t.Run(tc.method, func(t *testing.T) {
			mock := NewMockServer(t)
			mockEmptyStackList(mock)
			mock.On("POST", tc.path, RespondJSON(http.StatusOK, map[string]interface{}{"Id": 62}))
			mock.On("PUT", "/stacks/62", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 62}))
			mock.On("GET", "/stacks/62", RespondJSON(http.StatusOK, map[string]interface{}{
				"Id": 62, "Name": "k8s", "Status": 1, "Type": 3, "EndpointId": 2,
			}))
			mock.On("GET", "/stacks/62/file", RespondJSON(http.StatusOK, map[string]interface{}{"StackFileContent": "x"}))

			r := resourcePortainerStack()
			attrs := map[string]string{
				"deployment_type": "kubernetes", "method": tc.method, "name": "k8s", "endpoint_id": "2",
				"namespace": "default", "active": "true", "kubernetes_prune": "false",
			}
			for k, v := range tc.attrs {
				attrs[k] = v
			}
			d := dataWithRawConfig(r, attrs, map[string]cty.Value{"kubernetes_prune": cty.False})
			if err := rcCreate(r, d, mock.Client()); err != nil {
				t.Fatalf("Create failed: %v", err)
			}
			body := decodeBody(t, mock, "POST", tc.path)
			if got, ok := body["prune"]; !ok || got != false {
				t.Errorf("prune: expected false to be sent, got %v (present %v)", got, ok)
			}
			if _, ok := body["forceApply"]; ok {
				t.Error("forceApply must not be sent when unset")
			}
			// Not returned (Portainer 2.45): the configured value stays.
			if d.Get("kubernetes_prune").(bool) {
				t.Error("kubernetes_prune: expected the configured false to stay in state")
			}
		})
	}
}

func TestStackCreate_KubernetesOptions_NotSentForDocker(t *testing.T) {
	mock := NewMockServer(t)
	mockEmptyStackList(mock)
	mock.On("POST", "/stacks/create/standalone/string", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 63}))
	mock.On("PUT", "/stacks/63", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 63}))
	mock.On("GET", "/stacks/63", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 63, "Name": "web", "Status": 1, "EndpointId": 1}))
	mock.On("GET", "/stacks/63/file", RespondJSON(http.StatusOK, map[string]interface{}{"StackFileContent": "services: {}"}))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	_ = d.Set("deployment_type", "standalone")
	_ = d.Set("method", "string")
	_ = d.Set("name", "web")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("stack_file_content", "services: {}")
	_ = d.Set("kubernetes_prune", true)
	_ = d.Set("active", true)
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	for _, req := range mock.Requests() {
		if strings.Contains(string(req.Body), "kubernetesPrune") || strings.Contains(string(req.Body), "forceApply") {
			t.Errorf("%s %s: Kubernetes options sent for a Docker stack: %s", req.Method, req.Path, req.Body)
		}
	}
}

func TestStackUpdate_Repository_KubernetesOptions(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/stacks/64/git", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("PUT", "/stacks/64/git/redeploy", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/stacks/64", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 64, "Name": "k8sgit", "Status": 1, "Type": 3, "EndpointId": 2,
		"Option":    map[string]interface{}{"Prune": false, "KubernetesPrune": false, "KubernetesForceApply": false},
		"gitConfig": map[string]interface{}{"URL": "https://github.com/acme/k8s.git", "ReferenceName": "refs/heads/main"},
	}))

	r := resourcePortainerStack()
	d := dataWithRawConfig(r, map[string]string{
		"deployment_type": "kubernetes", "method": "repository", "name": "k8sgit", "endpoint_id": "2",
		"repository_url": "https://github.com/acme/k8s.git", "active": "true",
		"kubernetes_prune": "false", "kubernetes_force_apply": "true",
	}, map[string]cty.Value{"kubernetes_prune": cty.False})
	d.SetId("64")
	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	git := decodeBody(t, mock, "POST", "/stacks/64/git")
	if got, ok := git["kubernetesPrune"]; !ok || got != false {
		t.Errorf("kubernetesPrune: expected false to be sent, got %v (present %v)", got, ok)
	}
	// Set in state but not in the configuration: Portainer keeps its choice.
	if _, ok := git["kubernetesForceApply"]; ok {
		t.Error("kubernetesForceApply must not be sent when the configuration leaves it out")
	}
	redeploy := decodeBody(t, mock, "PUT", "/stacks/64/git/redeploy")
	if _, ok := redeploy["kubernetesPrune"]; ok {
		t.Error("the redeploy payload takes no Kubernetes options")
	}
	if d.Get("kubernetes_force_apply").(bool) {
		t.Error("kubernetes_force_apply: expected the value Portainer returned (false)")
	}
}

func TestStackRead_KubernetesOptions_NotReturnedKeepsState(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/stacks/65", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 65, "Name": "k8sgit", "Status": 1, "Type": 3, "EndpointId": 2,
		"Option": map[string]interface{}{"Prune": false},
	}))
	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("65")
	_ = d.Set("deployment_type", "kubernetes")
	_ = d.Set("method", "repository")
	_ = d.Set("kubernetes_prune", true)
	_ = d.Set("kubernetes_force_apply", true)
	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if !d.Get("kubernetes_prune").(bool) || !d.Get("kubernetes_force_apply").(bool) {
		t.Error("options Portainer does not return must keep their state value")
	}
}

func TestStackDiff_KubernetesOptions(t *testing.T) {
	base := func(deployment, method string, prune bool) map[string]interface{} {
		return map[string]interface{}{
			"deployment_type": deployment, "method": method, "name": "k8s", "endpoint_id": 2,
			"namespace": "default", "stack_file_content": "apiVersion: v1",
			"repository_url": "https://github.com/acme/k8s.git", "kubernetes_prune": prune,
		}
	}
	state := func(method string) *terraform.InstanceState {
		return &terraform.InstanceState{ID: "7", Attributes: map[string]string{
			"id": "7", "deployment_type": "kubernetes", "method": method, "name": "k8s", "endpoint_id": "2",
			"namespace": "default", "stack_file_content": "apiVersion: v1",
			"repository_url": "https://github.com/acme/k8s.git", "kubernetes_prune": "true",
			"repository_reference_name": "refs/heads/main", "active": "true",
		}}
	}
	r := resourcePortainerStack()

	if _, err := r.Diff(context.Background(), nil, terraform.NewResourceConfigRaw(base("standalone", "string", true)), nil); err == nil ||
		!strings.Contains(err.Error(), "kubernetes_prune only applies") {
		t.Errorf("expected kubernetes_prune to be rejected on a Docker stack, got %v", err)
	}

	diff, err := r.Diff(context.Background(), state("string"), terraform.NewResourceConfigRaw(base("kubernetes", "string", false)), nil)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if diff == nil || diff.Attributes["kubernetes_prune"] == nil || !diff.Attributes["kubernetes_prune"].RequiresNew {
		t.Error("a string stack cannot change kubernetes_prune in place, expected a replacement")
	}

	diff, err = r.Diff(context.Background(), state("repository"), terraform.NewResourceConfigRaw(base("kubernetes", "repository", false)), nil)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if diff == nil || diff.Attributes["kubernetes_prune"] == nil || diff.Attributes["kubernetes_prune"].RequiresNew {
		t.Errorf("a repository stack changes kubernetes_prune in place, got %#v", diff)
	}
}
