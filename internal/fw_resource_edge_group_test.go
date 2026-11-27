package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// fakeEdgeGroups is a stateful stand-in for Portainer's edge group API.
type fakeEdgeGroups struct {
	mu     sync.Mutex
	nextID int
	groups map[int]map[string]interface{}
	// nullLists makes the API answer null instead of [] for empty lists.
	nullLists bool
}

func newFakeEdgeGroups(m *MockServer, ids ...int) *fakeEdgeGroups {
	f := &fakeEdgeGroups{groups: map[int]map[string]interface{}{}}
	m.On("GET", "/edge_groups", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		list := make([]map[string]interface{}, 0, len(f.groups))
		for id, g := range f.groups {
			list = append(list, map[string]interface{}{"Id": id, "Name": g["Name"], "Dynamic": g["Dynamic"]})
		}
		RespondJSON(http.StatusOK, list)(w, r)
	})
	m.On("POST", "/edge_groups", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.nextID++
		f.groups[f.nextID] = f.fromPayload(body)
		RespondJSON(http.StatusOK, map[string]interface{}{"Id": f.nextID})(w, r)
	})
	for _, id := range ids {
		id := id
		p := "/edge_groups/" + strconv.Itoa(id)
		m.On("GET", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			g, ok := f.groups[id]
			if !ok {
				RespondString(http.StatusNotFound, "", "not found")(w, r)
				return
			}
			RespondJSON(http.StatusOK, g)(w, r)
		})
		m.On("PUT", p, func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			f.groups[id] = f.fromPayload(body)
			RespondJSON(http.StatusOK, map[string]interface{}{"Id": id})(w, r)
		})
		m.On("DELETE", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			delete(f.groups, id)
			w.WriteHeader(http.StatusNoContent)
		})
	}
	return f
}

func (f *fakeEdgeGroups) fromPayload(body map[string]interface{}) map[string]interface{} {
	var empty interface{} = []int{}
	if f.nullLists {
		empty = nil
	}
	g := map[string]interface{}{
		"Name": body["name"], "Dynamic": body["dynamic"], "PartialMatch": body["partialMatch"],
		"Endpoints": empty, "TagIds": empty,
	}
	if v, ok := body["endpoints"]; ok {
		g["Endpoints"] = v
	}
	if v, ok := body["tagIDs"]; ok {
		g["TagIds"] = v
	}
	return g
}

func TestEdgeGroupResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEdgeGroups(mock, 1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `resource "portainer_edge_group" "g" {
  name    = "production"
  dynamic = true
}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_edge_group.g", "id", "1"),
					resource.TestCheckResourceAttr("portainer_edge_group.g", "partial_match", "false"),
					resource.TestCheckNoResourceAttr("portainer_edge_group.g", "tag_ids"),
				),
			},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_edge_group" "g" {
  name          = "production"
  dynamic       = true
  partial_match = true
  tag_ids       = [3, 4]
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_edge_group.g", plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_edge_group.g", "tag_ids.#", "2"),
					resource.TestCheckResourceAttr("portainer_edge_group.g", "partial_match", "true"),
				),
			},
			{
				ResourceName:      "portainer_edge_group.g",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})

	put := mock.FindRequest("PUT", "/edge_groups/1")
	if put == nil {
		t.Fatal("expected PUT /edge_groups/1")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatal(err)
	}
	if payload["partialMatch"] != true || payload["tagIDs"] == nil {
		t.Errorf("unexpected update payload %v", payload)
	}
	if _, ok := payload["endpoints"]; ok {
		t.Errorf("endpoints must be left out when not configured, got %v", payload)
	}
}

// TestEdgeGroupResource_ExistingNameIsAdopted: a group of the same name is
// updated in place rather than created twice.
func TestEdgeGroupResource_ExistingNameIsAdopted(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeGroups(mock, 42)
	fake.groups[42] = map[string]interface{}{"Name": "production", "Dynamic": false, "PartialMatch": false, "Endpoints": []int{}, "TagIds": []int{}}

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_edge_group" "g" {
  name      = "production"
  dynamic   = false
  endpoints = [7]
}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_edge_group.g", "id", "42"),
				resource.TestCheckResourceAttr("portainer_edge_group.g", "endpoints.0", "7"),
			),
		}},
	})
	if mock.FindRequest("POST", "/edge_groups") != nil {
		t.Error("an existing group must not be created again")
	}
}

func TestEdgeGroupResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeGroups(mock, 1, 2)
	config := mockProviderConfig(mock) + `resource "portainer_edge_group" "g" {
  name    = "production"
  dynamic = true
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					delete(fake.groups, 1)
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestEdgeGroupResource_Errors(t *testing.T) {
	t.Parallel()
	t.Run("list error", func(t *testing.T) {
		mock := NewMockServer(t)
		mock.On("GET", "/edge_groups", RespondString(http.StatusInternalServerError, "", "boom"))
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{{
				Config:      mockProviderConfig(mock) + edgeGroupHCL("x"),
				ExpectError: regexp.MustCompile(`failed to check for existing edge group`),
			}},
		})
	})
	t.Run("create error", func(t *testing.T) {
		mock := NewMockServer(t)
		mock.On("GET", "/edge_groups", RespondJSON(http.StatusOK, []interface{}{}))
		mock.On("POST", "/edge_groups", RespondString(http.StatusBadRequest, "", "invalid"))
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{{
				Config:      mockProviderConfig(mock) + edgeGroupHCL("x"),
				ExpectError: regexp.MustCompile(`failed to create edge group`),
			}},
		})
	})
	t.Run("empty name", func(t *testing.T) {
		mock := NewMockServer(t)
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{{
				Config:      mockProviderConfig(mock) + edgeGroupHCL(""),
				ExpectError: regexp.MustCompile(`at least 1`),
			}},
		})
	})
	t.Run("delete error", func(t *testing.T) {
		mock := NewMockServer(t)
		newFakeEdgeGroups(mock)
		mock.On("GET", "/edge_groups/1", RespondJSON(http.StatusOK, map[string]interface{}{"Name": "x", "Dynamic": true}))
		blocked := true
		mock.On("DELETE", "/edge_groups/1", func(w http.ResponseWriter, r *http.Request) {
			if blocked {
				RespondString(http.StatusConflict, "", "edge stack still deployed")(w, r)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		})
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{
				{Config: mockProviderConfig(mock) + edgeGroupHCL("x")},
				{
					Config:      mockProviderConfig(mock),
					ExpectError: regexp.MustCompile(`edge stack still deployed`),
				},
				{
					// Let the post-test destroy succeed.
					PreConfig: func() { blocked = false },
					Config:    mockProviderConfig(mock),
				},
			},
		})
	})
}

// TestEdgeGroup_UpgradeFromV300: state written by v3.0.0 (SDKv2) against the
// same fake API plans no change with this build, with the optional lists set
// and left out, whether the API answers [] or null for an empty list.
func TestEdgeGroup_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for _, nullLists := range []bool{false, true} {
		t.Run("nullLists="+strconv.FormatBool(nullLists), func(t *testing.T) {
			mock := NewMockServer(t)
			fake := newFakeEdgeGroups(mock, 1, 2)
			fake.nullLists = nullLists

			config := mockProviderConfig(mock) + `
resource "portainer_edge_group" "bare" {
  name    = "bare"
  dynamic = true
}
resource "portainer_edge_group" "full" {
  name          = "full"
  dynamic       = false
  partial_match = true
  endpoints     = [3, 4]
  tag_ids       = []
}`
			resource.Test(t, resource.TestCase{
				IsUnitTest: true,
				Steps: []resource.TestStep{
					{ExternalProviders: previousRelease, Config: config},
					{
						ProtoV5ProviderFactories: testProtoV5Factories,
						Config:                   config,
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
					},
				},
			})
		})
	}
}

// TestAccEdgeGroupResource_UpgradeFromSDKv2 runs the upgrade against a real
// Portainer; edge groups need nothing beyond a CE instance. Portainer refuses
// a dynamic group without tags, so the test brings its own tag.
func TestAccEdgeGroupResource_UpgradeFromSDKv2(t *testing.T) {
	name := acctest.RandomWithPrefix("acc-edge-group")
	config := fmt.Sprintf(`resource "portainer_tag" "t" {
  name = %q
}

resource "portainer_edge_group" "g" {
  name    = %q
  dynamic = true
  tag_ids = [portainer_tag.t.id]
}`, name, name)
	resource.Test(t, resource.TestCase{
		PreCheck: func() { accPreCheck(t) },
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: accProvider() + config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   accProvider() + config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   accProvider() + config,
				ResourceName:             "portainer_edge_group.g",
				ImportState:              true,
				ImportStateVerify:        true,
			},
		},
	})
}

func edgeGroupHCL(name string) string {
	return fmt.Sprintf(`resource "portainer_edge_group" "g" {
  name    = %q
  dynamic = true
}`, name)
}
