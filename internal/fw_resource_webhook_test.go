package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakeWebhooks is a stateful stand-in for Portainer's webhook API: list (with
// the EndpointID/ResourceID filter), create, update of the registry, reassign
// and delete.
type fakeWebhooks struct {
	mu       sync.Mutex
	nextID   int
	webhooks map[int]*webhookRecord
}

func newFakeWebhooks(m *MockServer) *fakeWebhooks {
	f := &fakeWebhooks{webhooks: map[int]*webhookRecord{}}
	m.On("GET", "/webhooks", func(w http.ResponseWriter, r *http.Request) {
		var filter struct {
			EndpointID int
			ResourceID string
		}
		if v := r.URL.Query().Get("filters"); v != "" {
			_ = json.Unmarshal([]byte(v), &filter)
		}
		f.mu.Lock()
		defer f.mu.Unlock()
		list := []webhookRecord{}
		for id := 1; id <= f.nextID; id++ {
			wh, ok := f.webhooks[id]
			if !ok || (filter.EndpointID != 0 && (wh.EndpointID != filter.EndpointID || wh.ResourceID != filter.ResourceID)) {
				continue
			}
			list = append(list, *wh)
		}
		RespondJSON(http.StatusOK, list)(w, r)
	})
	m.On("POST", "/webhooks", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			EndpointID  int    `json:"endpointID"`
			RegistryID  int    `json:"registryID"`
			ResourceID  string `json:"resourceID"`
			WebhookType int    `json:"webhookType"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.nextID++
		wh := &webhookRecord{ID: f.nextID, Token: fmt.Sprintf("tok-%d", f.nextID), EndpointID: body.EndpointID, RegistryID: body.RegistryID, ResourceID: body.ResourceID, Type: body.WebhookType}
		f.webhooks[wh.ID] = wh
		RespondJSON(http.StatusOK, wh)(w, r)
	})
	for id := 1; id <= 10; id++ {
		id := id
		p := "/webhooks/" + strconv.Itoa(id)
		m.On("PUT", p, func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				RegistryID int `json:"registryID"`
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mutate(id, func(wh *webhookRecord) { wh.RegistryID = body.RegistryID })
			RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
		})
		m.On("PUT", p+"/reassign", func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				ResourceID  string
				WebhookType int
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mutate(id, func(wh *webhookRecord) { wh.ResourceID, wh.Type = body.ResourceID, body.WebhookType })
			RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
		})
		m.On("DELETE", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			delete(f.webhooks, id)
			f.mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		})
	}
	return f
}

func (f *fakeWebhooks) mutate(id int, fn func(*webhookRecord)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if wh, ok := f.webhooks[id]; ok {
		fn(wh)
	}
}

func webhookConfig(m *MockServer, resourceID, extra string) string {
	return mockProviderConfig(m) + fmt.Sprintf(`
resource "portainer_webhook" "w" {
  endpoint_id  = 1
  resource_id  = %q
  webhook_type = 1
  %s
}
`, resourceID, extra)
}

func lastRequestBody(t *testing.T, mock *MockServer, method, path string) map[string]interface{} {
	t.Helper()
	var last *RecordedRequest
	for _, r := range mock.Requests() {
		if r.Method == method && r.Path == path {
			last = r
		}
	}
	if last == nil {
		return nil
	}
	var payload map[string]interface{}
	if err := last.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode %s %s: %v", method, path, err)
	}
	return payload
}

// TestWebhookResource_Lifecycle: create payload and token, import, a registry
// change sent as an update, and a new target resource replacing the webhook
// by default.
func TestWebhookResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeWebhooks(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: webhookConfig(mock, "svc-1", ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_webhook.w", "id", "1"),
					resource.TestCheckResourceAttr("portainer_webhook.w", "token", "tok-1"),
					resource.TestCheckResourceAttr("portainer_webhook.w", "reassign_on_change", "false"),
					func(*terraform.State) error {
						p := lastRequestBody(t, mock, "POST", "/webhooks")
						if p["endpointID"] != float64(1) || p["resourceID"] != "svc-1" || p["webhookType"] != float64(1) {
							return fmt.Errorf("create payload: %v", p)
						}
						if _, ok := p["registryID"]; ok {
							return fmt.Errorf("an unset registry must be left out: %v", p)
						}
						return nil
					},
				),
			},
			{
				ResourceName:            "portainer_webhook.w",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"registry_id"},
			},
			{
				// The long form, which Business Edition needs.
				ResourceName: "portainer_webhook.w",
				ImportState:  true,
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					r := s.RootModule().Resources["portainer_webhook.w"].Primary
					return r.Attributes["endpoint_id"] + ":" + r.Attributes["resource_id"] + ":" + r.ID, nil
				},
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"registry_id"},
			},
			{
				Config: webhookConfig(mock, "svc-1", "registry_id = 2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_webhook.w", plancheck.ResourceActionUpdate),
				}},
				Check: func(*terraform.State) error {
					if p := lastRequestBody(t, mock, "PUT", "/webhooks/1"); p["registryID"] != float64(2) {
						return fmt.Errorf("registry update payload: %v", p)
					}
					return nil
				},
			},
			{
				Config: webhookConfig(mock, "svc-2", "registry_id = 2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_webhook.w", plancheck.ResourceActionDestroyBeforeCreate),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_webhook.w", "id", "2"),
					func(*terraform.State) error {
						if mock.FindRequest("PUT", "/webhooks/1/reassign") != nil {
							return fmt.Errorf("the reassign endpoint must not be used without reassign_on_change")
						}
						return nil
					},
				),
			},
		},
	})
}

// TestWebhookResource_Reassign: with reassign_on_change the webhook is moved
// in place and keeps its token.
func TestWebhookResource_Reassign(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeWebhooks(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: webhookConfig(mock, "svc-old", "reassign_on_change = true")},
			{
				Config: webhookConfig(mock, "svc-new", "reassign_on_change = true"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_webhook.w", plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_webhook.w", "id", "1"),
					resource.TestCheckResourceAttr("portainer_webhook.w", "token", "tok-1"),
					func(*terraform.State) error {
						p := lastRequestBody(t, mock, "PUT", "/webhooks/1/reassign")
						if p["ResourceID"] != "svc-new" || p["WebhookType"] != float64(1) {
							return fmt.Errorf("the reassign payload uses PascalCase keys, got %v", p)
						}
						return nil
					},
				),
			},
		},
	})
}

// TestWebhookResource_Drift: the list is filtered by environment and
// resource; a webhook reassigned outside Terraform is found in the full list
// and shows as drift, and one deleted outside Terraform is planned again.
func TestWebhookResource_Drift(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeWebhooks(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: webhookConfig(mock, "svc-1", ""),
			},
			{
				PreConfig: func() {
					// The refresh after the apply listed the webhooks of the
					// environment and resource only.
					var filtered bool
					for _, r := range mock.Requests() {
						if r.Method == http.MethodGet && r.Path == "/webhooks" {
							q, _ := url.ParseQuery(r.Query)
							filtered = strings.Contains(q.Get("filters"), `"EndpointID":1`) && strings.Contains(q.Get("filters"), `"ResourceID":"svc-1"`)
							break
						}
					}
					if !filtered {
						t.Error("expected the list filtered by environment and resource")
					}
					fake.mutate(1, func(wh *webhookRecord) { wh.ResourceID = "elsewhere" })
				},
				Config:             webhookConfig(mock, "svc-1", ""),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				PreConfig: func() {
					fake.mu.Lock()
					delete(fake.webhooks, 1)
					fake.mu.Unlock()
				},
				Config: webhookConfig(mock, "svc-1", ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_webhook.w", plancheck.ResourceActionCreate),
				}},
			},
		},
	})
}

func TestWebhookResource_Errors(t *testing.T) {
	t.Parallel()
	t.Run("create", func(t *testing.T) {
		mock := NewMockServer(t)
		mock.On("POST", "/webhooks", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps:                    []resource.TestStep{{Config: webhookConfig(mock, "svc-1", ""), ExpectError: regexp.MustCompile(`failed to create webhook`)}},
		})
	})
	t.Run("registry update", func(t *testing.T) {
		mock := NewMockServer(t)
		mock.On("PUT", "/webhooks/1", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))
		newFakeWebhooks(mock)
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{
				{Config: webhookConfig(mock, "svc-1", "")},
				{Config: webhookConfig(mock, "svc-1", "registry_id = 3"), ExpectError: regexp.MustCompile(`failed to update webhook`)},
			},
		})
	})
	t.Run("malformed import ID", func(t *testing.T) {
		mock := NewMockServer(t)
		newFakeWebhooks(mock)
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{{
				Config:        webhookConfig(mock, "svc-1", ""),
				ResourceName:  "portainer_webhook.w",
				ImportState:   true,
				ImportStateId: "abc",
				ExpectError:   regexp.MustCompile(`invalid webhook ID`),
			}},
		})
	})
}

func TestWebhookResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for name, extra := range map[string]string{
		"minimal":  "",
		"registry": "registry_id = 2",
		"reassign": "reassign_on_change = true",
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			newFakeWebhooks(mock)
			config := webhookConfig(mock, "svc-1", extra)
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

func TestWebhookDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/webhooks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 4, "EndpointId": 1, "ResourceId": "svc-1", "Type": 1, "Token": "tok-4"},
	}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `data "portainer_webhook" "w" {
  resource_id = "svc-1"
  endpoint_id = 1
}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("data.portainer_webhook.w", "id", "4"),
					resource.TestCheckResourceAttr("data.portainer_webhook.w", "webhook_type", "1"),
					resource.TestCheckResourceAttr("data.portainer_webhook.w", "token", "tok-4"),
				),
			},
			{
				Config: mockProviderConfig(mock) + `data "portainer_webhook" "w" {
  resource_id = "svc-1"
  endpoint_id = 2
}`,
				ExpectError: regexp.MustCompile(`webhook for resource svc-1 in endpoint 2 not found`),
			},
		},
	})
}

