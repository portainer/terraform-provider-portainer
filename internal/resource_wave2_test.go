package internal

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// =========================================================================
// Remainder of the Business Edition second wave: pod security rules, custom
// resources, GPU reporting, GitOps workflows, stack conversion and the two
// one-shot actions.
// =========================================================================

// TestPodSecurityRule_SendsNestedSections pins the payload shape: Portainer
// stores every section as its own object, even the ones that hold nothing but
// a switch.
func TestPodSecurityRule_SendsNestedSections(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/kubernetes/5/opa", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/kubernetes/5/opa", RespondJSON(http.StatusOK, map[string]interface{}{
		"enabled": true, "endPointID": 5,
		"privilegedContainers": map[string]interface{}{"enabled": true},
		"hostNamespaces":       map[string]interface{}{"enabled": true},
	}))

	r := resourceKubernetesPodSecurityRule()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 5)
	_ = d.Set("enabled", true)
	_ = d.Set("privileged_containers", true)
	_ = d.Set("host_namespaces", true)
	_ = d.Set("capabilities", []interface{}{map[string]interface{}{
		"enabled":       true,
		"allowed":       []interface{}{"NET_BIND_SERVICE"},
		"required_drop": []interface{}{"ALL"},
	}})
	_ = d.Set("host_ports", []interface{}{map[string]interface{}{
		"enabled": true, "host_network": false, "min": 30000, "max": 32767,
	}})
	_ = d.Set("host_filesystem", []interface{}{map[string]interface{}{
		"enabled": true,
		"allowed_path": []interface{}{map[string]interface{}{
			"path_prefix": "/var/log", "readonly": true,
		}},
	}})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("PUT", "/kubernetes/5/opa").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}

	if payload["endPointID"] != float64(5) || payload["enabled"] != true {
		t.Errorf("the rule identity was not sent: %v", payload)
	}
	privileged, ok := payload["privilegedContainers"].(map[string]interface{})
	if !ok || privileged["enabled"] != true {
		t.Errorf("a plain switch must still be sent as an object: %v", payload["privilegedContainers"])
	}
	capabilities, ok := payload["capabilities"].(map[string]interface{})
	if !ok {
		t.Fatalf("capabilities: got %v", payload["capabilities"])
	}
	if allowed, ok := capabilities["allowedCapabilities"].([]interface{}); !ok || len(allowed) != 1 {
		t.Errorf("the allowed capabilities were not sent: %v", capabilities["allowedCapabilities"])
	}
	ports, ok := payload["hostPorts"].(map[string]interface{})
	if !ok || ports["min"] != float64(30000) || ports["max"] != float64(32767) {
		t.Errorf("the host port range was not sent: %v", payload["hostPorts"])
	}
	filesystem, ok := payload["hostFilesystem"].(map[string]interface{})
	if !ok {
		t.Fatalf("hostFilesystem: got %v", payload["hostFilesystem"])
	}
	paths, ok := filesystem["allowedPaths"].([]interface{})
	if !ok || len(paths) != 1 {
		t.Fatalf("the allowed paths were not sent: %v", filesystem["allowedPaths"])
	}
	if paths[0].(map[string]interface{})["pathPrefix"] != "/var/log" {
		t.Errorf("the path prefix was not sent: %v", paths[0])
	}
}

// TestPodSecurityRule_ForbiddenSysctlsFieldName pins a field Portainer named
// badly in its own schema: the sysctl list is stored under a key that says
// "required drop capabilities".
func TestPodSecurityRule_ForbiddenSysctlsFieldName(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/kubernetes/5/opa", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/kubernetes/5/opa", RespondJSON(http.StatusOK, map[string]interface{}{"enabled": true}))

	r := resourceKubernetesPodSecurityRule()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 5)
	_ = d.Set("forbidden_sysctls", []interface{}{map[string]interface{}{
		"enabled": true, "sysctls": []interface{}{"kernel.shm*"},
	}})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("PUT", "/kubernetes/5/opa").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	section, ok := payload["forbiddenSysctlsList"].(map[string]interface{})
	if !ok {
		t.Fatalf("forbiddenSysctlsList: got %v", payload["forbiddenSysctlsList"])
	}
	list, ok := section["requiredDropCapabilities"].([]interface{})
	if !ok || len(list) != 1 || list[0] != "kernel.shm*" {
		t.Errorf("the sysctls must go under Portainer's own key name, got %v", section)
	}
}

