package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakeGitopsWorkflows is a stateful stand-in for Portainer's GitOps workflow
// API. A workflow created or migrated onto the reconcile engine reports its
// artifacts as gitopsDeployment, numbered from 21; Edge stack artifacts are
// numbered from 11. ignoreEngine plays Portainer 2.45, which drops the engine
// field it does not know.
type fakeGitopsWorkflows struct {
	mu            sync.Mutex
	nextID        int
	ignoreEngine  bool
	deleteMissing bool
	workflows     map[int]*fakeGitopsWorkflow
}

type fakeGitopsWorkflow struct {
	ID        int                       `json:"id"`
	Name      string                    `json:"name"`
	Artifacts []fakeGitopsWorkflowEntry `json:"artifacts"`
}

type fakeGitopsWorkflowEntry struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	Type string `json:"type"`
}

func newFakeGitopsWorkflows(m *MockServer) *fakeGitopsWorkflows {
	f := &fakeGitopsWorkflows{workflows: map[int]*fakeGitopsWorkflow{}}
	m.On("POST", "/gitops/workflows", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name      string `json:"name"`
			Engine    string `json:"engine"`
			Artifacts []struct {
				Name string `json:"name"`
			} `json:"artifacts"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.nextID++
		workflow := &fakeGitopsWorkflow{ID: f.nextID, Name: body.Name}
		reconcile := body.Engine == "reconcile" && !f.ignoreEngine
		for i, a := range body.Artifacts {
			entry := fakeGitopsWorkflowEntry{ID: 11 + i, Name: a.Name, Type: "edgeStack"}
			if reconcile {
				entry = fakeGitopsWorkflowEntry{ID: 21 + i, Name: a.Name, Type: "gitopsDeployment"}
			}
			workflow.Artifacts = append(workflow.Artifacts, entry)
		}
		f.workflows[workflow.ID] = workflow
		RespondJSON(http.StatusOK, map[string]interface{}{"id": workflow.ID})(w, r)
	})
	for id := 1; id <= 3; id++ {
		id := id
		base := fmt.Sprintf("/gitops/workflows/%d", id)
		m.On("GET", base, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			workflow, ok := f.workflows[id]
			if !ok {
				RespondString(http.StatusNotFound, "application/json", `{"message":"workflow not found"}`)(w, r)
				return
			}
			RespondJSON(http.StatusOK, workflow)(w, r)
		})
		m.On("PUT", base, func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Name   string `json:"name"`
				Engine string `json:"engine"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			workflow, ok := f.workflows[id]
			if !ok {
				RespondString(http.StatusNotFound, "application/json", `{"message":"workflow not found"}`)(w, r)
				return
			}
			workflow.Name = body.Name
			if body.Engine == "reconcile" && !f.ignoreEngine {
				for i := range workflow.Artifacts {
					workflow.Artifacts[i].ID = 21 + i
					workflow.Artifacts[i].Type = "gitopsDeployment"
				}
			}
			RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
		})
		for _, mode := range []string{"destroy", "detach"} {
			m.On("DELETE", base+"/"+mode, func(w http.ResponseWriter, r *http.Request) {
				f.mu.Lock()
				defer f.mu.Unlock()
				if _, ok := f.workflows[id]; !ok || f.deleteMissing {
					RespondString(http.StatusNotFound, "application/json", `{"message":"workflow not found"}`)(w, r)
					return
				}
				delete(f.workflows, id)
				w.WriteHeader(http.StatusNoContent)
			})
		}
	}
	return f
}

// gitopsLastPayload decodes the body of the last method+path request.
func gitopsLastPayload(t *testing.T, mock *MockServer, method, path string) map[string]interface{} {
	t.Helper()
	var last *RecordedRequest
	for _, r := range mock.Requests() {
		if r.Method == method && r.Path == path {
			last = r
		}
	}
	if last == nil {
		t.Fatalf("expected %s %s", method, path)
	}
	var payload map[string]interface{}
	if err := last.DecodeJSON(&payload); err != nil {
		t.Fatalf("%s %s: payload is not JSON: %v", method, path, err)
	}
	return payload
}

