package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakeResourceControls is a stateful stand-in for Portainer's resource
// controls: POST/PUT/DELETE /resource_controls and a stack (GET /stacks/{id})
// carrying its control.
type fakeResourceControls struct {
	mu       sync.Mutex
	nextID   int
	controls map[int]map[string]interface{}
	stacks   map[string]int // stack ID -> control ID
}

func newFakeResourceControls(m *MockServer) *fakeResourceControls {
	f := &fakeResourceControls{nextID: 10, controls: map[int]map[string]interface{}{}, stacks: map[string]int{}}
	m.On("POST", "/resource_controls", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.nextID++
		f.controls[f.nextID] = f.control(f.nextID, body["Type"], body["ResourceID"], body["AdministratorsOnly"], body["Public"], body["Teams"], body["Users"])
		RespondJSON(http.StatusOK, map[string]interface{}{"Id": f.nextID})(w, r)
	})
	for id := 1; id <= 20; id++ {
		id := id
		p := fmt.Sprintf("/resource_controls/%d", id)
		m.On("PUT", p, func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			rc, ok := f.controls[id]
			if !ok {
				RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`)(w, r)
				return
			}
			f.controls[id] = f.control(id, rc["Type"], rc["ResourceId"], body["administratorsOnly"], body["public"], body["teams"], body["users"])
			RespondJSON(http.StatusOK, f.controls[id])(w, r)
		})
		m.On("DELETE", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if _, ok := f.controls[id]; !ok {
				RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`)(w, r)
				return
			}
			delete(f.controls, id)
			w.WriteHeader(http.StatusNoContent)
		})
	}
	for _, sid := range []string{"5", "6"} {
		sid := sid
		m.On("GET", "/stacks/"+sid, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			body := map[string]interface{}{"Id": sid}
			if rcID, ok := f.stacks[sid]; ok {
				if rc, ok := f.controls[rcID]; ok {
					body["ResourceControl"] = rc
				}
			}
			RespondJSON(http.StatusOK, body)(w, r)
		})
	}
	return f
}

func (f *fakeResourceControls) control(id int, typ, resourceID, admins, public, teams, users interface{}) map[string]interface{} {
	accesses := func(raw interface{}, key string) []map[string]interface{} {
		list, _ := raw.([]interface{})
		out := make([]map[string]interface{}, 0, len(list))
		for _, v := range list {
			out = append(out, map[string]interface{}{key: v})
		}
		return out
	}
	return map[string]interface{}{
		"Id": id, "Type": typ, "ResourceId": resourceID,
		"AdministratorsOnly": admins, "Public": public,
		"TeamAccesses": accesses(teams, "TeamId"), "UserAccesses": accesses(users, "UserId"),
	}
}

// stackControl gives stack sid an existing control, the way Portainer creates
// one for every stack it deploys.
func (f *fakeResourceControls) stackControl(sid string, id int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.controls[id] = f.control(id, float64(6), sid, true, false, []interface{}{}, []interface{}{})
	f.stacks[sid] = id
}