// TestPodSecurityRule_UsersStrategyRanges covers the deepest part of the
// payload: run-as strategies with their identifier ranges.
func TestPodSecurityRule_UsersStrategyRanges(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/kubernetes/5/opa", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/kubernetes/5/opa", RespondJSON(http.StatusOK, map[string]interface{}{"enabled": true}))

	r := resourceKubernetesPodSecurityRule()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 5)
	_ = d.Set("users", []interface{}{map[string]interface{}{
		"enabled": true,
		"run_as_user": []interface{}{map[string]interface{}{
			"type": "MustRunAs",
			"id_range": []interface{}{
				map[string]interface{}{"min": 1000, "max": 2000},
			},
		}},
	}})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("PUT", "/kubernetes/5/opa").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	users := payload["users"].(map[string]interface{})
	runAsUser, ok := users["runAsUser"].(map[string]interface{})
	if !ok || runAsUser["type"] != "MustRunAs" {
		t.Fatalf("the run-as strategy was not sent: %v", users["runAsUser"])
	}
	ranges, ok := runAsUser["idrange"].([]interface{})
	if !ok || len(ranges) != 1 {
		t.Fatalf("the identifier ranges were not sent: %v", runAsUser["idrange"])
	}
	if ranges[0].(map[string]interface{})["min"] != float64(1000) {
		t.Errorf("the range bounds were not sent: %v", ranges[0])
	}
}

// TestPodSecurityRule_DeleteDisablesRule pins the destroy: a cluster always
// has a rule, so switching it off is the only "absent" state there is - and
// leaving an enforced policy behind for a rule Terraform no longer manages
// would be worse than turning it off.
func TestPodSecurityRule_DeleteDisablesRule(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/kubernetes/5/opa", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceKubernetesPodSecurityRule()
	d := r.TestResourceData()
	d.SetId("5")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("PUT", "/kubernetes/5/opa").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if payload["enabled"] != false {
		t.Errorf("destroy must switch the rule off, got %v", payload["enabled"])
	}
	if d.Id() != "" {
		t.Error("the resource must be removed from state")
	}
}

// TestDataSourceKubernetesCustomResources_QueriesDefinition pins the required
// query parameter, without which Portainer does not know what to list.
func TestDataSourceKubernetesCustomResources_QueriesDefinition(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/5/customresources", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"name": "prod", "namespace": "apps", "definitionName": "widgets.example.com", "uid": "abc", "creationDate": "2026-01-01"},
	}))

	ds := dataSourceKubernetesCustomResources()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 5)
	_ = d.Set("definition", "widgets.example.com")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	req := mock.FindRequest("GET", "/kubernetes/5/customresources")
	if req == nil || !strings.Contains(req.Query, "definition=widgets.example.com") {
		t.Errorf("expected the definition in the query, got %q", req.Query)
	}
	resources := d.Get("resources").([]interface{})
	if len(resources) != 1 || resources[0].(map[string]interface{})["namespace"] != "apps" {
		t.Errorf("the resources were not read: %v", resources)
	}
}

