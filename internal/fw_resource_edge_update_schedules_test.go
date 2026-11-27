package internal

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strconv"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// fakeEdgeUpdateSchedules is a stateful stand-in for Portainer's edge update
// schedule API; an update is a POST to the schedule's URL.
type fakeEdgeUpdateSchedules struct {
	mu        sync.Mutex
	nextID    int
	schedules map[int]map[string]interface{}
}

func newFakeEdgeUpdateSchedules(m *MockServer, ids ...int) *fakeEdgeUpdateSchedules {
	f := &fakeEdgeUpdateSchedules{schedules: map[int]map[string]interface{}{}}
	toStored := func(id int, body map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{
			"id": id, "name": body["name"], "agentImage": body["agentImage"], "updaterImage": body["updaterImage"],
			"registryId": body["registryID"], "scheduledTime": body["scheduledTime"], "edgeGroupIds": body["groupIDs"], "type": body["type"],
		}
	}
	m.On("POST", "/edge_update_schedules", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.nextID++
		f.schedules[f.nextID] = toStored(f.nextID, body)
		RespondJSON(http.StatusOK, f.schedules[f.nextID])(w, r)
	})
	for _, id := range ids {
		id := id
		p := "/edge_update_schedules/" + strconv.Itoa(id)
		m.On("GET", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			s, ok := f.schedules[id]
			if !ok {
				RespondString(http.StatusNotFound, "", "not found")(w, r)
				return
			}
			RespondJSON(http.StatusOK, s)(w, r)
		})
		m.On("POST", p, func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			f.schedules[id] = toStored(id, body)
			RespondJSON(http.StatusOK, f.schedules[id])(w, r)
		})
		m.On("DELETE", p, func(w http.ResponseWriter, _ *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			delete(f.schedules, id)
			w.WriteHeader(http.StatusNoContent)
		})
	}
	return f
}

func edgeUpdateScheduleHCL(agent string, groups string) string {
	return `resource "portainer_edge_update_schedules" "s" {
  name           = "fleet-update"
  agent_image    = "` + agent + `"
  updater_image  = "portainer/updater:latest"
  registry_id    = 3
  scheduled_time = "2026-01-01T00:00:00Z"
  group_ids      = ` + groups + `
  type           = 0
}`
}

func TestEdgeUpdateSchedulesResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEdgeUpdateSchedules(mock, 1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + edgeUpdateScheduleHCL("portainer/agent:2.20.0", "[1, 2]"),
				Check:  resource.TestCheckResourceAttr("portainer_edge_update_schedules.s", "id", "1"),
			},
			{
				Config: mockProviderConfig(mock) + edgeUpdateScheduleHCL("portainer/agent:2.21.0", "[2]"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_edge_update_schedules.s", plancheck.ResourceActionUpdate)},
				},
				Check: resource.TestCheckResourceAttr("portainer_edge_update_schedules.s", "group_ids.#", "1"),
			},
			{ResourceName: "portainer_edge_update_schedules.s", ImportState: true, ImportStateVerify: true},
		},
	})

	var create map[string]interface{}
	if err := mock.FindRequest("POST", "/edge_update_schedules").DecodeJSON(&create); err != nil {
		t.Fatal(err)
	}
	if create["name"] != "fleet-update" || create["agentImage"] != "portainer/agent:2.20.0" || create["registryID"] != float64(3) || create["type"] != float64(0) {
		t.Errorf("unexpected create payload %v", create)
	}
	if mock.FindRequest("POST", "/edge_update_schedules/1") == nil {
		t.Error("an update is a POST to the schedule's URL")
	}
}

func TestEdgeUpdateSchedulesResource_Errors(t *testing.T) {
	t.Parallel()
	t.Run("create", func(t *testing.T) {
		mock := NewMockServer(t)
		mock.On("POST", "/edge_update_schedules", RespondString(http.StatusBadRequest, "", "bad schedule"))
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{{
				Config:      mockProviderConfig(mock) + edgeUpdateScheduleHCL("a", "[1]"),
				ExpectError: regexp.MustCompile(`failed to create edge update schedule`),
			}},
		})
	})
	t.Run("type out of range", func(t *testing.T) {
		mock := NewMockServer(t)
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{{
				Config:      mockProviderConfig(mock) + regexp.MustCompile(`type += 0`).ReplaceAllString(edgeUpdateScheduleHCL("a", "[1]"), "type = 2"),
				ExpectError: regexp.MustCompile(`between 0 and 1`),
			}},
		})
	})
	t.Run("gone and delete 404", func(t *testing.T) {
		mock := NewMockServer(t)
		fake := newFakeEdgeUpdateSchedules(mock, 1)
		config := mockProviderConfig(mock) + edgeUpdateScheduleHCL("a", "[1]")
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{
				{Config: config},
				{
					PreConfig: func() {
						fake.mu.Lock()
						delete(fake.schedules, 1)
						fake.mu.Unlock()
					},
					Config:             config,
					PlanOnly:           true,
					ExpectNonEmptyPlan: true,
				},
			},
		})
	})
}

func TestEdgeUpdateSchedules_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEdgeUpdateSchedules(mock, 1)
	config := mockProviderConfig(mock) + edgeUpdateScheduleHCL("portainer/agent:2.20.0", "[1, 2]")
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

// TestAccEdgeUpdateSchedulesResource is not run: a schedule needs edge
// environments and is Business Edition only.
func TestAccEdgeUpdateSchedulesResource(t *testing.T) {
	t.Skip("edge update schedules need Business Edition and edge environments (PORTAINER_ACC_BE=1 and an edge fleet), which the acceptance setup does not provide")
}
