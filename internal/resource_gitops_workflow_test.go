package internal

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// =========================================================================
// Portainer 3.0 additions to portainer_gitops_workflow: the reconcile engine,
// its targets and options, Helm artifacts and dependencies - and the promise
// that none of it changes what a Portainer 2.45 workflow sends or plans.
// =========================================================================

// gitopsReconcileArtifact is an artifact on the reconcile engine that sets
// every Portainer 3.0 field.
func gitopsReconcileArtifact() map[string]interface{} {
	return map[string]interface{}{
		"name":                  "web",
		"deployment_type":       "kubernetes",
		"environment_ids":       []interface{}{5},
		"environment_group_ids": []interface{}{2},
		"dependencies":          []interface{}{"crds"},
		"file": []interface{}{map[string]interface{}{
			"source_id": 3, "path": "charts/web", "ref": "refs/heads/main",
		}},
		"config": []interface{}{map[string]interface{}{
			"drift_mode":             "enabled",
			"ignore_paths":           []interface{}{"/spec/replicas"},
			"timeout":                "5m",
			"kubernetes_prune":       false,
			"kubernetes_force_apply": true,
		}},
		"helm": []interface{}{map[string]interface{}{
			"chart_path":   "charts/web",
			"release_name": "web",
			"namespace":    "web",
			"values_files": []interface{}{"values/prod.yaml"},
			"install": []interface{}{map[string]interface{}{
				"retries": 2, "remediate_last_failure": false, "strategy": "uninstall",
			}},
		}},
	}
}

// gitopsReconcileRawConfig is the part of the configuration the payload
// consults to tell a configured false from an unset switch.
func gitopsReconcileRawConfig() cty.Value {
	return cty.ObjectVal(map[string]cty.Value{
		"artifact": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
			"config": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
				"kubernetes_prune":       cty.False,
				"kubernetes_force_apply": cty.True,
			})}),
			"helm": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
				"install": cty.ListVal([]cty.Value{cty.ObjectVal(map[string]cty.Value{
					"remediate_last_failure": cty.False,
				})}),
			})}),
		})}),
	})
}

// gitopsReconcileData is a reconcile workflow whose configuration is in the
// raw config too. engineInState is the engine the prior state records.
func gitopsReconcileData(t *testing.T, id, engineInState string) (*schema.Resource, *schema.ResourceData) {
	t.Helper()
	r := resourceGitopsWorkflow()
	seed := r.TestResourceData()
	// State() is nil without an ID; a create clears it again below.
	seed.SetId("4")
	_ = seed.Set("name", "platform")
	_ = seed.Set("engine", engineInState)
	_ = seed.Set("reconcile_interval", "10m")
	artifact := gitopsReconcileArtifact()
	if id != "" {
		artifact["artifact_id"] = 21
	}
	_ = seed.Set("artifact", []interface{}{artifact})
	state := seed.State()
	state.RawConfig = gitopsReconcileRawConfig()
	d := r.Data(state)
	d.SetId(id)
	return r, d
}

// gitopsEngineChangeDiff plans a one-artifact workflow whose state records
// engineInState against a configuration adding settings to that artifact's
// workflow, so only the engine fields differ.
func gitopsEngineChangeDiff(t *testing.T, engineInState string, settings map[string]interface{}) (*terraform.InstanceDiff, error) {
	t.Helper()
	_, _, diff, err := gitopsEngineChange(t, engineInState, settings, nil)
	return diff, err
}