// TestDataSourceKubernetesCustomResource_ScopeSelectsPath covers the two paths
// Portainer has: a namespaced resource and a cluster-scoped one, chosen by
// whether a namespace was given.
func TestDataSourceKubernetesCustomResource_ScopeSelectsPath(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/5/customresources/apps/prod", RespondJSON(http.StatusOK, map[string]interface{}{
		"apiVersion": "example.com/v1", "kind": "Widget",
	}))
	mock.On("GET", "/kubernetes/5/customresources/cluster-wide", RespondJSON(http.StatusOK, map[string]interface{}{
		"apiVersion": "example.com/v1", "kind": "ClusterWidget",
	}))

	ds := dataSourceKubernetesCustomResource()

	namespaced := ds.TestResourceData()
	_ = namespaced.Set("endpoint_id", 5)
	_ = namespaced.Set("definition", "widgets.example.com")
	_ = namespaced.Set("name", "prod")
	_ = namespaced.Set("namespace", "apps")
	if err := rcRead(ds, namespaced, mock.Client()); err != nil {
		t.Fatalf("namespaced read failed: %v", err)
	}
	if !strings.Contains(namespaced.Get("manifest").(string), "Widget") {
		t.Errorf("the manifest was not carried through: %v", namespaced.Get("manifest"))
	}

	clusterScoped := ds.TestResourceData()
	_ = clusterScoped.Set("endpoint_id", 5)
	_ = clusterScoped.Set("definition", "widgets.example.com")
	_ = clusterScoped.Set("name", "cluster-wide")
	if err := rcRead(ds, clusterScoped, mock.Client()); err != nil {
		t.Fatalf("cluster-scoped read failed: %v", err)
	}
	if mock.FindRequest("GET", "/kubernetes/5/customresources/cluster-wide") == nil {
		t.Error("a resource without a namespace must go to the cluster-scoped path")
	}
}

// TestDataSourceKubernetesGPU_ReadsSummaryAndWorkloads covers the GPU report,
// including a workload that could not be scheduled - which is the thing worth
// looking at in it.
func TestDataSourceKubernetesGPU_ReadsSummaryAndWorkloads(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/5/gpu", RespondJSON(http.StatusOK, map[string]interface{}{
		"Summary": map[string]interface{}{
			"GPUNodeCount": 2, "DegradedCount": 1,
			"OperatorDetected": true, "DevicePluginDetected": true,
			"TotalCapacity":    map[string]int64{"nvidia.com/gpu": 8},
			"TotalAllocatable": map[string]int64{"nvidia.com/gpu": 8},
			"TotalAllocated":   map[string]int64{"nvidia.com/gpu": 5},
		},
		"Nodes": []map[string]interface{}{
			{
				"Name": "gpu-1", "Status": "Ready", "StatusReason": "",
				"Capacity":    map[string]int64{"nvidia.com/gpu": 4},
				"Allocatable": map[string]int64{"nvidia.com/gpu": 4},
				"Allocated":   map[string]int64{"nvidia.com/gpu": 3},
			},
		},
		"Workloads": []map[string]interface{}{
			{
				"PodName": "trainer-0", "Namespace": "ml", "NodeName": "",
				"PodPhase": "Pending", "OwnerKind": "StatefulSet", "OwnerName": "trainer",
				"SchedulingIssue": "insufficient nvidia.com/gpu",
				"GPURequests":     map[string]int64{"nvidia.com/gpu": 4},
			},
		},
	}))

	ds := dataSourceKubernetesGPU()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 5)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("gpu_node_count"); got != 2 {
		t.Errorf("gpu_node_count: got %v", got)
	}
	if got := d.Get("degraded_count"); got != 1 {
		t.Errorf("degraded_count: got %v", got)
	}
	capacity := d.Get("total_capacity").(map[string]interface{})
	if capacity["nvidia.com/gpu"] != "8" && capacity["nvidia.com/gpu"] != 8 {
		t.Errorf("total_capacity: got %v", capacity)
	}
	nodes := d.Get("nodes").([]interface{})
	if len(nodes) != 1 || nodes[0].(map[string]interface{})["name"] != "gpu-1" {
		t.Errorf("the nodes were not read: %v", nodes)
	}
	workloads := d.Get("workloads").([]interface{})
	if len(workloads) != 1 {
		t.Fatalf("expected one workload, got %d", len(workloads))
	}
	workload := workloads[0].(map[string]interface{})
	if workload["scheduling_issue"] != "insufficient nvidia.com/gpu" {
		t.Errorf("the scheduling issue was not read: %+v", workload)
	}
}

