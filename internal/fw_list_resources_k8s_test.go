package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// k8sIdentityOf is the identity of object "obj" of case c.
func k8sIdentityOf(c k8sManifestCase) map[string]knownvalue.Check {
	ident := map[string]knownvalue.Check{
		"endpoint_id": knownvalue.Int64Exact(1),
		"name":        knownvalue.StringExact("obj"),
	}
	switch strings.Count(c.id, ":") {
	case 2:
		ident["namespace"] = knownvalue.StringExact("default")
	case 3:
		ident["namespace"] = knownvalue.StringExact("default")
		ident["type"] = knownvalue.StringExact("persistent-volume-claim")
	}
	return ident
}

// TestKubernetesManifestResources_Identity: every manifest resource stores
// the parts of its ID as identity and imports by it.
func TestKubernetesManifestResources_Identity(t *testing.T) {
	t.Parallel()
	for _, c := range k8sManifestCases {
		t.Run(c.resType, func(t *testing.T) {
			t.Parallel()
			mock := NewMockServer(t)
			fake := newFakeK8sAPI()
			fake.serve(mock, c.collection, "obj")
			addr := c.resType + ".r"
			config := c.config(mock, "obj", "a")
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_12_0), skipOnOpenTofuIdentity},
				Steps: []resource.TestStep{
					{
						Config: config,
						ConfigStateChecks: []statecheck.StateCheck{
							statecheck.ExpectIdentity(addr, k8sIdentityOf(c)),
							statecheck.ExpectKnownValue(addr, tfjsonpath.New("id"), knownvalue.StringExact(c.id)),
						},
					},
					{
						Config:          config,
						ResourceName:    addr,
						ImportState:     true,
						ImportStateKind: resource.ImportBlockWithResourceIdentity,
						// As after an import by ID, manifest is not read back.
						ExpectNonEmptyPlan: true,
					},
				},
			})
		})
	}
}

// TestKubernetesVolume_IdentityClusterScoped: a persistent volume has no
// namespace in its identity and imports without one.
func TestKubernetesVolume_IdentityClusterScoped(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeK8sAPI()
	fake.serve(mock, "/endpoints/1/kubernetes/api/v1/persistentvolumes", "pv1")
	config := mockProviderConfig(mock) + `
resource "portainer_kubernetes_volume" "pv" {
  endpoint_id = 1
  type        = "persistent-volume"
  manifest    = jsonencode({ apiVersion = "v1", kind = "PersistentVolume", metadata = { name = "pv1" } })
}
`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_12_0), skipOnOpenTofuIdentity},
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectIdentity("portainer_kubernetes_volume.pv", map[string]knownvalue.Check{
						"endpoint_id": knownvalue.Int64Exact(1),
						"namespace":   knownvalue.Null(),
						"type":        knownvalue.StringExact("persistent-volume"),
						"name":        knownvalue.StringExact("pv1"),
					}),
				},
			},
			{
				Config:             config,
				ResourceName:       "portainer_kubernetes_volume.pv",
				ImportState:        true,
				ImportStateKind:    resource.ImportBlockWithResourceIdentity,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestKubernetesManifestResource_IdentityFollowsRename: a renamed object is
// recreated in an update, and the identity follows the new ID.
func TestKubernetesManifestResource_IdentityFollowsRename(t *testing.T) {
	t.Parallel()
	c := k8sManifestCaseFor(t, "portainer_kubernetes_configmaps")
	mock := NewMockServer(t)
	fake := newFakeK8sAPI()
	fake.serve(mock, c.collection, "obj", "obj2")
	addr := c.resType + ".r"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_12_0), skipOnOpenTofuIdentity},
		Steps: []resource.TestStep{
			{Config: c.config(mock, "obj", "a")},
			{
				Config: c.config(mock, "obj2", "a"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction(addr, plancheck.ResourceActionUpdate)},
				},
				ConfigStateChecks: []statecheck.StateCheck{
					statecheck.ExpectIdentity(addr, map[string]knownvalue.Check{
						"endpoint_id": knownvalue.Int64Exact(1),
						"namespace":   knownvalue.StringExact("default"),
						"name":        knownvalue.StringExact("obj2"),
					}),
				},
			},
		},
	})
}