func gitopsRequestCount(mock *MockServer, method, path string) int {
	n := 0
	for _, r := range mock.Requests() {
		if r.Method == method && r.Path == path {
			n++
		}
	}
	return n
}

// gitopsCheck runs fn as a check function, failing the step on t.Error.
func gitopsCheck(fn func() error) resource.TestCheckFunc {
	return func(*terraform.State) error { return fn() }
}

// gitops245Config is a Portainer 2.45 workflow: none of the 3.0 arguments.
const gitops245Config = `
resource "portainer_gitops_workflow" "w" {
  name = "platform"
  artifact {
    name            = "monitoring"
    type            = "edgeStack"
    deployment_type = "compose"
    edge_group_ids  = [1, 2]
    file {
      source_id = 3
      path      = %q
      ref       = "refs/heads/main"
    }
    config {
      environment    = { LOG_LEVEL = "debug" }
      registry_ids   = [7]
      pre_pull_image = true
      parallel {
        batch_count    = 5
        delay          = "30"
        failure_action = "continue"
      }
    }
  }
}
`

// TestGitopsWorkflowResource_Portainer245Lifecycle is the compatibility
// promise: a configuration that sets none of the 3.0 arguments sends exactly
// what it sent before, an update keys artifacts by identifier and type, and
// destroy goes to the destroy endpoint.
func TestGitopsWorkflowResource_Portainer245Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeGitopsWorkflows(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: gitopsCheck(func() error {
			if mock.FindRequest("DELETE", "/gitops/workflows/1/destroy") == nil {
				return fmt.Errorf("expected DELETE /gitops/workflows/1/destroy")
			}
			return nil
		}),
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(gitops245Config, "portainer.yaml"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_gitops_workflow.w", "id", "1"),
					resource.TestCheckResourceAttr("portainer_gitops_workflow.w", "workflow_id", "1"),
					resource.TestCheckResourceAttr("portainer_gitops_workflow.w", "on_destroy", "destroy"),
					resource.TestCheckResourceAttr("portainer_gitops_workflow.w", "engine", ""),
					resource.TestCheckResourceAttr("portainer_gitops_workflow.w", "artifact.0.artifact_id", "11"),
					gitopsCheck(func() error {
						payload := gitopsLastPayload(t, mock, "POST", "/gitops/workflows")
						for _, key := range []string{"engine", "reconcile"} {
							if _, ok := payload[key]; ok {
								return fmt.Errorf("payload.%s must not be sent when it is not configured: %v", key, payload[key])
							}
						}
						artifact := payload["artifacts"].([]interface{})[0].(map[string]interface{})
						if artifact["name"] != "monitoring" || artifact["deploymentType"] != "compose" {
							return fmt.Errorf("the artifact identity was not sent: %v", artifact)
						}
						if _, ok := artifact["dependencies"]; ok {
							return fmt.Errorf("dependencies must not be sent when not configured")
						}
						targets := artifact["targets"].(map[string]interface{})
						if len(targets) != 1 || len(targets["edgeGroups"].([]interface{})) != 2 {
							return fmt.Errorf("a 2.45 artifact targets Edge groups only: %v", targets)
						}
						files := artifact["files"].([]interface{})
						if len(files) != 1 || files[0].(map[string]interface{})["path"] != "portainer.yaml" {
							return fmt.Errorf("files: %v", files)
						}
						config := artifact["config"].(map[string]interface{})
						if config["prePullImage"] != true || len(config["registries"].([]interface{})) != 1 {
							return fmt.Errorf("config: %v", config)
						}
						if env := config["envVars"].([]interface{}); len(env) != 1 || env[0].(map[string]interface{})["name"] != "LOG_LEVEL" {
							return fmt.Errorf("config.envVars: %v", config["envVars"])
						}
						parallel, ok := config["parallelConfig"].(map[string]interface{})
						if !ok || parallel["batchCount"] != float64(5) || parallel["failureAction"] != "continue" {
							return fmt.Errorf("config.parallelConfig: %v", config["parallelConfig"])
						}
						for _, key := range []string{"helm", "driftMode", "ignorePaths", "timeout", "kubernetesPrune", "kubernetesForceApply"} {
							if _, ok := config[key]; ok {
								return fmt.Errorf("config.%s must not be sent when not configured: %v", key, config[key])
							}
						}
						return nil
					}),
				),
			},
			{
				Config:   mockProviderConfig(mock) + fmt.Sprintf(gitops245Config, "portainer.yaml"),
				PlanOnly: true,
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(gitops245Config, "stack.yaml"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_gitops_workflow.w", plancheck.ResourceActionUpdate)},
				},
				Check: gitopsCheck(func() error {
					payload := gitopsLastPayload(t, mock, "PUT", "/gitops/workflows/1")
					if _, ok := payload["engine"]; ok {
						return fmt.Errorf("engine must not be sent: %v", payload["engine"])
					}
					artifact := payload["artifacts"].([]interface{})[0].(map[string]interface{})
					if artifact["id"] != float64(11) || artifact["type"] != "edgeStack" {
						return fmt.Errorf("an update must identify the artifact by id and type: %v", artifact)
					}
					if _, ok := artifact["name"]; ok {
						return fmt.Errorf("an update payload must not carry the artifact name: %v", artifact["name"])
					}
					if files := artifact["files"].([]interface{}); files[0].(map[string]interface{})["path"] != "stack.yaml" {
						return fmt.Errorf("files: %v", files)
					}
					return nil
				}),
			},
			{
				ResourceName:  "portainer_gitops_workflow.w",
				ImportState:   true,
				ImportStateId: "1",
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 || states[0].Attributes["workflow_id"] != "1" || states[0].Attributes["name"] != "platform" {
						return fmt.Errorf("imported state: %v", states)
					}
					return nil
				},
			},
		},
	})
}