func TestWebhookDataSource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/webhooks", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_webhook" "w" {
  resource_id = "svc-1"
  endpoint_id = 1
}`,
			ExpectError: regexp.MustCompile(`failed to list webhooks`),
		}},
	})
}

// --- portainer_webhook_execute ---

func TestWebhookExecuteResource(t *testing.T) {
	t.Parallel()
	cases := []struct{ attr, value, path string }{
		{"token", "tok-abc", "/webhooks/tok-abc"},
		{"stack_id", "st-1", "/stacks/webhooks/st-1"},
		{"edge_stack_id", "es-1", "/edge_stacks/webhooks/es-1"},
	}
	for _, tc := range cases {
		t.Run(tc.attr, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("POST", tc.path, RespondString(http.StatusNoContent, "", ""))
			config := mockProviderConfig(mock) + fmt.Sprintf(`resource "portainer_webhook_execute" "x" { %s = %q }`, tc.attr, tc.value)
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				CheckDestroy: func(*terraform.State) error {
					if n := len(mock.Requests()); n != 1 {
						return fmt.Errorf("read and destroy must not call the API, got %d requests", n)
					}
					return nil
				},
				Steps: []resource.TestStep{{
					Config: config,
					Check: resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttr("portainer_webhook_execute.x", "id", tc.value),
						func(*terraform.State) error {
							req := mock.FindRequest("POST", tc.path)
							if req == nil {
								return fmt.Errorf("expected POST %s", tc.path)
							}
							if req.Headers.Get("X-API-Key") != "" {
								return fmt.Errorf("a webhook call carries no credentials")
							}
							return nil
						},
					),
				}},
			})
		})
	}
}

func TestWebhookExecuteResource_Errors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/webhooks/bad", RespondString(http.StatusNotFound, "text/plain", "nope"))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:      mockProviderConfig(mock) + `resource "portainer_webhook_execute" "x" {}`,
				ExpectError: regexp.MustCompile(`one of 'token', 'stack_id' or 'edge_stack_id' must be set`),
			},
			{
				Config:      mockProviderConfig(mock) + `resource "portainer_webhook_execute" "x" { token = "bad" }`,
				ExpectError: regexp.MustCompile(`failed to execute webhook: HTTP 404`),
			},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_webhook_execute" "x" {
  token    = "bad"
  stack_id = "1"
}`,
				ExpectError: regexp.MustCompile(`cannot be specified`),
			},
		},
	})
}

func TestWebhookExecuteResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/stacks/webhooks/st-1", RespondString(http.StatusNoContent, "", ""))
	config := mockProviderConfig(mock) + `resource "portainer_webhook_execute" "x" { stack_id = "st-1" }`
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

// TestAccWebhookResource needs a Swarm environment with a service
// (PORTAINER_ACC_SWARM_ENV_ID, PORTAINER_ACC_SWARM_SERVICE): service webhooks
// are the only kind Community Edition accepts.
func TestAccWebhookResource(t *testing.T) {
	envID, service := os.Getenv("PORTAINER_ACC_SWARM_ENV_ID"), os.Getenv("PORTAINER_ACC_SWARM_SERVICE")
	if envID == "" || service == "" {
		t.Skip("PORTAINER_ACC_SWARM_ENV_ID and PORTAINER_ACC_SWARM_SERVICE must name a Swarm environment and service")
	}
	config := fmt.Sprintf(`
resource "portainer_webhook" "w" {
  endpoint_id  = %s
  resource_id  = %q
  webhook_type = 1
}

data "portainer_webhook" "w" {
  endpoint_id = portainer_webhook.w.endpoint_id
  resource_id = portainer_webhook.w.resource_id
}
`, envID, service)
	resource.Test(t, resource.TestCase{
		PreCheck: func() { accPreCheck(t) },
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: accProvider() + config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   accProvider() + config,
				ConfigPlanChecks:         resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
				Check:                    resource.TestCheckResourceAttrPair("data.portainer_webhook.w", "token", "portainer_webhook.w", "token"),
			},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   accProvider() + config,
				ResourceName:             "portainer_webhook.w",
				ImportState:              true,
				// The long form works on both editions (Business Edition
				// cannot list webhooks without the environment and resource).
				ImportStateIdFunc: func(s *terraform.State) (string, error) {
					r := s.RootModule().Resources["portainer_webhook.w"].Primary
					return r.Attributes["endpoint_id"] + ":" + r.Attributes["resource_id"] + ":" + r.ID, nil
				},
				ImportStateVerify: true,
			},
		},
	})
}