func gitopsWorkflowData(t *testing.T) (*schema.Resource, *schema.ResourceData) {
	t.Helper()
	r := resourceGitopsWorkflow()
	d := r.TestResourceData()
	_ = d.Set("name", "platform")
	_ = d.Set("artifact", []interface{}{map[string]interface{}{
		"name":            "monitoring",
		"type":            "edgeStack",
		"deployment_type": "compose",
		"edge_group_ids":  []interface{}{1, 2},
		"file": []interface{}{map[string]interface{}{
			"source_id": 3, "path": "portainer.yaml", "ref": "refs/heads/main",
		}},
		"config": []interface{}{map[string]interface{}{
			"environment":    map[string]interface{}{"LOG_LEVEL": "debug"},
			"registry_ids":   []interface{}{7},
			"pre_pull_image": true,
			"parallel": []interface{}{map[string]interface{}{
				"batch_count": 5, "delay": "30", "failure_action": "continue",
			}},
		}},
	}})
	return r, d
}

// TestGitopsWorkflow_CreateSendsArtifacts pins the create payload, whose
// artifacts carry their name and whose targets are nested a level down.
func TestGitopsWorkflow_CreateSendsArtifacts(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/workflows", RespondJSON(http.StatusOK, map[string]interface{}{"id": 4}))
	mock.On("GET", "/gitops/workflows/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 4, "name": "platform",
		"artifacts": []map[string]interface{}{{"id": 11, "name": "monitoring"}},
	}))

	r, d := gitopsWorkflowData(t)
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload struct {
		Name      string `json:"name"`
		Artifacts []struct {
			Name           string `json:"name"`
			DeploymentType string `json:"deploymentType"`
			Targets        struct {
				EdgeGroups []int `json:"edgeGroups"`
			} `json:"targets"`
			Files []struct {
				SourceID int    `json:"sourceId"`
				Path     string `json:"path"`
				Ref      string `json:"ref"`
			} `json:"files"`
			Config map[string]interface{} `json:"config"`
		} `json:"artifacts"`
	}
	if err := mock.FindRequest("POST", "/gitops/workflows").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if payload.Name != "platform" || len(payload.Artifacts) != 1 {
		t.Fatalf("the workflow was not sent: %+v", payload)
	}
	artifact := payload.Artifacts[0]
	if artifact.Name != "monitoring" || artifact.DeploymentType != "compose" {
		t.Errorf("the artifact identity was not sent: %+v", artifact)
	}
	if len(artifact.Targets.EdgeGroups) != 2 {
		t.Errorf("the edge groups must be nested under targets: %+v", artifact.Targets)
	}
	if len(artifact.Files) != 1 || artifact.Files[0].Path != "portainer.yaml" {
		t.Errorf("the files were not sent: %+v", artifact.Files)
	}
	if artifact.Config["prePullImage"] != true {
		t.Errorf("the config was not sent: %v", artifact.Config)
	}
	parallel, ok := artifact.Config["parallelConfig"].(map[string]interface{})
	if !ok || parallel["batchCount"] != float64(5) {
		t.Errorf("the parallel config was not sent: %v", artifact.Config["parallelConfig"])
	}

	// The read fills in the identifier the update payload needs.
	artifacts := d.Get("artifact").([]interface{})
	if got := artifacts[0].(map[string]interface{})["artifact_id"]; got != 11 {
		t.Errorf("artifact_id: expected the identifier Portainer assigned, got %v", got)
	}
}

// TestGitopsWorkflow_UpdateKeysArtifactsById pins the difference between the
// two payloads: an update identifies an artifact by its identifier and type
// where a create names it.
func TestGitopsWorkflow_UpdateKeysArtifactsById(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/gitops/workflows/4", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/gitops/workflows/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 4, "name": "platform",
		"artifacts": []map[string]interface{}{{"id": 11, "name": "monitoring"}},
	}))

	r, d := gitopsWorkflowData(t)
	d.SetId("4")
	artifacts := d.Get("artifact").([]interface{})
	block := artifacts[0].(map[string]interface{})
	block["artifact_id"] = 11
	_ = d.Set("artifact", []interface{}{block})

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	var payload struct {
		Artifacts []map[string]interface{} `json:"artifacts"`
	}
	if err := mock.FindRequest("PUT", "/gitops/workflows/4").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if len(payload.Artifacts) != 1 {
		t.Fatalf("expected one artifact, got %d", len(payload.Artifacts))
	}
	artifact := payload.Artifacts[0]
	if artifact["id"] != float64(11) || artifact["type"] != "edgeStack" {
		t.Errorf("an update must identify the artifact by id and type: %v", artifact)
	}
	if _, ok := artifact["name"]; ok {
		t.Errorf("an update payload must not carry the artifact name: %v", artifact["name"])
	}
}

