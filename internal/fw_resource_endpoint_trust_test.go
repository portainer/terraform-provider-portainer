package internal

import (
	"fmt"
	"net/http"
	"os"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakeTrustedEndpoint is environment 12, trusted by POST /endpoints/edge/trust.
type fakeTrustedEndpoint struct {
	mu      sync.Mutex
	trusted bool
}

func newFakeTrustedEndpoint(m *MockServer) *fakeTrustedEndpoint {
	f := &fakeTrustedEndpoint{}
	m.On("POST", "/endpoints/edge/trust", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		f.trusted = true
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	m.On("GET", "/endpoints/12", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		RespondJSON(http.StatusOK, map[string]interface{}{
			"Id": 12, "Name": "shop-floor-3", "EdgeID": "abc-123", "UserTrusted": f.trusted,
		})(w, r)
	})
	return f
}

// TestEndpointTrustResource_SendsBatchOfOne pins the payload: Portainer's
// endpoint takes a batch, and the relations are keyed by the environment
// identifier as a string rather than being part of the entry itself. Destroy
// does not call Portainer (there is no endpoint to revoke trust).
func TestEndpointTrustResource_SendsBatchOfOne(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeTrustedEndpoint(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		CheckDestroy: func(_ *terraform.State) error {
			for _, r := range mock.Requests() {
				if r.Method != http.MethodGet && r.Method != http.MethodPost {
					return fmt.Errorf("destroy must not call Portainer, got %s %s", r.Method, r.Path)
				}
			}
			if n := countRequestsA2(mock, "POST", "/endpoints/edge/trust"); n != 1 {
				return fmt.Errorf("expected one trust call, got %d", n)
			}
			return nil
		},
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `resource "portainer_endpoint_trust" "t" {
  endpoint_id    = 12
  group_id       = 2
  edge_group_ids = [4, 5]
  tag_ids        = [7]
}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_endpoint_trust.t", "id", "12"),
					resource.TestCheckResourceAttr("portainer_endpoint_trust.t", "name", "shop-floor-3"),
					resource.TestCheckResourceAttr("portainer_endpoint_trust.t", "edge_id", "abc-123"),
					resource.TestCheckResourceAttr("portainer_endpoint_trust.t", "trusted", "true"),
					func(_ *terraform.State) error {
						var payload struct {
							EndpointIDs []int `json:"EndpointIDs"`
							Relations   map[string]struct {
								Group      int   `json:"Group"`
								EdgeGroups []int `json:"EdgeGroups"`
								Tags       []int `json:"Tags"`
							} `json:"Relations"`
						}
						if err := mock.FindRequest("POST", "/endpoints/edge/trust").DecodeJSON(&payload); err != nil {
							return err
						}
						if len(payload.EndpointIDs) != 1 || payload.EndpointIDs[0] != 12 {
							return fmt.Errorf("expected a batch holding just this environment, got %v", payload.EndpointIDs)
						}
						relation, ok := payload.Relations["12"]
						if !ok {
							return fmt.Errorf("the relations must be keyed by the environment identifier, got %v", payload.Relations)
						}
						if relation.Group != 2 || len(relation.EdgeGroups) != 2 || len(relation.Tags) != 1 {
							return fmt.Errorf("the configured relations were not sent: %+v", relation)
						}
						return nil
					},
				),
			},
			{
				ResourceName:      "portainer_endpoint_trust.t",
				ImportState:       true,
				ImportStateVerify: true,
				// Only honoured when trust is granted, never read back.
				ImportStateVerifyIgnore: []string{"group_id", "edge_group_ids", "tag_ids"},
			},
		},
	})
}

// TestEndpointTrustResource_OmitsUnconfiguredRelations is the destructive
// case: Portainer reads an empty array as "clear this", so sending one for a
// relation the configuration never mentioned would strip what the
// environment already carries. The omission is per field.
func TestEndpointTrustResource_OmitsUnconfiguredRelations(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		config  string
		present []string
		absent  []string
	}{
		"none":      {`endpoint_id = 12`, nil, nil},
		"tags only": {"endpoint_id = 12\n  tag_ids = [7]", []string{"Tags"}, []string{"Group", "EdgeGroups"}},
		"empty":     {"endpoint_id = 12\n  tag_ids = []\n  group_id = 0", nil, nil},
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			newFakeTrustedEndpoint(mock)
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{{
					Config: mockProviderConfig(mock) + "resource \"portainer_endpoint_trust\" \"t\" {\n  " + tc.config + "\n}",
					Check: func(_ *terraform.State) error {
						var payload struct {
							Relations map[string]map[string]interface{} `json:"Relations"`
						}
						if err := mock.FindRequest("POST", "/endpoints/edge/trust").DecodeJSON(&payload); err != nil {
							return err
						}
						if tc.present == nil {
							if payload.Relations != nil {
								return fmt.Errorf("no relations were configured, so none must be sent: %v", payload.Relations)
							}
							return nil
						}
						relation := payload.Relations["12"]
						for _, key := range tc.present {
							if _, ok := relation[key]; !ok {
								return fmt.Errorf("the configured %s must be sent", key)
							}
						}
						for _, key := range tc.absent {
							if _, ok := relation[key]; ok {
								return fmt.Errorf("%s was not configured, so it must not be sent: %v", key, relation[key])
							}
						}
						return nil
					},
				}},
			})
		})
	}
}

// TestEndpointTrustResource_UntrustedLeavesState covers the drift an apply
// can fix: an environment put back in the waiting room is planned for trust
// again.
func TestEndpointTrustResource_UntrustedLeavesState(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeTrustedEndpoint(mock)
	config := mockProviderConfig(mock) + `resource "portainer_endpoint_trust" "t" { endpoint_id = 12 }`

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.trusted = false
					fake.mu.Unlock()
				},
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_endpoint_trust.t", plancheck.ResourceActionCreate)},
				},
			},
		},
	})
}

// TestEndpointTrustResource_RelationChangeReplaces: the relations are only
// honoured by the trust call, so changing one re-runs it.
func TestEndpointTrustResource_RelationChangeReplaces(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeTrustedEndpoint(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + `resource "portainer_endpoint_trust" "t" { endpoint_id = 12 }`},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_endpoint_trust" "t" {
  endpoint_id = 12
  tag_ids     = [3]
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_endpoint_trust.t", plancheck.ResourceActionDestroyBeforeCreate)},
				},
			},
		},
	})
}

// TestEndpointTrustResource_UpgradeFromV300: state written by v3.0.0 plans no
// change with this build, with and without the optional relations.
func TestEndpointTrustResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for name, body := range map[string]string{
		"minimal": `endpoint_id = 12`,
		"full":    "endpoint_id = 12\n  group_id = 2\n  edge_group_ids = [4, 5]\n  tag_ids = [7]",
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			newFakeTrustedEndpoint(mock)
			config := mockProviderConfig(mock) + "resource \"portainer_endpoint_trust\" \"t\" {\n  " + body + "\n}"
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

// TestAccEndpointTrustResource needs Business Edition and an edge agent
// waiting for trust, which a plain CE instance cannot provide.
func TestAccEndpointTrustResource(t *testing.T) {
	if os.Getenv("PORTAINER_ACC_BE") != "1" {
		t.Skip("portainer_endpoint_trust needs Business Edition (PORTAINER_ACC_BE=1) and an edge agent in the waiting room")
	}
	t.Skip("no edge agent in the waiting room is provisioned by the acceptance environment")
}