// TestKubernetesManifestResources_IdentityAfterUpgradeFromV300: state written
// by v3.0.0 has no identity; the first refresh stores it and plans nothing.
func TestKubernetesManifestResources_IdentityAfterUpgradeFromV300(t *testing.T) {
	t.Parallel()
	c := k8sManifestCaseFor(t, "portainer_kubernetes_secret")
	mock := NewMockServer(t)
	fake := newFakeK8sAPI()
	fake.serve(mock, c.collection, "obj")
	config := c.config(mock, "obj", "a")
	resource.Test(t, resource.TestCase{
		IsUnitTest:             true,
		TerraformVersionChecks: []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_12_0), skipOnOpenTofuIdentity},
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				ConfigPlanChecks:         resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				ConfigStateChecks:        []statecheck.StateCheck{statecheck.ExpectIdentity(c.resType+".r", k8sIdentityOf(c))},
			},
		},
	})
}

// serveK8sList answers GET listPath with the objects the fake stores under
// collection, the way the API server lists them: no apiVersion and kind on
// the items, namespace in their metadata.
func serveK8sList(m *MockServer, f *fakeK8sAPI, listPath, collection, apiVersion, kind, namespace string) {
	m.On("GET", listPath, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		keys := make([]string, 0, len(f.objects))
		for k := range f.objects {
			if strings.HasPrefix(k, collection+"/") {
				keys = append(keys, k)
			}
		}
		sort.Strings(keys)
		items := make([]map[string]interface{}, 0, len(keys))
		for _, k := range keys {
			live := k8sFakeLive(f.objects[k])
			delete(live, "apiVersion")
			delete(live, "kind")
			if namespace != "" {
				live["metadata"].(map[string]interface{})["namespace"] = namespace
			}
			items = append(items, live)
		}
		f.mu.Unlock()
		RespondJSON(http.StatusOK, map[string]interface{}{"apiVersion": apiVersion, "kind": kind + "List", "items": items})(w, r)
	})
}

// TestKubernetesManifestResources_List: every manifest kind is listed with
// its identity, with and without a namespace; include_resource builds the
// state through Read, with the live object as manifest; an object another
// object controls is left out.
func TestKubernetesManifestResources_List(t *testing.T) {
	t.Parallel()
	for _, c := range k8sManifestCases {
		t.Run(c.resType, func(t *testing.T) {
			t.Parallel()
			mock := NewMockServer(t)
			fake := newFakeK8sAPI()
			fake.serve(mock, c.collection, "obj")
			namespaced := strings.Contains(c.collection, "/namespaces/default/")
			ns := ""
			if namespaced {
				ns = "default"
			}
			serveK8sList(mock, fake, c.collection, c.collection, c.apiVersion, c.kind, ns)
			allNamespaces := strings.Replace(c.collection, "/namespaces/default", "", 1)
			if allNamespaces != c.collection {
				serveK8sList(mock, fake, allNamespaces, c.collection, c.apiVersion, c.kind, ns)
			}
			if c.resType == "portainer_kubernetes_volume" {
				empty := RespondJSON(http.StatusOK, map[string]interface{}{"items": []interface{}{}})
				mock.On("GET", "/endpoints/1/kubernetes/api/v1/persistentvolumes", empty)
				mock.On("GET", "/endpoints/1/kubernetes/apis/storage.k8s.io/v1/volumeattachments", empty)
			}
			// A controlled object (a Job of a CronJob, ...) is not listed.
			fake.mu.Lock()
			fake.objects[c.collection+"/owned"] = map[string]interface{}{"metadata": map[string]interface{}{
				"name": "owned", "ownerReferences": []interface{}{map[string]interface{}{"kind": "Owner", "name": "x", "controller": true}},
			}}
			fake.mu.Unlock()

			display := "obj"
			if namespaced {
				display = "default/obj"
			}
			if c.resType == "portainer_kubernetes_volume" {
				display = "persistent-volume-claim default/obj"
			}
			typ := c.resType
			nsFilter := ""
			if namespaced {
				nsFilter = `namespace = "default"`
			}
			query := fmt.Sprintf(`
list %[1]q "all" {
  provider         = portainer
  include_resource = true
  config {
    endpoint_id = 1
  }
}

list %[1]q "match" {
  provider = portainer
  config {
    endpoint_id = 1
    %[2]s
  }
}
`, typ, nsFilter)
			byName := queryfilter.ByDisplayName(knownvalue.StringExact(display))
			values := []querycheck.KnownValueCheck{
				{Path: tfjsonpath.New("id"), KnownValue: knownvalue.StringExact(c.id)},
				{Path: tfjsonpath.New("endpoint_id"), KnownValue: knownvalue.Int64Exact(1)},
			}
			if !c.isSecret() {
				values = append(values, querycheck.KnownValueCheck{
					Path:       tfjsonpath.New("manifest"),
					KnownValue: knownvalue.StringRegexp(regexp.MustCompile(`(?s)apiVersion: ` + regexp.QuoteMeta(c.apiVersion) + `\n.*kind: ` + c.kind + `\n.*app: a`)),
				})
			}
			checks := []querycheck.QueryResultCheck{
				querycheck.ExpectLength(typ+".all", 1),
				querycheck.ExpectResourceDisplayName(typ+".all", byName, knownvalue.StringExact(display)),
				querycheck.ExpectResourceKnownValues(typ+".all", byName, values),
				querycheck.ExpectLength(typ+".match", 1),
				querycheck.ExpectIdentity(typ+".match", k8sIdentityOf(c)),
			}
			if namespaced {
				query += fmt.Sprintf(`
list %q "miss" {
  provider = portainer
  config {
    endpoint_id = 1
    namespace      = "other"
  }
}
`, typ)
				mock.On("GET", strings.Replace(c.collection, "/namespaces/default/", "/namespaces/other/", 1),
					RespondJSON(http.StatusOK, map[string]interface{}{"items": []interface{}{}}))
				checks = append(checks, querycheck.ExpectLength(typ+".miss", 0))
			}
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_14_0), skipOnOpenTofuList},
				Steps: []resource.TestStep{
					{Config: c.config(mock, "obj", "a")},
					{Query: true, Config: query, QueryResultChecks: checks},
				},
			})
		})
	}
}