// gitopsEngineChange is gitopsEngineChangeDiff with the state it planned
// from; artifactSettings are added to the configured artifact only.
func gitopsEngineChange(t *testing.T, engineInState string, settings, artifactSettings map[string]interface{}) (*schema.Resource, *terraform.InstanceState, *terraform.InstanceDiff, error) {
	t.Helper()
	artifact := map[string]interface{}{
		"name": "web", "type": "edgeStack", "deployment_type": "kubernetes", "edge_group_ids": []interface{}{1},
		"file": []interface{}{map[string]interface{}{"source_id": 3, "path": "k8s", "ref": "refs/heads/main"}},
	}
	r := resourceGitopsWorkflow()
	seed := r.TestResourceData()
	seed.SetId("4")
	_ = seed.Set("name", "platform")
	_ = seed.Set("engine", engineInState)
	inState := map[string]interface{}{"artifact_id": 11}
	for k, v := range artifact {
		inState[k] = v
	}
	_ = seed.Set("artifact", []interface{}{inState})
	state := seed.State()

	configured := map[string]interface{}{}
	for k, v := range artifact {
		configured[k] = v
	}
	for k, v := range artifactSettings {
		configured[k] = v
	}
	config := map[string]interface{}{"name": "platform", "artifact": []interface{}{configured}}
	for k, v := range settings {
		config[k] = v
	}
	diff, err := r.Diff(context.Background(), state, terraform.NewResourceConfigRaw(config), nil)
	return r, state, diff, err
}

func decodeWorkflowPayload(t *testing.T, mock *MockServer, method, path string) map[string]interface{} {
	t.Helper()
	req := mock.FindRequest(method, path)
	if req == nil {
		t.Fatalf("expected %s %s", method, path)
	}
	var payload map[string]interface{}
	if err := req.DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	return payload
}

// TestGitopsWorkflow_Portainer245PayloadUnchanged is the compatibility
// promise: a configuration that sets none of the 3.0 fields sends exactly what
// it sent before, so Portainer 2.45 sees no field it does not know.
func TestGitopsWorkflow_Portainer245PayloadUnchanged(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/workflows", RespondJSON(http.StatusOK, map[string]interface{}{"id": 4}))
	mock.On("GET", "/gitops/workflows/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 4, "name": "platform",
		"artifacts": []map[string]interface{}{{"id": 11, "name": "monitoring", "type": "edgeStack"}},
	}))

	r, d := gitopsWorkflowData(t)
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	payload := decodeWorkflowPayload(t, mock, "POST", "/gitops/workflows")
	for _, key := range []string{"engine", "reconcile"} {
		if _, ok := payload[key]; ok {
			t.Errorf("payload.%s must not be sent when it is not configured: %v", key, payload[key])
		}
	}
	artifact := payload["artifacts"].([]interface{})[0].(map[string]interface{})
	if _, ok := artifact["dependencies"]; ok {
		t.Errorf("dependencies must not be sent when not configured: %v", artifact["dependencies"])
	}
	targets := artifact["targets"].(map[string]interface{})
	if len(targets) != 1 || targets["edgeGroups"] == nil {
		t.Errorf("a 2.45 artifact targets Edge groups only: %v", targets)
	}
	config := artifact["config"].(map[string]interface{})
	for _, key := range []string{"helm", "driftMode", "ignorePaths", "timeout", "kubernetesPrune", "kubernetesForceApply"} {
		if _, ok := config[key]; ok {
			t.Errorf("config.%s must not be sent when not configured: %v", key, config[key])
		}
	}

	// Edge stack artifacts must not put an engine into state the
	// configuration does not have, or every plan would show it.
	if got := d.Get("engine"); got != "" {
		t.Errorf("engine: expected it left unset for an unset configuration, got %v", got)
	}
}

