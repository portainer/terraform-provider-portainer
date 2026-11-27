package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const egAssociationConfig = `resource "portainer_endpoint_association" "a" { endpoint_id = 12 }`

// TestEndpointAssociationResource_Lifecycle: create sends the de-association
// PUT, the ID is the environment ID, and destroy only forgets the resource.
func TestEndpointAssociationResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEndpointGroups(mock).setEndpointGroup(12, 1)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + egAssociationConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_endpoint_association.a", "id", "12"),
					resource.TestCheckResourceAttr("portainer_endpoint_association.a", "endpoint_id", "12"),
					func(*terraform.State) error {
						if a1CountRequests(mock, "PUT", "/endpoints/12/association") != 1 {
							return fmt.Errorf("expected one PUT /endpoints/12/association")
						}
						return nil
					},
				),
			},
		},
		CheckDestroy: func(*terraform.State) error {
			for _, r := range mock.Requests() {
				if r.Method == http.MethodDelete {
					return fmt.Errorf("destroy must not call the API, got DELETE %s", r.Path)
				}
			}
			return nil
		},
	})
}

func TestEndpointAssociationResource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("PUT", "/endpoints/4/association", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_endpoint_association" "a" { endpoint_id = 4 }`,
			ExpectError: regexp.MustCompile(`failed to de-associate endpoint 4`),
		}},
	})
}

// TestEndpointAssociationResource_EnvironmentGone: a deleted environment
// drops the resource.
func TestEndpointAssociationResource_EnvironmentGone(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEndpointGroups(mock)
	fake.setEndpointGroup(12, 1)
	config := mockProviderConfig(mock) + egAssociationConfig
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig:          func() { fake.removeEndpoint(12) },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestEndpointAssociationResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEndpointGroups(mock).setEndpointGroup(12, 1)
	config := mockProviderConfig(mock) + egAssociationConfig
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