func (c k8sManifestCase) isSecret() bool { return c.resType == "portainer_kubernetes_secret" }

// TestK8sListManifest: a listed object becomes a manifest without the fields
// the API server manages.
func TestK8sListManifest(t *testing.T) {
	t.Parallel()
	obj := map[string]interface{}{
		"metadata": map[string]interface{}{
			"name": "cm", "namespace": "default", "uid": "u", "resourceVersion": "1", "creationTimestamp": "t",
			"managedFields": []interface{}{}, "labels": map[string]interface{}{"app": "a"},
			"annotations": map[string]interface{}{"kubectl.kubernetes.io/last-applied-configuration": "{}"},
		},
		"data":   map[string]interface{}{"k": "v"},
		"status": map[string]interface{}{},
	}
	got, err := k8sListManifest(obj, "v1", "ConfigMap")
	if err != nil {
		t.Fatal(err)
	}
	want := "apiVersion: v1\ndata:\n  k: v\nkind: ConfigMap\nmetadata:\n  labels:\n    app: a\n  name: cm\n  namespace: default\n"
	if got != want {
		t.Errorf("k8sListManifest:\n%s\nwant:\n%s", got, want)
	}
	if _, ok := obj["status"]; !ok {
		t.Error("k8sListManifest changed its input")
	}
}

// The Edge and GitOps resources with the { id } identity, run through
// testFWIdentity and testFWList (fw_list_identity_test.go).

var fwEdgeStackCase = fwIdentityCase{
	typ:   "portainer_edge_stack",
	setup: func(m *MockServer) { newFakeEdgeStacks(m, 1) },
	config: `resource "portainer_edge_stack" "r" {
  name               = "web"
  deployment_type    = 1
  edge_groups        = [1]
  stack_file_content = "kind: List"
}`,
	display: "web",
	values: map[string]knownvalue.Check{
		"id":              knownvalue.StringExact("1"),
		"name":            knownvalue.StringExact("web"),
		"deployment_type": knownvalue.Int64Exact(1),
	},
	filter:        `name = "web"`,
	miss:          `name = "db"`,
	importChanges: true, // the stack file is not read back
}

func TestEdgeStackResource_Identity(t *testing.T) {
	t.Parallel()
	testFWIdentity(t, fwEdgeStackCase)
}

func TestEdgeStackResource_List(t *testing.T) {
	t.Parallel()
	testFWList(t, fwEdgeStackCase)
}

var fwEdgeJobCase = fwIdentityCase{
	typ:   "portainer_edge_job",
	setup: func(m *MockServer) { newFakeEdgeJobs(m, 1) },
	config: `resource "portainer_edge_job" "r" {
  name            = "cleanup"
  cron_expression = "0 * * * *"
  edge_groups     = [1]
  endpoints       = []
  recurring       = true
  file_content    = "true"
}`,
	display: "cleanup",
	values: map[string]knownvalue.Check{
		"id":              knownvalue.StringExact("1"),
		"cron_expression": knownvalue.StringExact("0 * * * *"),
		"recurring":       knownvalue.Bool(true),
		"file_content":    knownvalue.Null(),
	},
	filter:        `name = "cleanup"`,
	miss:          `name = "other"`,
	importChanges: true, // the script is not read back
}