// TestGitopsWorkflow_Portainer245StateHasNoDiff: a workflow created before the
// 3.0 attributes existed plans no change once the provider knows them.
func TestGitopsWorkflow_Portainer245StateHasNoDiff(t *testing.T) {
	r := resourceGitopsWorkflow()
	artifact := map[string]interface{}{
		"name": "monitoring", "type": "edgeStack", "deployment_type": "compose",
		"edge_group_ids": []interface{}{1},
		"file": []interface{}{map[string]interface{}{
			"source_id": 3, "path": "portainer.yaml", "ref": "refs/heads/main",
		}},
	}
	state := &terraform.InstanceState{ID: "4", Attributes: map[string]string{
		"id": "4", "name": "platform", "on_destroy": "destroy", "workflow_id": "4",
		"artifact.#": "1", "artifact.0.name": "monitoring", "artifact.0.type": "edgeStack",
		"artifact.0.deployment_type": "compose", "artifact.0.artifact_id": "11",
		"artifact.0.edge_group_ids.#": "1", "artifact.0.edge_group_ids.0": "1",
		"artifact.0.file.#": "1", "artifact.0.file.0.source_id": "3",
		"artifact.0.file.0.path": "portainer.yaml", "artifact.0.file.0.ref": "refs/heads/main",
		"artifact.0.config.#": "0",
	}}
	config := terraform.NewResourceConfigRaw(map[string]interface{}{
		"name": "platform", "artifact": []interface{}{artifact},
	})

	diff, err := r.Diff(context.Background(), state, config, nil)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if diff != nil && len(diff.Attributes) > 0 {
		t.Errorf("a 2.45 workflow must plan no change, got %v", diff.Attributes)
	}
}

// TestGitopsWorkflow_CreateOnReconcileEngine pins the 3.0 create payload: the
// engine and its interval at the workflow level, the alternative targets, and
// a chart carried in the config.
func TestGitopsWorkflow_CreateOnReconcileEngine(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/workflows", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 4, "artifacts": []map[string]interface{}{{"deploymentId": 21}},
	}))
	mock.On("GET", "/gitops/workflows/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 4, "name": "platform",
		"artifacts": []map[string]interface{}{{"id": 21, "name": "web", "type": "gitopsDeployment"}},
	}))

	r, d := gitopsReconcileData(t, "", "reconcile")
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	payload := decodeWorkflowPayload(t, mock, "POST", "/gitops/workflows")
	if payload["engine"] != "reconcile" {
		t.Errorf("payload.engine: got %v", payload["engine"])
	}
	if reconcile, _ := payload["reconcile"].(map[string]interface{}); reconcile["interval"] != "10m" {
		t.Errorf("payload.reconcile.interval: got %v", payload["reconcile"])
	}

	artifact := payload["artifacts"].([]interface{})[0].(map[string]interface{})
	targets := artifact["targets"].(map[string]interface{})
	if _, ok := targets["edgeGroups"]; ok {
		t.Errorf("unconfigured Edge groups must not be sent beside other targets: %v", targets)
	}
	if len(targets["environments"].([]interface{})) != 1 || len(targets["environmentGroups"].([]interface{})) != 1 {
		t.Errorf("targets: %v", targets)
	}
	if deps := artifact["dependencies"].([]interface{}); len(deps) != 1 || deps[0] != "crds" {
		t.Errorf("dependencies: %v", artifact["dependencies"])
	}
	if _, ok := artifact["helm"]; ok {
		t.Error("a create carries the chart in the config, not beside it")
	}

	config := artifact["config"].(map[string]interface{})
	if config["driftMode"] != "enabled" || config["timeout"] != "5m" {
		t.Errorf("config: %v", config)
	}
	if paths := config["ignorePaths"].([]interface{}); len(paths) != 1 || paths[0] != "/spec/replicas" {
		t.Errorf("config.ignorePaths: %v", config["ignorePaths"])
	}
	if got, ok := config["kubernetesPrune"]; !ok || got != false {
		t.Errorf("config.kubernetesPrune: a configured false must be sent, got %v (present=%v)", got, ok)
	}
	if config["kubernetesForceApply"] != true {
		t.Errorf("config.kubernetesForceApply: got %v", config["kubernetesForceApply"])
	}
	helm := config["helm"].(map[string]interface{})
	if helm["chartPath"] != "charts/web" || helm["releaseName"] != "web" || helm["namespace"] != "web" {
		t.Errorf("config.helm: %v", helm)
	}
	install := helm["install"].(map[string]interface{})
	if install["retries"] != float64(2) || install["strategy"] != "uninstall" {
		t.Errorf("config.helm.install: %v", install)
	}
	if got, ok := install["remediateLastFailure"]; !ok || got != false {
		t.Errorf("config.helm.install.remediateLastFailure: a configured false must be sent, got %v (present=%v)", got, ok)
	}
	if _, ok := helm["upgrade"]; ok {
		t.Error("an unconfigured upgrade remediation must not be sent")
	}

	if got := d.Get("engine"); got != "reconcile" {
		t.Errorf("engine: got %v", got)
	}
	if got := d.Get("artifact").([]interface{})[0].(map[string]interface{})["artifact_id"]; got != 21 {
		t.Errorf("artifact_id: expected the deployment identifier, got %v", got)
	}
}