// TestGitopsWorkflow_DestroyModeSelectsEndpoint is the reason on_destroy
// exists: the two endpoints do genuinely different things to the deployed
// stacks, and which one runs must be a stated decision.
func TestGitopsWorkflow_DestroyModeSelectsEndpoint(t *testing.T) {
	for mode, path := range map[string]string{
		"destroy": "/gitops/workflows/4/destroy",
		"detach":  "/gitops/workflows/4/detach",
	} {
		mock := NewMockServer(t)
		mock.On("DELETE", path, RespondJSON(http.StatusOK, map[string]interface{}{}))

		r := resourceGitopsWorkflow()
		d := r.TestResourceData()
		d.SetId("4")
		_ = d.Set("on_destroy", mode)

		if err := rcDelete(r, d, mock.Client()); err != nil {
			t.Fatalf("%s: Delete failed: %v", mode, err)
		}
		if mock.FindRequest("DELETE", path) == nil {
			t.Errorf("%s: expected the request to go to %s", mode, path)
		}
	}
}

// TestDataSourceStackConversion_ReturnsFiles covers the conversion preview.
func TestDataSourceStackConversion_ReturnsFiles(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/stacks/9/convert", RespondJSON(http.StatusOK, map[string]interface{}{
		"files": map[string]string{
			"deployment.yaml": "apiVersion: apps/v1\n",
			"service.yaml":    "apiVersion: v1\n",
		},
	}))

	ds := dataSourceStackConversion()
	d := ds.TestResourceData()
	_ = d.Set("stack_id", 9)
	_ = d.Set("target_format", "kubernetes")
	_ = d.Set("namespace", "apps")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("POST", "/stacks/9/convert").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if payload["targetFormat"] != "kubernetes" || payload["namespace"] != "apps" {
		t.Errorf("the conversion request was not sent: %v", payload)
	}

	files := d.Get("files").(map[string]interface{})
	if len(files) != 2 || !strings.Contains(files["deployment.yaml"].(string), "apps/v1") {
		t.Errorf("the converted files were not read: %v", files)
	}
}

// TestKubernetesClusterUpgrade_PostsToEnvironment covers the upgrade action.
func TestKubernetesClusterUpgrade_PostsToEnvironment(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/cloud/endpoints/5/upgrade", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceKubernetesClusterUpgrade()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 5)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if mock.FindRequest("POST", "/cloud/endpoints/5/upgrade") == nil {
		t.Fatal("no upgrade request was sent")
	}
	if !strings.HasPrefix(d.Id(), "5-upgrade-") {
		t.Errorf("the ID should name the environment and the run, got %q", d.Id())
	}
}

// TestUserMembershipsSync_PostsToUser covers the membership sync action.
func TestUserMembershipsSync_PostsToUser(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/users/7/memberships/sync", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceUserMembershipsSync()
	d := r.TestResourceData()
	_ = d.Set("user_id", 7)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if mock.FindRequest("POST", "/users/7/memberships/sync") == nil {
		t.Fatal("no sync request was sent")
	}
	if !strings.HasPrefix(d.Id(), "7-memberships-sync-") {
		t.Errorf("the ID should name the user and the run, got %q", d.Id())
	}
}