// TestGitopsWorkflowResource_ConfigOmittedWhenEmpty keeps an artifact from
// carrying a config object full of zero values Portainer would read as
// settings, and detach selects its own endpoint.
func TestGitopsWorkflowResource_ConfigOmittedWhenEmpty(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeGitopsWorkflows(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: gitopsCheck(func() error {
			if mock.FindRequest("DELETE", "/gitops/workflows/1/detach") == nil {
				return fmt.Errorf("on_destroy = detach must call the detach endpoint")
			}
			if mock.FindRequest("DELETE", "/gitops/workflows/1/destroy") != nil {
				return fmt.Errorf("on_destroy = detach must not destroy the artifacts")
			}
			return nil
		}),
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_gitops_workflow" "w" {
  name       = "platform"
  on_destroy = "detach"
  artifact {
    name            = "monitoring"
    deployment_type = "compose"
    edge_group_ids  = [1]
    file {
      source_id = 3
      path      = "portainer.yaml"
      ref       = "refs/heads/main"
    }
  }
}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_gitops_workflow.w", "artifact.0.type", "edgeStack"),
				gitopsCheck(func() error {
					artifact := gitopsLastPayload(t, mock, "POST", "/gitops/workflows")["artifacts"].([]interface{})[0].(map[string]interface{})
					if _, ok := artifact["config"]; ok {
						return fmt.Errorf("an artifact with no configured options must not carry a config object: %v", artifact["config"])
					}
					return nil
				}),
			),
		}},
	})
}

// gitopsReconcileConfig is a workflow on the reconcile engine that sets every
// Portainer 3.0 argument; driftMode varies between steps.
const gitopsReconcileConfig = `
resource "portainer_gitops_workflow" "w" {
  name               = "platform"
  engine             = "reconcile"
  reconcile_interval = "10m"
  artifact {
    name                  = "web"
    deployment_type       = "kubernetes"
    environment_ids       = [5]
    environment_group_ids = [2]
    dependencies          = ["crds"]
    file {
      source_id = 3
      path      = "charts/web"
      ref       = "refs/heads/main"
    }
    config {
      drift_mode             = %q
      ignore_paths           = ["/spec/replicas"]
      timeout                = "5m"
      kubernetes_prune       = false
      kubernetes_force_apply = true
    }
    helm {
      chart_path   = "charts/web"
      release_name = "web"
      namespace    = "web"
      values_files = ["values/prod.yaml"]
      install {
        retries                = 2
        remediate_last_failure = false
        strategy               = "uninstall"
      }
    }
  }
}
`