// TestGitopsWorkflow_ReconcileIgnoredByOlderServer: Portainer 2.45 drops the
// engine field it does not know and deploys Edge stacks. The apply has to say
// so rather than record a reconcile workflow that is not one.
func TestGitopsWorkflow_ReconcileIgnoredByOlderServer(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/workflows", RespondJSON(http.StatusOK, map[string]interface{}{"id": 4}))
	mock.On("GET", "/gitops/workflows/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 4, "name": "platform",
		"artifacts": []map[string]interface{}{{"id": 11, "name": "web", "type": "edgeStack"}},
	}))

	r, d := gitopsReconcileData(t, "", "reconcile")
	err := rcCreate(r, d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "requires Portainer Business Edition 3.0 or later") {
		t.Fatalf("expected the engine requirement to be named, got %v", err)
	}
	if got := d.Get("engine"); got != "edgeStack" {
		t.Errorf("engine: state must record what the server runs, got %v", got)
	}
}

// TestGitopsWorkflow_MigrationIsInPlace: Portainer migrates a workflow onto the
// reconcile engine through the update call, matching the artifacts by their
// current Edge stack identifier - so the change is no replacement, and the
// artifacts are still named as Edge stacks in that one call.
func TestGitopsWorkflow_MigrationIsInPlace(t *testing.T) {
	r, state, diff, err := gitopsEngineChange(t, "edgeStack",
		map[string]interface{}{"engine": "reconcile", "reconcile_interval": "5m"},
		map[string]interface{}{"helm": []interface{}{map[string]interface{}{"chart_path": "charts/web"}}})
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if diff == nil || diff.RequiresNew() {
		t.Fatal("migrating onto the reconcile engine must be an in-place update")
	}

	mock := NewMockServer(t)
	mock.On("PUT", "/gitops/workflows/4", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/gitops/workflows/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 4, "name": "platform",
		"artifacts": []map[string]interface{}{{"id": 21, "name": "web", "type": "gitopsDeployment"}},
	}))

	d, err := schema.InternalMap(r.Schema).Data(state, diff)
	if err != nil {
		t.Fatalf("failed to build the planned ResourceData: %v", err)
	}
	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	payload := decodeWorkflowPayload(t, mock, "PUT", "/gitops/workflows/4")
	if payload["engine"] != "reconcile" {
		t.Errorf("payload.engine: a migration must send it, got %v", payload["engine"])
	}
	if reconcile, _ := payload["reconcile"].(map[string]interface{}); reconcile["interval"] != "5m" {
		t.Errorf("payload.reconcile: got %v", payload["reconcile"])
	}
	artifact := payload["artifacts"].([]interface{})[0].(map[string]interface{})
	if artifact["id"] != float64(11) || artifact["type"] != "edgeStack" {
		t.Errorf("a migrating artifact is matched by its Edge stack: %v", artifact)
	}
	if helm, ok := artifact["helm"].(map[string]interface{}); !ok || helm["chartPath"] != "charts/web" {
		t.Errorf("an update carries the chart beside the config: %v", artifact["helm"])
	}
	if config, ok := artifact["config"].(map[string]interface{}); ok && config["helm"] != nil {
		t.Error("an update must not carry the chart in the config")
	}
	if got := d.Get("artifact").([]interface{})[0].(map[string]interface{})["artifact_id"]; got != 21 {
		t.Errorf("artifact_id: expected the new deployment identifier, got %v", got)
	}
}

