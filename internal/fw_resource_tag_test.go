package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// fakeTags is a stateful stand-in for Portainer's tag API.
type fakeTags struct {
	mu     sync.Mutex
	nextID int64
	tags   []portainerTag
}

func (f *fakeTags) register(m *MockServer) {
	m.On("GET", "/tags", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		RespondJSON(http.StatusOK, f.tags)(w, nil)
	})
	m.On("POST", "/tags", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Name string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.nextID++
		tag := portainerTag{ID: f.nextID, Name: body.Name}
		f.tags = append(f.tags, tag)
		RespondJSON(http.StatusOK, tag)(w, r)
	})
}

func (f *fakeTags) registerDelete(m *MockServer, id string) {
	m.On("DELETE", "/tags/"+id, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		for i, t := range f.tags {
			if strings.HasSuffix(r.URL.Path, "/"+id) && t.ID == f.nextID {
				f.tags = append(f.tags[:i], f.tags[i+1:]...)
				break
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})
}

// TestTagResource_Lifecycle runs real Terraform against the fake: create,
// an empty plan, a rename that replaces the tag, import and destroy.
func TestTagResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := &fakeTags{}
	fake.register(mock)
	fake.registerDelete(mock, "1")
	fake.registerDelete(mock, "2")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `resource "portainer_tag" "t" { name = "production" }`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_tag.t", "id", "1"),
					resource.TestCheckResourceAttr("portainer_tag.t", "name", "production"),
				),
			},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_tag" "t" { name = "staging" }`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_tag.t", plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: resource.TestCheckResourceAttr("portainer_tag.t", "id", "2"),
			},
			{
				ResourceName:      "portainer_tag.t",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

// TestTagResource_GoneOutsideTerraform: a tag deleted in Portainer is planned
// for creation again.
func TestTagResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := &fakeTags{}
	fake.register(mock)
	fake.registerDelete(mock, "1")
	fake.registerDelete(mock, "2")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + `resource "portainer_tag" "t" { name = "production" }`},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.tags = nil
					fake.mu.Unlock()
				},
				Config:             mockProviderConfig(mock) + `resource "portainer_tag" "t" { name = "production" }`,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestTagResource_EmptyNameRejected(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_tag" "t" { name = "" }`,
			ExpectError: regexp.MustCompile(`at least 1`),
		}},
	})
}

// TestAccTagResource_UpgradeFromSDKv2: a tag created by v3.0.0 (SDKv2) plans
// no change with this build, then imports cleanly.
func TestAccTagResource_UpgradeFromSDKv2(t *testing.T) {
	config := fmt.Sprintf(`resource "portainer_tag" "t" { name = %q }`, acctest.RandomWithPrefix("acc-tag"))
	resource.Test(t, resource.TestCase{
		PreCheck: func() { accPreCheck(t) },
		Steps: []resource.TestStep{
			{
				ExternalProviders: previousRelease,
				Config:            accProvider() + config,
			},
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
				ResourceName:             "portainer_tag.t",
				ImportState:              true,
				ImportStateVerify:        true,
			},
		},
	})
}

// TestTagResource_UpgradeFromV300: state written by v3.0.0 (SDKv2) against the
// same fake API plans no change with this build.
func TestTagResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := &fakeTags{}
	fake.register(mock)
	fake.registerDelete(mock, "1")

	config := mockProviderConfig(mock) + `resource "portainer_tag" "t" { name = "production" }`
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
}