// TestGitopsWorkflowResource_ReconcileEngine pins the 3.0 payloads: the
// engine and its interval at the workflow level, the alternative targets, a
// chart carried in the config on create and beside it on update, configured
// false switches sent - and engine not sent again once reconciled.
func TestGitopsWorkflowResource_ReconcileEngine(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeGitopsWorkflows(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(gitopsReconcileConfig, "enabled"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_gitops_workflow.w", "engine", "reconcile"),
					resource.TestCheckResourceAttr("portainer_gitops_workflow.w", "artifact.0.artifact_id", "21"),
					gitopsCheck(func() error {
						payload := gitopsLastPayload(t, mock, "POST", "/gitops/workflows")
						if payload["engine"] != "reconcile" {
							return fmt.Errorf("payload.engine: got %v", payload["engine"])
						}
						if reconcile, _ := payload["reconcile"].(map[string]interface{}); reconcile["interval"] != "10m" {
							return fmt.Errorf("payload.reconcile.interval: got %v", payload["reconcile"])
						}
						artifact := payload["artifacts"].([]interface{})[0].(map[string]interface{})
						targets := artifact["targets"].(map[string]interface{})
						if _, ok := targets["edgeGroups"]; ok {
							return fmt.Errorf("unconfigured Edge groups must not be sent beside other targets: %v", targets)
						}
						if len(targets["environments"].([]interface{})) != 1 || len(targets["environmentGroups"].([]interface{})) != 1 {
							return fmt.Errorf("targets: %v", targets)
						}
						if deps := artifact["dependencies"].([]interface{}); len(deps) != 1 || deps[0] != "crds" {
							return fmt.Errorf("dependencies: %v", artifact["dependencies"])
						}
						if _, ok := artifact["helm"]; ok {
							return fmt.Errorf("a create carries the chart in the config, not beside it")
						}
						config := artifact["config"].(map[string]interface{})
						if config["driftMode"] != "enabled" || config["timeout"] != "5m" {
							return fmt.Errorf("config: %v", config)
						}
						if paths := config["ignorePaths"].([]interface{}); len(paths) != 1 || paths[0] != "/spec/replicas" {
							return fmt.Errorf("config.ignorePaths: %v", config["ignorePaths"])
						}
						if got, ok := config["kubernetesPrune"]; !ok || got != false {
							return fmt.Errorf("config.kubernetesPrune: a configured false must be sent, got %v (present=%v)", got, ok)
						}
						if config["kubernetesForceApply"] != true {
							return fmt.Errorf("config.kubernetesForceApply: got %v", config["kubernetesForceApply"])
						}
						helm := config["helm"].(map[string]interface{})
						if helm["chartPath"] != "charts/web" || helm["releaseName"] != "web" || helm["namespace"] != "web" {
							return fmt.Errorf("config.helm: %v", helm)
						}
						install := helm["install"].(map[string]interface{})
						if install["retries"] != float64(2) || install["strategy"] != "uninstall" {
							return fmt.Errorf("config.helm.install: %v", install)
						}
						if got, ok := install["remediateLastFailure"]; !ok || got != false {
							return fmt.Errorf("config.helm.install.remediateLastFailure: a configured false must be sent, got %v (present=%v)", got, ok)
						}
						if _, ok := helm["upgrade"]; ok {
							return fmt.Errorf("an unconfigured upgrade remediation must not be sent")
						}
						return nil
					}),
				),
			},
			{
				Config:   mockProviderConfig(mock) + fmt.Sprintf(gitopsReconcileConfig, "enabled"),
				PlanOnly: true,
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(gitopsReconcileConfig, "warn"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_gitops_workflow.w", plancheck.ResourceActionUpdate)},
				},
				Check: gitopsCheck(func() error {
					payload := gitopsLastPayload(t, mock, "PUT", "/gitops/workflows/1")
					if _, ok := payload["engine"]; ok {
						return fmt.Errorf("engine is a migration and must not be sent when it did not change: %v", payload["engine"])
					}
					if reconcile, _ := payload["reconcile"].(map[string]interface{}); reconcile["interval"] != "10m" {
						return fmt.Errorf("payload.reconcile: a configured interval is always sent, got %v", payload["reconcile"])
					}
					artifact := payload["artifacts"].([]interface{})[0].(map[string]interface{})
					if artifact["id"] != float64(21) || artifact["type"] != "gitopsDeployment" {
						return fmt.Errorf("a reconciled artifact is updated as a gitopsDeployment: %v", artifact)
					}
					if helm, ok := artifact["helm"].(map[string]interface{}); !ok || helm["chartPath"] != "charts/web" {
						return fmt.Errorf("an update carries the chart beside the config: %v", artifact["helm"])
					}
					config := artifact["config"].(map[string]interface{})
					if config["helm"] != nil {
						return fmt.Errorf("an update must not carry the chart in the config")
					}
					if got, ok := config["kubernetesPrune"]; !ok || got != false {
						return fmt.Errorf("config.kubernetesPrune: got %v (present=%v)", got, ok)
					}
					if config["driftMode"] != "warn" {
						return fmt.Errorf("config.driftMode: got %v", config["driftMode"])
					}
					return nil
				}),
			},
		},
	})
}