// TestResourceControlResource_StackLookup: the control Portainer created for a
// stack is updated, not duplicated; teams and users changed in Portainer show
// as drift; destroy deletes the control found by lookup.
func TestResourceControlResource_StackLookup(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeResourceControls(mock)
	fake.stackControl("5", 3)
	config := mockProviderConfig(mock) + `
resource "portainer_resource_control" "rc" {
  resource_id = "5"
  teams       = [7]
}
`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(*terraform.State) error {
			if mock.FindRequest("DELETE", "/resource_controls/3") == nil {
				return fmt.Errorf("expected DELETE /resource_controls/3")
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_resource_control.rc", "id", "3"),
					resource.TestCheckResourceAttr("portainer_resource_control.rc", "type", "6"),
					resource.TestCheckNoResourceAttr("portainer_resource_control.rc", "users"),
					func(*terraform.State) error {
						if mock.FindRequest("POST", "/resource_controls") != nil {
							return fmt.Errorf("an existing control must not be created again")
						}
						p := lastRequestBody(t, mock, "PUT", "/resource_controls/3")
						if p["administratorsOnly"] != false || p["public"] != false || fmt.Sprint(p["teams"]) != "[7]" || fmt.Sprint(p["users"]) != "[]" {
							return fmt.Errorf("update payload: %v", p)
						}
						return nil
					},
				),
			},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.controls[3]["TeamAccesses"] = []map[string]interface{}{{"TeamId": 8}}
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestResourceControlResource_DirectID: an explicit resource_control_id is
// updated without any lookup, and Read calls no API.
func TestResourceControlResource_DirectID(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeResourceControls(mock)
	fake.stackControl("6", 12)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_resource_control" "rc" {
  resource_control_id = 12
  public              = true
}
`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_resource_control.rc", "id", "12"),
				func(*terraform.State) error {
					for _, r := range mock.Requests() {
						if r.Method == http.MethodGet {
							return fmt.Errorf("no lookup expected with resource_control_id, got GET %s", r.Path)
						}
					}
					if p := lastRequestBody(t, mock, "PUT", "/resource_controls/12"); p["public"] != true {
						return fmt.Errorf("update payload: %v", p)
					}
					return nil
				},
			),
		}},
	})
}

// TestResourceControlResource_PostsWhenNoneExists: a resource without a
// control gets one; one Portainer cannot read back stays as configured.
func TestResourceControlResource_PostsWhenNoneExists(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeResourceControls(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `
resource "portainer_resource_control" "rc" {
  resource_id         = "abc"
  type                = 1
  administrators_only = true
  sub_resource_ids    = ["x", "y"]
}
`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_resource_control.rc", "id", "11"),
					func(*terraform.State) error {
						p := lastRequestBody(t, mock, "POST", "/resource_controls")
						if p["ResourceID"] != "abc" || p["Type"] != float64(1) || p["AdministratorsOnly"] != true || fmt.Sprint(p["SubResourceIDs"]) != "[x y]" || fmt.Sprint(p["Teams"]) != "[]" {
							return fmt.Errorf("create payload: %v", p)
						}
						return nil
					},
				),
			},
			{
				Config: mockProviderConfig(mock) + `
resource "portainer_resource_control" "rc" {
  resource_id         = "abc"
  type                = 1
  administrators_only = true
  sub_resource_ids    = ["x", "y"]
}
`,
				PlanOnly: true,
			},
		},
	})
}

// TestResourceControlResource_ReadErrors: a failing lookup is reported and
// keeps the state; a missing stack or a stack without control drops it.
func TestResourceControlResource_ReadErrors(t *testing.T) {
	t.Parallel()
	cfg := func(m *MockServer) string {
		return mockProviderConfig(m) + `resource "portainer_resource_control" "rc" { resource_id = "5" }`
	}
	t.Run("server error", func(t *testing.T) {
		mock := NewMockServer(t)
		var broken bool
		var mu sync.Mutex
		mock.On("GET", "/stacks/5", func(w http.ResponseWriter, r *http.Request) {
			mu.Lock()
			defer mu.Unlock()
			if broken {
				RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`)(w, r)
				return
			}
			RespondJSON(http.StatusOK, map[string]interface{}{"ResourceControl": map[string]interface{}{"Id": 3, "Type": 6, "ResourceId": "5"}})(w, r)
		})
		mock.On("PUT", "/resource_controls/3", RespondJSON(http.StatusOK, map[string]interface{}{}))
		mock.On("DELETE", "/resource_controls/3", RespondString(http.StatusNoContent, "", ""))
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{
				{Config: cfg(mock)},
				{
					PreConfig:   func() { mu.Lock(); broken = true; mu.Unlock() },
					Config:      cfg(mock),
					PlanOnly:    true,
					ExpectError: regexp.MustCompile(`failed to read resource control`),
				},
				{PreConfig: func() { mu.Lock(); broken = false; mu.Unlock() }, Config: cfg(mock), PlanOnly: true},
			},
		})
	})
	for name, handler := range map[string]http.HandlerFunc{
		"stack gone":            RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`),
		"stack without control": RespondJSON(http.StatusOK, map[string]interface{}{"Id": 5}),
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			fake := newFakeResourceControls(mock)
			fake.stackControl("6", 3)
			var gone bool
			var mu sync.Mutex
			mock.mu.Lock()
			mock.routes = append([]route{{method: "GET", path: "/stacks/5", handler: func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				g := gone
				mu.Unlock()
				if g {
					handler(w, r)
					return
				}
				RespondJSON(http.StatusOK, map[string]interface{}{"ResourceControl": map[string]interface{}{"Id": 3, "Type": 6, "ResourceId": "5"}})(w, r)
			}}}, mock.routes...)
			mock.mu.Unlock()
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{
					{Config: cfg(mock)},
					{
						PreConfig:          func() { mu.Lock(); gone = true; mu.Unlock() },
						Config:             cfg(mock),
						PlanOnly:           true,
						ExpectNonEmptyPlan: true,
					},
				},
			})
		})
	}
}

// TestResourceControlResource_Delete: 403 and 404 count as gone, another
// failure is reported.
func TestResourceControlResource_Delete(t *testing.T) {
	t.Parallel()
	for status, wantErr := range map[int]bool{http.StatusForbidden: false, http.StatusNotFound: false, http.StatusInternalServerError: true} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			mock := NewMockServer(t)
			failing := true
			var mu sync.Mutex
			mock.On("PUT", "/resource_controls/12", RespondJSON(http.StatusOK, map[string]interface{}{}))
			mock.On("DELETE", "/resource_controls/12", func(w http.ResponseWriter, r *http.Request) {
				mu.Lock()
				defer mu.Unlock()
				if failing {
					RespondString(status, "application/json", `{"message":"nope"}`)(w, r)
					return
				}
				w.WriteHeader(http.StatusNoContent)
			})
			config := mockProviderConfig(mock) + `resource "portainer_resource_control" "rc" { resource_control_id = 12 }`
			steps := []resource.TestStep{{Config: config}}
			if wantErr {
				steps = append(steps,
					resource.TestStep{Config: config, Destroy: true, ExpectError: regexp.MustCompile(`failed to delete resource control`)},
					resource.TestStep{Config: config, PreConfig: func() { mu.Lock(); failing = false; mu.Unlock() }},
				)
			}
			resource.UnitTest(t, resource.TestCase{ProtoV5ProviderFactories: testProtoV5Factories, Steps: steps})
		})
	}
}

func TestResourceControlResource_RequiresResourceID(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_resource_control" "rc" { public = true }`,
			ExpectError: regexp.MustCompile(`resource_id is required to create a resource control`),
		}},
	})
}

func TestResourceControlResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for name, cfg := range map[string]string{
		"stack": `resource "portainer_resource_control" "rc" { resource_id = "5" }`,
		"stack-all": `resource "portainer_resource_control" "rc" {
  resource_id         = "5"
  type                = 6
  administrators_only = false
  public              = true
  teams               = [1, 2]
  users               = [3]
}`,
		"direct": `resource "portainer_resource_control" "rc" { resource_control_id = 12 }`,
		"created": `resource "portainer_resource_control" "rc" {
  resource_id      = "abc"
  type             = 1
  sub_resource_ids = ["x"]
}`,
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			fake := newFakeResourceControls(mock)
			fake.stackControl("5", 3)
			fake.stackControl("6", 12)
			config := mockProviderConfig(mock) + cfg
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
		})
	}
}

// --- lookupResourceControl: the lookups behind Read, Update and Delete ---

func TestLookupResourceControl_Unsupported(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	for _, tc := range []struct {
		typ      int
		resource string
	}{{1, "abc"}, {9, "1"}, {6, "4_"}} {
		_, _, err := lookupResourceControl(context.Background(), mock.Client(), tc.typ, tc.resource, 0)
		if !errors.Is(err, errResourceControlNotLookupable) {
			t.Errorf("type %d %q: expected not lookupable, got %v", tc.typ, tc.resource, err)
		}
	}
	if len(mock.Requests()) != 0 {
		t.Error("nothing to look up must not call the API")
	}
}

func TestLookupResourceControl_ExternalStack(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		containers []interface{}
		wantID     string
		wantErr    error
	}{
		"own control": {[]interface{}{map[string]interface{}{"Id": "c1", "Portainer": map[string]interface{}{"ResourceControl": map[string]interface{}{
			"Id": 17, "Type": 6, "ResourceId": "4_server",
		}}}}, "17", nil},
		"no containers": {[]interface{}{}, "", errResourceControlNotLookupable},
		"shadowed by the container's own control": {[]interface{}{map[string]interface{}{"Id": "c1", "Portainer": map[string]interface{}{"ResourceControl": map[string]interface{}{"Id": 20, "Type": 1, "ResourceId": "c1"}}}}, "", errResourceControlNotLookupable},
		"container shows no control":              {[]interface{}{map[string]interface{}{"Id": "c1"}}, "", errResourceControlNotFound},
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("GET", "/endpoints/4/docker/containers/json", RespondJSON(http.StatusOK, tc.containers))
			id, _, err := lookupResourceControl(context.Background(), mock.Client(), 6, "4_server", 0)
			if id != tc.wantID || (tc.wantErr == nil) != (err == nil) || (tc.wantErr != nil && !errors.Is(err, tc.wantErr)) {
				t.Errorf("got %q, %v; want %q, %v", id, err, tc.wantID, tc.wantErr)
			}
			if mock.FindRequest("GET", "/stacks/4_server") != nil {
				t.Error("an external stack must not be looked up through GET /stacks/{id}")
			}
		})
	}
}

