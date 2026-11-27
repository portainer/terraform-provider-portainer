package internal

import (
	"encoding/json"
	"net/http"
	"os"
	"strconv"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// fakeD2K is a stateful stand-in for POST /endpoints/d2k and the
// /endpoints/{id} calls a D2K environment is managed through.
type fakeD2K struct {
	mu     sync.Mutex
	nextID int
	envs   map[int]map[string]interface{}
	// instanceError makes the next install report an error for its instance
	// while still creating the environment.
	instanceError string
	lastCreate    map[string]interface{}
	lastUpdate    map[string]interface{}
}

func newFakeD2K(m *MockServer, ids ...int) *fakeD2K {
	f := &fakeD2K{nextID: 11, envs: map[int]map[string]interface{}{}}
	m.On("POST", "/endpoints/d2k", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			ParentEnvironmentID int
			Instances           []struct{ Name, EnvironmentName string }
			SwarmMode           bool
			GroupID             int
			TagIDs              []int
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.lastCreate = map[string]interface{}{"body": body}
		results := make([]map[string]interface{}, 0, len(body.Instances))
		for _, in := range body.Instances {
			f.nextID++
			envName := in.EnvironmentName
			if envName == "" {
				envName = in.Name
			}
			tags := body.TagIDs
			if tags == nil {
				tags = []int{}
			}
			f.envs[f.nextID] = map[string]interface{}{
				"Id": f.nextID, "Name": envName, "GroupId": body.GroupID, "TagIds": tags,
				"D2K": map[string]interface{}{"Namespace": d2kNamespacePrefix + in.Name, "ParentEnvironmentID": body.ParentEnvironmentID, "SwarmMode": body.SwarmMode},
			}
			results = append(results, map[string]interface{}{
				"Name": in.Name, "Namespace": d2kNamespacePrefix + in.Name, "Error": f.instanceError,
				"Environment": map[string]interface{}{"Id": f.nextID},
			})
		}
		RespondJSON(http.StatusOK, results)(w, r)
	})
	for _, id := range ids {
		id := id
		p := "/endpoints/" + strconv.Itoa(id)
		m.On("GET", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			env, ok := f.envs[id]
			if !ok {
				RespondString(http.StatusNotFound, "", "not found")(w, r)
				return
			}
			RespondJSON(http.StatusOK, env)(w, r)
		})
		m.On("PUT", p, func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			f.lastUpdate = body
			env := f.envs[id]
			if v, ok := body["name"]; ok {
				env["Name"] = v
			}
			if v, ok := body["groupID"]; ok {
				env["GroupId"] = v
			}
			if v, ok := body["tagIDs"]; ok && v != nil {
				env["TagIds"] = v
			}
			RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
		})
		m.On("DELETE", p, func(w http.ResponseWriter, _ *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			delete(f.envs, id)
			w.WriteHeader(http.StatusNoContent)
		})
	}
	return f
}

func TestD2KEnvironmentResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeD2K(mock, 12, 13)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `resource "portainer_d2k_environment" "d" {
  parent_environment_id = 3
  name                  = "team-a"
  swarm_mode            = true
  group_id              = 2
  tag_ids               = [4, 5]
}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_d2k_environment.d", "id", "12"),
					resource.TestCheckResourceAttr("portainer_d2k_environment.d", "environment_id", "12"),
					resource.TestCheckResourceAttr("portainer_d2k_environment.d", "namespace", "d2k-team-a"),
					resource.TestCheckResourceAttr("portainer_d2k_environment.d", "environment_name", "team-a"),
				),
			},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_d2k_environment" "d" {
  parent_environment_id = 3
  name                  = "team-a"
  environment_name      = "Renamed"
  swarm_mode            = true
  group_id              = 2
  tag_ids               = [4, 5]
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_d2k_environment.d", plancheck.ResourceActionUpdate)},
				},
				Check: resource.TestCheckResourceAttr("portainer_d2k_environment.d", "environment_name", "Renamed"),
			},
			{ResourceName: "portainer_d2k_environment.d", ImportState: true, ImportStateVerify: true},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_d2k_environment" "d" {
  parent_environment_id = 3
  name                  = "team-a"
  swarm_mode            = false
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_d2k_environment.d", plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: resource.TestCheckResourceAttr("portainer_d2k_environment.d", "id", "13"),
			},
		},
	})

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.lastUpdate["name"] != "Renamed" || fake.lastUpdate["groupID"] != float64(2) {
		t.Errorf("update body: got %v", fake.lastUpdate)
	}
	if tags, _ := fake.lastUpdate["tagIDs"].([]interface{}); len(tags) != 2 {
		t.Errorf("tagIDs: got %v", fake.lastUpdate["tagIDs"])
	}
	if _, ok := fake.lastUpdate["url"]; ok {
		t.Errorf("a D2K update must not send a URL, got %v", fake.lastUpdate)
	}
}

// TestD2KEnvironmentResource_InstanceErrorTaints: an environment created
// despite an instance error stays in state, tainted, so the next apply
// replaces it.
func TestD2KEnvironmentResource_InstanceErrorTaints(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeD2K(mock, 12, 13)
	fake.instanceError = "helm install failed"
	config := mockProviderConfig(mock) + `resource "portainer_d2k_environment" "d" {
  parent_environment_id = 3
  name                  = "team-a"
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config, ExpectError: edgeErrRe(`failed to create D2K environment "team-a": helm install failed`)},
			{
				PreConfig: func() { fake.instanceError = "" },
				Config:    config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_d2k_environment.d", plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: resource.TestCheckResourceAttr("portainer_d2k_environment.d", "id", "13"),
			},
		},
	})
}