// TestGitopsWorkflowResource_ReconcileIgnoredByOlderServer: Portainer 2.45
// drops the engine field it does not know and deploys Edge stacks. The apply
// has to say so rather than record a reconcile workflow that is not one.
func TestGitopsWorkflowResource_ReconcileIgnoredByOlderServer(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeGitopsWorkflows(mock)
	fake.ignoreEngine = true

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + fmt.Sprintf(gitopsReconcileConfig, "enabled"),
			ExpectError: regexp.MustCompile(`requires Portainer Business Edition 3.0 or later`),
		}},
	})
}

// gitopsEngineConfig is a one-artifact workflow; settings go into the
// workflow, artifactSettings into the artifact.
func gitopsEngineConfig(mock *MockServer, settings, artifactSettings string) string {
	return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_gitops_workflow" "w" {
  name = "platform"
  %s
  artifact {
    name            = "web"
    deployment_type = "kubernetes"
    edge_group_ids  = [1]
    file {
      source_id = 3
      path      = "k8s"
      ref       = "refs/heads/main"
    }
    %s
  }
}`, settings, artifactSettings)
}

// TestGitopsWorkflowResource_MigrationIsInPlace: Portainer migrates a
// workflow onto the reconcile engine through the update call, matching the
// artifacts by their current Edge stack identifier - so the change is no
// replacement, and the artifacts are still named as Edge stacks in that one
// call. Leaving the engine again replaces the workflow.
func TestGitopsWorkflowResource_MigrationIsInPlace(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeGitopsWorkflows(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: gitopsEngineConfig(mock, `engine = "edgeStack"`, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_gitops_workflow.w", "engine", "edgeStack"),
					resource.TestCheckResourceAttr("portainer_gitops_workflow.w", "artifact.0.artifact_id", "11"),
				),
			},
			{
				Config: gitopsEngineConfig(mock, `
  engine             = "reconcile"
  reconcile_interval = "5m"`, `helm { chart_path = "charts/web" }`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_gitops_workflow.w", plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_gitops_workflow.w", "id", "1"),
					resource.TestCheckResourceAttr("portainer_gitops_workflow.w", "engine", "reconcile"),
					resource.TestCheckResourceAttr("portainer_gitops_workflow.w", "artifact.0.artifact_id", "21"),
					gitopsCheck(func() error {
						payload := gitopsLastPayload(t, mock, "PUT", "/gitops/workflows/1")
						if payload["engine"] != "reconcile" {
							return fmt.Errorf("payload.engine: a migration must send it, got %v", payload["engine"])
						}
						if reconcile, _ := payload["reconcile"].(map[string]interface{}); reconcile["interval"] != "5m" {
							return fmt.Errorf("payload.reconcile: got %v", payload["reconcile"])
						}
						artifact := payload["artifacts"].([]interface{})[0].(map[string]interface{})
						if artifact["id"] != float64(11) || artifact["type"] != "edgeStack" {
							return fmt.Errorf("a migrating artifact is matched by its Edge stack: %v", artifact)
						}
						if helm, ok := artifact["helm"].(map[string]interface{}); !ok || helm["chartPath"] != "charts/web" {
							return fmt.Errorf("an update carries the chart beside the config: %v", artifact["helm"])
						}
						return nil
					}),
				),
			},
			{
				// Removing the engine from the configuration keeps the
				// recorded one: it is optional and computed.
				Config:   gitopsEngineConfig(mock, `reconcile_interval = "5m"`, `helm { chart_path = "charts/web" }`),
				PlanOnly: true,
			},
			{
				Config: gitopsEngineConfig(mock, `engine = "edgeStack"`, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_gitops_workflow.w", plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: resource.TestCheckResourceAttr("portainer_gitops_workflow.w", "id", "2"),
			},
		},
	})
}

// TestGitopsWorkflowResource_ArtifactRenameForcesNew guards a silent
// divergence: Portainer's update payload has no name field, so a renamed
// artifact would be accepted into state and never sent.
func TestGitopsWorkflowResource_ArtifactRenameForcesNew(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeGitopsWorkflows(mock)

	config := func(name string) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_gitops_workflow" "w" {
  name = "platform"
  artifact {
    name            = %q
    deployment_type = "compose"
    edge_group_ids  = [1]
    file {
      source_id = 3
      path      = "portainer.yaml"
      ref       = "refs/heads/main"
    }
  }
}`, name)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config("monitoring")},
			{
				Config: config("observability"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_gitops_workflow.w", plancheck.ResourceActionDestroyBeforeCreate)},
				},
			},
		},
	})
}