// TestWebhook_ResourceChangeReassigns is the reason resource_id stopped
// forcing replacement: Portainer can move a webhook in place, which keeps its
// token and therefore every URL already handed out.
func TestWebhook_ResourceChangeReassigns(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/webhooks/3/reassign", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/webhooks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 3, "EndpointId": 1, "ResourceId": "svc-new", "Type": 1, "Token": "tok"},
	}))

	r := resourceWebhook()

	// The update path is driven by HasChange, which only reports a change when
	// the resource data carries a real diff - so one is built through the SDK's
	// own Diff rather than written with Set.
	state := &terraform.InstanceState{
		ID: "3",
		Attributes: map[string]string{
			"id": "3", "endpoint_id": "1", "resource_id": "svc-old", "webhook_type": "1",
			"reassign_on_change": "true",
		},
	}
	config := terraform.NewResourceConfigRaw(map[string]interface{}{
		"endpoint_id": 1, "resource_id": "svc-new", "webhook_type": 1,
		"reassign_on_change": true,
	})
	diff, err := r.Diff(context.Background(), state, config, nil)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	d, err := schema.InternalMap(r.Schema).Data(state, diff)
	if err != nil {
		t.Fatalf("building resource data from the diff failed: %v", err)
	}

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	req := mock.FindRequest("PUT", "/webhooks/3/reassign")
	if req == nil {
		t.Fatal("changing the target resource must reassign the webhook")
	}
	var payload map[string]interface{}
	if err := req.DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if payload["ResourceID"] != "svc-new" || payload["WebhookType"] != float64(1) {
		t.Errorf("the reassign payload uses PascalCase keys, got %v", payload)
	}
}

// TestDataSourceKubernetesCRDs_ListAndInspect covers the two definition data
// sources, which the other tests reach only indirectly.
func TestDataSourceKubernetesCRDs_ListAndInspect(t *testing.T) {
	definition := map[string]interface{}{
		"name": "widgets.example.com", "group": "example.com", "scope": "Namespaced",
		"creationDate": "2026-01-01", "releaseName": "widgets", "releaseNamespace": "ops",
		"releaseVersion": "1.2.3",
	}

	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/5/customresourcedefinitions", RespondJSON(http.StatusOK, []interface{}{definition}))
	mock.On("GET", "/kubernetes/5/customresourcedefinitions/widgets.example.com",
		RespondJSON(http.StatusOK, definition))

	list := dataSourceKubernetesCustomResourceDefinitions()
	dList := list.TestResourceData()
	_ = dList.Set("endpoint_id", 5)
	if err := rcRead(list, dList, mock.Client()); err != nil {
		t.Fatalf("list read failed: %v", err)
	}
	definitions := dList.Get("definitions").([]interface{})
	if len(definitions) != 1 {
		t.Fatalf("expected one definition, got %d", len(definitions))
	}
	entry := definitions[0].(map[string]interface{})
	if entry["name"] != "widgets.example.com" || entry["scope"] != "Namespaced" {
		t.Errorf("the definition was not read: %+v", entry)
	}
	if entry["release_name"] != "widgets" || entry["release_version"] != "1.2.3" {
		t.Errorf("the Helm release fields were not read: %+v", entry)
	}

	single := dataSourceKubernetesCustomResourceDefinition()
	dSingle := single.TestResourceData()
	_ = dSingle.Set("endpoint_id", 5)
	_ = dSingle.Set("name", "widgets.example.com")
	if err := rcRead(single, dSingle, mock.Client()); err != nil {
		t.Fatalf("inspect read failed: %v", err)
	}
	if got := dSingle.Get("scope"); got != "Namespaced" {
		t.Errorf("scope: got %v", got)
	}
	// The name is an argument, so a response is never allowed to rewrite it.
	if got := dSingle.Get("name"); got != "widgets.example.com" {
		t.Errorf("name: the argument must survive the read, got %v", got)
	}
}