func TestD2KEnvironmentResource_Errors(t *testing.T) {
	t.Parallel()
	config := `resource "portainer_d2k_environment" "d" {
  parent_environment_id = 3
  name                  = "team-a"
}`
	cases := map[string]struct {
		setup func(*MockServer)
		want  string
	}{
		"no result": {
			setup: func(m *MockServer) {
				m.On("POST", "/endpoints/d2k", RespondJSON(http.StatusOK, []interface{}{}))
			},
			want: `Portainer reported no result for it`,
		},
		"no environment": {
			setup: func(m *MockServer) {
				m.On("POST", "/endpoints/d2k", RespondJSON(http.StatusOK, []map[string]interface{}{{"Name": "team-a"}}))
			},
			want: `Portainer returned no environment`,
		},
		"unsupported server": {
			setup: func(m *MockServer) {
				m.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{"ServerVersion": "2.45.0", "ServerEdition": "CE"}))
			},
			want: `portainer_d2k_environment requires Portainer Business Edition 3.0 or later`,
		},
		"not a D2K environment": {
			setup: func(m *MockServer) {
				m.On("POST", "/endpoints/d2k", RespondJSON(http.StatusOK, []map[string]interface{}{{"Name": "team-a", "Environment": map[string]interface{}{"Id": 12}}}))
				m.On("GET", "/endpoints/12", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 12, "Name": "plain"}))
				m.On("DELETE", "/endpoints/12", RespondString(http.StatusNoContent, "", ""))
			},
			want: `environment 12 is not a D2K environment`,
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			tc.setup(mock)
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{{
					Config:      mockProviderConfig(mock) + config,
					ExpectError: edgeErrRe(tc.want),
				}},
			})
		})
	}
}

func TestD2KEnvironmentResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeD2K(mock, 12)
	config := mockProviderConfig(mock) + `resource "portainer_d2k_environment" "d" {
  parent_environment_id = 3
  name                  = "team-a"
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					delete(fake.envs, 12)
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestD2KEnvironment_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeD2K(mock, 12, 13)
	config := mockProviderConfig(mock) + `
resource "portainer_d2k_environment" "bare" {
  parent_environment_id = 3
  name                  = "bare"
}
resource "portainer_d2k_environment" "full" {
  parent_environment_id = 3
  name                  = "full"
  environment_name      = "Full"
  swarm_mode            = true
  group_id              = 2
  tag_ids               = [4, 5]
}`
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				ConfigPlanChecks:         resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

func TestAccD2KEnvironmentResource(t *testing.T) {
	if os.Getenv("PORTAINER_ACC_BE") != "1" || os.Getenv("PORTAINER_ACC_K8S_ENV_ID") == "" {
		t.Skip("D2K needs Portainer Business Edition 3.0 (PORTAINER_ACC_BE=1) and a Kubernetes Edge Agent environment (PORTAINER_ACC_K8S_ENV_ID)")
	}
	accPreCheck(t)
	config := accProvider() + `resource "portainer_d2k_environment" "d" {
  parent_environment_id = ` + os.Getenv("PORTAINER_ACC_K8S_ENV_ID") + `
  name                  = "acc-d2k"
}`
	resource.Test(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{ResourceName: "portainer_d2k_environment.d", ImportState: true, ImportStateVerify: true},
		},
	})
}
