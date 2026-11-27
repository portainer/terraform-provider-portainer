package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/acctest"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestEndpointGroupResource_Lifecycle: create (POST payload), update (PUT
// payload), import, destroy.
func TestEndpointGroupResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEndpointGroups(mock)
	fake.seed(1, "Unassigned")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `resource "portainer_endpoint_group" "g" {
  name        = "Production"
  description = "prod group"
  tag_ids     = [10, 20]
}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_endpoint_group.g", "id", "2"),
					resource.TestCheckResourceAttr("portainer_endpoint_group.g", "tag_ids.#", "2"),
					func(*terraform.State) error {
						body := a1LastBody(mock, "POST", "/endpoint_groups")
						if body["name"] != "Production" || body["description"] != "prod group" {
							return fmt.Errorf("POST payload: %v", body)
						}
						if tags, _ := body["tagIDs"].([]interface{}); len(tags) != 2 {
							return fmt.Errorf("POST tagIDs: %v", body["tagIDs"])
						}
						return nil
					},
				),
			},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_endpoint_group" "g" {
  name        = "renamed"
  description = "new desc"
  tag_ids     = [10]
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_endpoint_group.g", plancheck.ResourceActionUpdate)},
				},
				Check: func(*terraform.State) error {
					body := a1LastBody(mock, "PUT", "/endpoint_groups/2")
					if body["name"] != "renamed" || body["description"] != "new desc" {
						return fmt.Errorf("PUT payload: %v", body)
					}
					return nil
				},
			},
			{
				ResourceName:      "portainer_endpoint_group.g",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
		CheckDestroy: func(*terraform.State) error {
			if fake.group(2) != nil {
				return fmt.Errorf("group 2 still exists")
			}
			return nil
		},
	})
}

// TestEndpointGroupResource_OnlyName: omitted description and tags stay null
// and send no tags.
func TestEndpointGroupResource_OnlyName(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEndpointGroups(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_endpoint_group" "g" { name = "solo" }`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckNoResourceAttr("portainer_endpoint_group.g", "description"),
				resource.TestCheckNoResourceAttr("portainer_endpoint_group.g", "tag_ids.#"),
				func(*terraform.State) error {
					body := a1LastBody(mock, "POST", "/endpoint_groups")
					if _, ok := body["description"]; ok {
						return fmt.Errorf("description must be omitted: %v", body)
					}
					if body["tagIDs"] != nil {
						return fmt.Errorf("tagIDs must be null: %v", body)
					}
					return nil
				},
			),
		}},
	})
}

// TestEndpointGroupResource_ExistingNameAdopted: a group with the same name is
// updated instead of duplicated.
func TestEndpointGroupResource_ExistingNameAdopted(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEndpointGroups(mock)
	fake.seed(7, "Production")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_endpoint_group" "g" {
  name        = "Production"
  description = "adopted"
}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_endpoint_group.g", "id", "7"),
				func(*terraform.State) error {
					if mock.FindRequest("POST", "/endpoint_groups") != nil {
						return fmt.Errorf("no POST expected when the name exists")
					}
					if body := a1LastBody(mock, "PUT", "/endpoint_groups/7"); body["description"] != "adopted" {
						return fmt.Errorf("PUT payload: %v", body)
					}
					return nil
				},
			),
		}},
	})
}

// TestEndpointGroupResource_GoneOutsideTerraform: a deleted group is planned
// for creation again, and destroying it is not an error (404 swallowed).
func TestEndpointGroupResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEndpointGroups(mock)
	config := mockProviderConfig(mock) + `resource "portainer_endpoint_group" "g" { name = "gone" }`

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig:          func() { fake.remove(2) },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestEndpointGroupResource_ReadError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoint_groups", RespondJSON(http.StatusOK, []interface{}{}))
	mock.On("POST", "/endpoint_groups", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 3}))
	mock.On("GET", "/endpoint_groups/3", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_endpoint_group" "g" { name = "x" }`,
			ExpectError: regexp.MustCompile(`failed to read endpoint group`),
		}},
	})
}

func TestEndpointGroupResource_EmptyNameRejected(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_endpoint_group" "g" { name = "" }`,
			ExpectError: regexp.MustCompile(`at least 1`),
		}},
	})
}

// TestEndpointGroupResource_UpgradeFromV300: state written by v3.0.0 plans
// no change with this build, with and without the optional attributes.
func TestEndpointGroupResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for name, cfg := range map[string]string{
		"all":        `resource "portainer_endpoint_group" "g" {` + "\n" + `  name = "full"` + "\n" + `  description = "d"` + "\n" + `  tag_ids = [1, 2]` + "\n}",
		"name":       `resource "portainer_endpoint_group" "g" { name = "bare" }`,
		"empty_tags": `resource "portainer_endpoint_group" "g" {` + "\n" + `  name = "et"` + "\n" + `  tag_ids = []` + "\n}",
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			newFakeEndpointGroups(mock)
			config := mockProviderConfig(mock) + cfg
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

func TestAccEndpointGroupResource_basic(t *testing.T) {
	name := acctest.RandomWithPrefix("acc-eg")
	config := fmt.Sprintf(`
resource "portainer_endpoint_group" "g" {
  name        = %q
  description = "acceptance"
}

data "portainer_endpoint_group" "g" {
  name       = portainer_endpoint_group.g.name
  depends_on = [portainer_endpoint_group.g]
}
`, name)
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: accProvider() + config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrPair("data.portainer_endpoint_group.g", "id", "portainer_endpoint_group.g", "id"),
					resource.TestCheckResourceAttr("data.portainer_endpoint_group.g", "description", "acceptance"),
				),
			},
			{
				Config:            accProvider() + config,
				ResourceName:      "portainer_endpoint_group.g",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
}

func TestAccEndpointGroupResource_UpgradeFromSDKv2(t *testing.T) {
	config := fmt.Sprintf(`resource "portainer_endpoint_group" "g" { name = %q }`, acctest.RandomWithPrefix("acc-eg"))
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
		},
	})
}

// --- data source portainer_endpoint_group ----------------------------------

func TestEndpointGroupDataSource_Read(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoint_groups", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "default", "Description": "Unassigned"},
		{"Id": 7, "Name": "infra", "Description": "Infra hosts"},
	}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_endpoint_group" "g" { name = "infra" }`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("data.portainer_endpoint_group.g", "id", "7"),
				resource.TestCheckResourceAttr("data.portainer_endpoint_group.g", "description", "Infra hosts"),
			),
		}},
	})
}

func TestEndpointGroupDataSource_NotFound(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoint_groups", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "default", "Description": "Unassigned"},
	}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `data "portainer_endpoint_group" "g" { name = "ghost" }`,
			ExpectError: regexp.MustCompile(`endpoint group ghost not found`),
		}},
	})
}

func TestEndpointGroupDataSource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoint_groups", RespondString(http.StatusForbidden, "application/json", `{"message":"nope"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `data "portainer_endpoint_group" "g" { name = "infra" }`,
			ExpectError: regexp.MustCompile(`failed to list endpoint groups`),
		}},
	})
}