// TestPodSecurityRule_RemainingSections covers the sections the other pod
// security tests do not touch, so no part of the payload builder goes
// unexercised.
func TestPodSecurityRule_RemainingSections(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/kubernetes/5/opa", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/kubernetes/5/opa", RespondJSON(http.StatusOK, map[string]interface{}{"enabled": true}))

	r := resourceKubernetesPodSecurityRule()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 5)
	_ = d.Set("allow_proc_mount", []interface{}{map[string]interface{}{
		"enabled": true, "proc_mount_type": "Default",
	}})
	_ = d.Set("allow_flex_volumes", []interface{}{map[string]interface{}{
		"enabled": true, "allowed_volumes": []interface{}{"azure-disk"},
	}})
	_ = d.Set("app_armor", []interface{}{map[string]interface{}{
		"enabled": true, "types": []interface{}{"runtime/default"},
	}})
	_ = d.Set("sec_comp", []interface{}{map[string]interface{}{
		"enabled": true, "types": []interface{}{"RuntimeDefault"},
	}})
	_ = d.Set("volume_types", []interface{}{map[string]interface{}{
		"enabled": true, "allowed_types": []interface{}{"configMap", "emptyDir"},
	}})
	_ = d.Set("selinux", []interface{}{map[string]interface{}{
		"enabled": true,
		"allowed_context": []interface{}{map[string]interface{}{
			"user": "system_u", "role": "system_r", "type": "container_t", "level": "s0",
		}},
	}})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("PUT", "/kubernetes/5/opa").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}

	if proc := payload["allowProcMount"].(map[string]interface{}); proc["procMountType"] != "Default" {
		t.Errorf("the proc mount type was not sent: %v", proc)
	}
	if flex := payload["allowFlexVolumes"].(map[string]interface{}); len(flex["allowedVolumes"].([]interface{})) != 1 {
		t.Errorf("the flex volumes were not sent: %v", flex)
	}
	// Portainer capitalises this one field where the rest of the section is
	// lower case.
	if armor := payload["appArmor"].(map[string]interface{}); len(armor["AppArmorType"].([]interface{})) != 1 {
		t.Errorf("the AppArmor profiles were not sent under Portainer's key: %v", armor)
	}
	if comp := payload["secComp"].(map[string]interface{}); len(comp["secCompType"].([]interface{})) != 1 {
		t.Errorf("the seccomp profiles were not sent: %v", comp)
	}
	if volumes := payload["volumeTypes"].(map[string]interface{}); len(volumes["allowedTypes"].([]interface{})) != 2 {
		t.Errorf("the volume types were not sent: %v", volumes)
	}
	selinux := payload["selinux"].(map[string]interface{})
	contexts, ok := selinux["allowedCapabilities"].([]interface{})
	if !ok || len(contexts) != 1 {
		t.Fatalf("the SELinux contexts were not sent: %v", selinux)
	}
	if contexts[0].(map[string]interface{})["type"] != "container_t" {
		t.Errorf("the SELinux context was not sent: %v", contexts[0])
	}
}

// TestPodSecurityRule_ReadOnlyReportsSwitches pins what the read does and does
// not touch: the switches come back from Portainer, the list sections stay as
// configured.
func TestPodSecurityRule_ReadOnlyReportsSwitches(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/5/opa", RespondJSON(http.StatusOK, map[string]interface{}{
		"enabled": true, "endPointID": 5,
		"privilegedContainers":     map[string]interface{}{"enabled": true},
		"restrictDefaultNamespace": map[string]interface{}{"enabled": true},
	}))

	r := resourceKubernetesPodSecurityRule()
	d := r.TestResourceData()
	d.SetId("5")
	_ = d.Set("capabilities", []interface{}{map[string]interface{}{
		"enabled": true, "allowed": []interface{}{"NET_BIND_SERVICE"},
	}})

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("privileged_containers"); got != true {
		t.Errorf("the switches must be read back, got %v", got)
	}
	if got := d.Get("restrict_default_namespace"); got != true {
		t.Errorf("restrict_default_namespace: got %v", got)
	}
	blocks := d.Get("capabilities").([]interface{})
	if len(blocks) != 1 {
		t.Fatalf("the configured section must survive the read, got %d", len(blocks))
	}
	if allowed := blocks[0].(map[string]interface{})["allowed"].([]interface{}); len(allowed) != 1 {
		t.Errorf("the read must not touch the list sections, got %v", allowed)
	}
}

// TestPodSecurityRule_MissingRuleLeavesState covers an environment that was
// removed outside Terraform.
func TestPodSecurityRule_MissingRuleLeavesState(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/5/opa", RespondString(http.StatusNotFound,
		"application/json", `{"message":"environment not found"}`))

	r := resourceKubernetesPodSecurityRule()
	d := r.TestResourceData()
	d.SetId("5")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Error("a missing environment must take the rule out of state")
	}
}