// TestGitopsWorkflowResource_PlanRejectsInvalidCombinations turns what
// Portainer rejects at apply time, or would silently ignore, into plan errors.
func TestGitopsWorkflowResource_PlanRejectsInvalidCombinations(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	for name, tc := range map[string]struct {
		settings, artifact, want string
	}{
		"interval without reconcile": {`
  engine             = "edgeStack"
  reconcile_interval = "5m"`, `edge_group_ids = [1]`, "only applies"},
		"interval with unset engine": {`reconcile_interval = "5m"`, `edge_group_ids = [1]`, "only applies"},
		"reconcile without interval": {`engine = "reconcile"`, `environment_ids = [5]`, "is required"},
		"artifact without target":    {"", "", "needs a target"},
		"invalid interval":           {`reconcile_interval = "soon"`, `edge_group_ids = [1]`, "must be a duration"},
	} {
		t.Run(name, func(t *testing.T) {
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{{
					Config: mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_gitops_workflow" "w" {
  name = "platform"
  %s
  artifact {
    name            = "web"
    deployment_type = "kubernetes"
    %s
    file {
      source_id = 3
      path      = "k8s"
      ref       = "refs/heads/main"
    }
  }
}`, tc.settings, tc.artifact),
					PlanOnly:    true,
					ExpectError: regexp.MustCompile(tc.want),
				}},
			})
		})
	}
}

// TestGitopsWorkflowResource_ImportDerivesReconcileEngine: Portainer reports
// no engine, so an imported reconcile workflow is recognised by its artifact
// types - otherwise a configuration saying `reconcile` would plan a second
// migration.
func TestGitopsWorkflowResource_ImportDerivesReconcileEngine(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeGitopsWorkflows(mock)
	fake.workflows[1] = &fakeGitopsWorkflow{ID: 1, Name: "platform", Artifacts: []fakeGitopsWorkflowEntry{{ID: 21, Name: "web", Type: "gitopsDeployment"}}}

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:        mockProviderConfig(mock) + fmt.Sprintf(gitopsReconcileConfig, "enabled"),
			ResourceName:  "portainer_gitops_workflow.w",
			ImportState:   true,
			ImportStateId: "1",
			ImportStateCheck: func(states []*terraform.InstanceState) error {
				if got := states[0].Attributes["engine"]; got != "reconcile" {
					return fmt.Errorf("engine: got %q", got)
				}
				return nil
			},
		}},
	})
}

// TestGitopsWorkflowResource_GoneOutsideTerraform: a workflow deleted in
// Portainer leaves state, and destroying one already gone does not fail.
func TestGitopsWorkflowResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeGitopsWorkflows(mock)
	config := gitopsEngineConfig(mock, "", "")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					delete(fake.workflows, 1)
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				// Created again; its delete then answers 404, which the
				// destroy at the end of the test takes as success.
				Config: config,
				Check: gitopsCheck(func() error {
					fake.mu.Lock()
					fake.deleteMissing = true
					fake.mu.Unlock()
					return nil
				}),
			},
		},
	})
}

// TestGitopsWorkflow_UpgradeFromV300: state written by v3.0.0 (SDKv2) against
// the same fake plans no change with this build, for configurations that set
// and that omit the optional arguments.
func TestGitopsWorkflow_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for name, config := range map[string]func(*MockServer) string{
		"minimal": func(m *MockServer) string { return gitopsEngineConfig(m, "", "") },
		"2.45 with options": func(m *MockServer) string {
			return mockProviderConfig(m) + fmt.Sprintf(gitops245Config, "portainer.yaml")
		},
		"reconcile with everything": func(m *MockServer) string {
			return mockProviderConfig(m) + strings.ReplaceAll(fmt.Sprintf(gitopsReconcileConfig, "enabled"), "= false", "= true")
		},
		"detach and engine": func(m *MockServer) string {
			return gitopsEngineConfig(m, `
  engine     = "edgeStack"
  on_destroy = "detach"`, `
    type = "stack"
    config {
      retry_period          = 30
      local_filesystem_path = "/data"
    }`)
		},
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			newFakeGitopsWorkflows(mock)
			resource.Test(t, resource.TestCase{
				IsUnitTest: true,
				Steps: []resource.TestStep{
					{ExternalProviders: previousRelease, Config: config(mock)},
					{
						ProtoV5ProviderFactories: testProtoV5Factories,
						Config:                   config(mock),
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
					},
				},
			})
		})
	}
}

// TestGitopsWorkflow_UpgradeFromV300ExplicitFalse documents the one case the
// upgrade cannot tell apart: SDKv2 stored false and 0 for an artifact
// argument whether it was configured that way or left out, and the upgrade
// reads them as left out, the common case. A configuration that spells out
// false therefore plans one update, which sends what Portainer already has,
// and is stable from then on.
func TestGitopsWorkflow_UpgradeFromV300ExplicitFalse(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeGitopsWorkflows(mock)
	config := mockProviderConfig(mock) + fmt.Sprintf(gitopsReconcileConfig, "enabled")
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_gitops_workflow.w", plancheck.ResourceActionUpdate)},
				},
				Check: gitopsCheck(func() error {
					config := gitopsLastPayload(t, mock, "PUT", "/gitops/workflows/1")["artifacts"].([]interface{})[0].(map[string]interface{})["config"].(map[string]interface{})
					if got, ok := config["kubernetesPrune"]; !ok || got != false {
						return fmt.Errorf("config.kubernetesPrune: got %v (present=%v)", got, ok)
					}
					return nil
				}),
			},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestAccGitopsWorkflowResource needs a Business Edition instance with a
// GitOps source and an Edge group, so it only runs when they are named.
func TestAccGitopsWorkflowResource(t *testing.T) {
	if os.Getenv("PORTAINER_ACC_BE") != "1" || os.Getenv("PORTAINER_ACC_GITOPS_SOURCE_ID") == "" || os.Getenv("PORTAINER_ACC_EDGE_GROUP_ID") == "" {
		t.Skip("set PORTAINER_ACC_BE=1, PORTAINER_ACC_GITOPS_SOURCE_ID and PORTAINER_ACC_EDGE_GROUP_ID to run against a Business Edition instance with a GitOps source and an Edge group")
	}
	config := accProvider() + fmt.Sprintf(`
resource "portainer_gitops_workflow" "w" {
  name = "acc-gitops-workflow"
  artifact {
    name            = "acc-artifact"
    deployment_type = "compose"
    edge_group_ids  = [%s]
    file {
      source_id = %s
      path      = "docker-compose.yml"
      ref       = "refs/heads/main"
    }
  }
}`, os.Getenv("PORTAINER_ACC_EDGE_GROUP_ID"), os.Getenv("PORTAINER_ACC_GITOPS_SOURCE_ID"))
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config, Check: resource.TestCheckResourceAttrSet("portainer_gitops_workflow.w", "workflow_id")},
			{Config: config, PlanOnly: true},
		},
	})
}