func TestLookupResourceControl_DockerTypes(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, resourceID, path string
		resourceType           int
		object                 map[string]interface{}
		rc                     map[string]interface{}
		wantID                 string
	}{
		{"own control", "abc", "/endpoints/3/docker/containers/abc/json", 1, map[string]interface{}{"Id": "abc"}, map[string]interface{}{"Id": 21, "Type": 1, "ResourceId": "abc", "AdministratorsOnly": true}, "21"},
		{"only an inherited stack control", "abc", "/endpoints/3/docker/containers/abc/json", 1, map[string]interface{}{"Id": "abc"}, map[string]interface{}{"Id": 5, "Type": 6, "ResourceId": "3_app"}, ""},
		{"no control", "abc", "/endpoints/3/docker/containers/abc/json", 1, map[string]interface{}{"Id": "abc"}, nil, ""},
		{"volume by name", "data", "/endpoints/3/docker/volumes/data", 3, map[string]interface{}{"Name": "data", "ResourceID": "data_dockerid"}, map[string]interface{}{"Id": 21, "Type": 3, "ResourceId": "data_dockerid"}, "21"},
		{"volume by Portainer's key", "data_dockerid", "/endpoints/3/docker/volumes/data", 3, map[string]interface{}{"Name": "data", "ResourceID": "data_dockerid"}, map[string]interface{}{"Id": 21, "Type": 3, "ResourceId": "data_dockerid"}, "21"},
		{"container by name", "web", "/endpoints/3/docker/containers/web/json", 1, map[string]interface{}{"Id": "abcdef0123", "Name": "/web"}, map[string]interface{}{"Id": 21, "Type": 1, "ResourceId": "abcdef0123"}, "21"},
		{"service", "svc", "/endpoints/3/docker/services/svc", 2, map[string]interface{}{"ID": "svcfullid"}, map[string]interface{}{"Id": 22, "Type": 2, "ResourceId": "svcfullid"}, "22"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockServer(t)
			if tc.rc != nil {
				tc.object["Portainer"] = map[string]interface{}{"ResourceControl": tc.rc}
			}
			mock.On("GET", tc.path, RespondJSON(http.StatusOK, tc.object))
			id, _, err := lookupResourceControl(context.Background(), mock.Client(), tc.resourceType, tc.resourceID, 3)
			if id != tc.wantID {
				t.Errorf("ID: expected %q, got %q (%v)", tc.wantID, id, err)
			}
			if tc.wantID == "" && !errors.Is(err, errResourceControlNotFound) {
				t.Errorf("expected not found, got %v", err)
			}
		})
	}
	// Without the environment a Docker object cannot be read.
	if _, _, err := lookupResourceControl(context.Background(), NewMockServer(t).Client(), 1, "abc", 0); !errors.Is(err, errResourceControlNotLookupable) {
		t.Errorf("expected not lookupable without endpoint_id, got %v", err)
	}
}

func TestLookupResourceControl_Stack(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/stacks/88", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 88}))
	mock.On("GET", "/stacks/89", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 89, "ResourceControl": map[string]interface{}{"Id": 4}}))
	if _, _, err := lookupResourceControl(context.Background(), mock.Client(), 6, "88", 0); !errors.Is(err, errResourceControlNotFound) {
		t.Errorf("a stack without control: expected not found, got %v", err)
	}
	if id, err := lookupResourceControlIDForTest(mock, "89"); id != "4" || err != nil {
		t.Errorf("expected control 4, got %q, %v", id, err)
	}
	if _, _, err := lookupResourceControl(context.Background(), mock.Client(), 6, "90", 0); !isAPINotFound(err) {
		t.Errorf("a missing stack: expected 404, got %v", err)
	}
}

func lookupResourceControlIDForTest(m *MockServer, stackID string) (string, error) {
	id, _, err := lookupResourceControlID(context.Background(), m.Client(), 6, stackID)
	return id, err
}

// TestAccResourceControlResource needs a Docker environment with a container
// (PORTAINER_ACC_SWARM_ENV_ID, PORTAINER_ACC_CONTAINER): the control of a
// Docker object is read back through Portainer's Docker proxy.
func TestAccResourceControlResource(t *testing.T) {
	envID, container := os.Getenv("PORTAINER_ACC_SWARM_ENV_ID"), os.Getenv("PORTAINER_ACC_CONTAINER")
	if envID == "" || container == "" {
		t.Skip("PORTAINER_ACC_SWARM_ENV_ID and PORTAINER_ACC_CONTAINER must name a Docker environment and a container on it")
	}
	config := fmt.Sprintf(`
resource "portainer_resource_control" "rc" {
  resource_id         = %q
  type                = 1
  endpoint_id         = %s
  administrators_only = true
}
`, container, envID)
	resource.Test(t, resource.TestCase{
		PreCheck: func() { accPreCheck(t) },
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: accProvider() + config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   accProvider() + config,
				ConfigPlanChecks:         resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}