// TestGitopsWorkflow_MissingWorkflowLeavesState covers a workflow removed
// outside Terraform, and the detach path's own not-found handling.
func TestGitopsWorkflow_MissingWorkflowLeavesState(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/gitops/workflows/4", RespondString(http.StatusNotFound,
		"application/json", `{"message":"workflow not found"}`))
	mock.On("DELETE", "/gitops/workflows/4/destroy", RespondString(http.StatusNotFound,
		"application/json", `{"message":"workflow not found"}`))

	r := resourceGitopsWorkflow()

	read := r.TestResourceData()
	read.SetId("4")
	if err := rcRead(r, read, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if read.Id() != "" {
		t.Error("a deleted workflow must leave state")
	}

	del := r.TestResourceData()
	del.SetId("4")
	_ = del.Set("on_destroy", "destroy")
	if err := rcDelete(r, del, mock.Client()); err != nil {
		t.Fatalf("destroying an already-deleted workflow must not fail: %v", err)
	}
}

// TestGitopsWorkflow_ConfigOmittedWhenEmpty keeps an artifact from carrying a
// config object full of zero values Portainer would read as settings.
func TestGitopsWorkflow_ConfigOmittedWhenEmpty(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/gitops/workflows", RespondJSON(http.StatusOK, map[string]interface{}{"id": 4}))
	mock.On("GET", "/gitops/workflows/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"id": 4, "name": "platform", "artifacts": []interface{}{},
	}))

	r := resourceGitopsWorkflow()
	d := r.TestResourceData()
	_ = d.Set("name", "platform")
	_ = d.Set("artifact", []interface{}{map[string]interface{}{
		"name":            "monitoring",
		"deployment_type": "compose",
		"edge_group_ids":  []interface{}{1},
		"file": []interface{}{map[string]interface{}{
			"source_id": 3, "path": "portainer.yaml", "ref": "refs/heads/main",
		}},
	}})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload struct {
		Artifacts []map[string]interface{} `json:"artifacts"`
	}
	if err := mock.FindRequest("POST", "/gitops/workflows").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if _, ok := payload.Artifacts[0]["config"]; ok {
		t.Errorf("an artifact with no configured options must not carry a config object: %v", payload.Artifacts[0]["config"])
	}
}

// TestWebhook_ResourceChangeReplacesByDefault is the guard on the regression
// that opting into reassign would otherwise have caused: the reassign endpoint
// does not exist in Portainer CE, so a webhook repointed without asking for it
// must still be replaced the way it always was.
func TestWebhook_ResourceChangeReplacesByDefault(t *testing.T) {
	r := resourceWebhook()

	state := &terraform.InstanceState{
		ID: "3",
		Attributes: map[string]string{
			"id": "3", "endpoint_id": "1", "resource_id": "svc-old", "webhook_type": "1",
		},
	}
	config := terraform.NewResourceConfigRaw(map[string]interface{}{
		"endpoint_id": 1, "resource_id": "svc-new", "webhook_type": 1,
	})

	diff, err := r.Diff(context.Background(), state, config, nil)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if diff == nil || !diff.RequiresNew() {
		t.Fatal("changing the target resource must replace the webhook unless reassign_on_change is set")
	}
}

// TestWebhook_ReassignOptInAvoidsReplacement covers the other side: asking for
// the Business Edition behaviour turns the same change into an update.
func TestWebhook_ReassignOptInAvoidsReplacement(t *testing.T) {
	r := resourceWebhook()

	state := &terraform.InstanceState{
		ID: "3",
		Attributes: map[string]string{
			"id": "3", "endpoint_id": "1", "resource_id": "svc-old",
			"webhook_type": "1", "reassign_on_change": "true",
		},
	}
	config := terraform.NewResourceConfigRaw(map[string]interface{}{
		"endpoint_id": 1, "resource_id": "svc-new", "webhook_type": 1,
		"reassign_on_change": true,
	})

	diff, err := r.Diff(context.Background(), state, config, nil)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	if diff == nil {
		t.Fatal("expected a diff")
	}
	if diff.RequiresNew() {
		t.Error("with reassign_on_change the webhook must be updated in place, not replaced")
	}
}