func TestEdgeJobResource_Identity(t *testing.T) {
	t.Parallel()
	testFWIdentity(t, fwEdgeJobCase)
}

func TestEdgeJobResource_List(t *testing.T) {
	t.Parallel()
	testFWList(t, fwEdgeJobCase)
}

func fwEdgeConfigurationsCase(t *testing.T) fwIdentityCase {
	fp := writeEdgeConfigFile(t, "a=1")
	return fwIdentityCase{
		typ:     "portainer_edge_configurations",
		setup:   func(m *MockServer) { newFakeEdgeConfigurations(m, 7) },
		config:  strings.Replace(edgeConfigurationHCL(fp, "general", "[1]"), `"c"`, `"r"`, 1),
		display: "cfg",
		values: map[string]knownvalue.Check{
			"id":        knownvalue.StringExact("7"),
			"name":      knownvalue.StringExact("cfg"),
			"type":      knownvalue.StringExact("general"),
			"file_path": knownvalue.Null(),
		},
		filter:        `name = "cfg"`,
		miss:          `name = "other"`,
		importChanges: true, // the file is not read back
	}
}

func TestEdgeConfigurationsResource_Identity(t *testing.T) {
	t.Parallel()
	testFWIdentity(t, fwEdgeConfigurationsCase(t))
}

func TestEdgeConfigurationsResource_List(t *testing.T) {
	t.Parallel()
	testFWList(t, fwEdgeConfigurationsCase(t))
}

// serveGitopsSourceList answers GET /gitops/sources from the fake.
func serveGitopsSourceList(m *MockServer, f *fakeGitopsSources) {
	m.On("GET", "/gitops/sources", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		list := make([]map[string]interface{}, 0, len(f.sources))
		for _, s := range f.sources {
			list = append(list, map[string]interface{}{"id": s.ID, "name": s.Name, "url": s.URL})
		}
		RespondJSON(http.StatusOK, list)(w, r)
	})
}

var fwGitopsSourceCase = fwIdentityCase{
	typ: "portainer_gitops_source",
	setup: func(m *MockServer) {
		serveGitopsSourceList(m, newFakeGitopsSources(m))
	},
	config: `resource "portainer_gitops_source" "r" {
  url      = "https://github.com/acme/infra.git"
  name     = "infra"
  username = "ci"
  password = "token"
}`,
	display: "infra",
	values: map[string]knownvalue.Check{
		"id":                  knownvalue.StringExact("12"),
		"url":                 knownvalue.StringExact("https://github.com/acme/infra"),
		"username":            knownvalue.StringExact("ci"),
		"password":            knownvalue.Null(),
		"administrators_only": knownvalue.Bool(false),
	},
	filter:        `name = "infra"`,
	miss:          `name = "other"`,
	importChanges: true, // the password is never read back
}

func TestGitopsSourceResource_Identity(t *testing.T) {
	t.Parallel()
	testFWIdentity(t, fwGitopsSourceCase)
}

func TestGitopsSourceResource_List(t *testing.T) {
	t.Parallel()
	testFWList(t, fwGitopsSourceCase)
}

var fwGitopsWorkflowCase = fwIdentityCase{
	typ: "portainer_gitops_workflow",
	setup: func(m *MockServer) {
		f := newFakeGitopsWorkflows(m)
		m.On("GET", "/gitops/workflows", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			list := make([]json.RawMessage, 0, len(f.workflows))
			for _, wf := range f.workflows {
				raw, _ := json.Marshal(map[string]interface{}{"id": wf.ID, "name": wf.Name})
				list = append(list, raw)
			}
			RespondJSON(http.StatusOK, list)(w, r)
		})
	},
	config: `resource "portainer_gitops_workflow" "r" {
  name = "platform"
  artifact {
    name            = "web"
    deployment_type = "kubernetes"
    edge_group_ids  = [1]
    file {
      source_id = 3
      path      = "k8s"
      ref       = "refs/heads/main"
    }
  }
}`,
	display: "platform",
	values: map[string]knownvalue.Check{
		"id":   knownvalue.StringExact("1"),
		"name": knownvalue.StringExact("platform"),
	},
	filter:        `name = "platform"`,
	miss:          `name = "other"`,
	importChanges: true, // the artifact settings are not read back
}

func TestGitopsWorkflowResource_Identity(t *testing.T) {
	t.Parallel()
	testFWIdentity(t, fwGitopsWorkflowCase)
}

func TestGitopsWorkflowResource_List(t *testing.T) {
	t.Parallel()
	testFWList(t, fwGitopsWorkflowCase)
}