// TestGitopsWorkflow_UpdateOnReconcileEngine: once migrated, artifacts are
// named as reconciled deployments and engine is not sent again.
func TestGitopsWorkflow_UpdateOnReconcileEngine(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/gitops/workflows/4", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/gitops/workflows/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 4, "name": "platform",
		"artifacts": []map[string]interface{}{{"id": 21, "name": "web", "type": "gitopsDeployment"}},
	}))

	r, d := gitopsReconcileData(t, "4", "reconcile")
	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	payload := decodeWorkflowPayload(t, mock, "PUT", "/gitops/workflows/4")
	if _, ok := payload["engine"]; ok {
		t.Errorf("engine is a migration and must not be sent when it did not change: %v", payload["engine"])
	}
	artifact := payload["artifacts"].([]interface{})[0].(map[string]interface{})
	if artifact["id"] != float64(21) || artifact["type"] != "gitopsDeployment" {
		t.Errorf("a reconciled artifact is updated as a gitopsDeployment: %v", artifact)
	}
	config := artifact["config"].(map[string]interface{})
	if got, ok := config["kubernetesPrune"]; !ok || got != false {
		t.Errorf("config.kubernetesPrune: got %v (present=%v)", got, ok)
	}
}

// TestGitopsWorkflow_LeavingReconcileForcesNew: Portainer has no way back
// from the reconcile engine, so asking for one replaces the workflow.
func TestGitopsWorkflow_LeavingReconcileForcesNew(t *testing.T) {
	diff, err := gitopsEngineChangeDiff(t, "reconcile", map[string]interface{}{"engine": "edgeStack"})
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if diff == nil || !diff.RequiresNew() {
		t.Fatal("leaving the reconcile engine must replace the workflow")
	}
}

// TestGitopsWorkflow_PlanRejectsInvalidCombinations turns what Portainer
// rejects at apply time, or would silently ignore, into plan errors.
func TestGitopsWorkflow_PlanRejectsInvalidCombinations(t *testing.T) {
	file := []interface{}{map[string]interface{}{"source_id": 3, "path": "k8s", "ref": "refs/heads/main"}}
	cases := map[string]struct {
		config map[string]interface{}
		want   string
	}{
		"interval without reconcile": {
			config: map[string]interface{}{
				"name": "platform", "engine": "edgeStack", "reconcile_interval": "5m",
				"artifact": []interface{}{map[string]interface{}{
					"name": "web", "deployment_type": "compose", "edge_group_ids": []interface{}{1}, "file": file,
				}},
			},
			want: "only applies",
		},
		"reconcile without interval": {
			config: map[string]interface{}{
				"name": "platform", "engine": "reconcile",
				"artifact": []interface{}{map[string]interface{}{
					"name": "web", "deployment_type": "kubernetes", "environment_ids": []interface{}{5}, "file": file,
				}},
			},
			want: "is required",
		},
		"artifact without target": {
			config: map[string]interface{}{
				"name": "platform",
				"artifact": []interface{}{map[string]interface{}{
					"name": "web", "deployment_type": "compose", "file": file,
				}},
			},
			want: "needs a target",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			r := resourceGitopsWorkflow()
			_, err := r.Diff(context.Background(), &terraform.InstanceState{}, terraform.NewResourceConfigRaw(tc.config), nil)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Errorf("expected an error containing %q, got %v", tc.want, err)
			}
		})
	}
}

// TestGitopsWorkflow_ReadDerivesReconcileEngine: Portainer reports no engine,
// so an imported reconcile workflow is recognised by its artifact types -
// otherwise a configuration saying `reconcile` would plan a second migration.
func TestGitopsWorkflow_ReadDerivesReconcileEngine(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/gitops/workflows/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 4, "name": "platform",
		"artifacts": []map[string]interface{}{{"id": 21, "name": "web", "type": "gitopsDeployment"}},
	}))

	r := resourceGitopsWorkflow()
	d := r.TestResourceData()
	d.SetId("4")
	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("engine"); got != "reconcile" {
		t.Errorf("engine: got %v", got)
	}
}
