package internal

import (
	"fmt"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func egLastRelations(fake *fakeEndpointGroups) map[string]map[string]interface{} {
	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.relations) == 0 {
		return nil
	}
	raw, _ := fake.relations[len(fake.relations)-1]["Relations"].(map[string]interface{})
	out := map[string]map[string]interface{}{}
	for k, v := range raw {
		out[k], _ = v.(map[string]interface{})
	}
	return out
}

// TestEndpointRelationsResource_Payload: the payload is keyed by environment
// ID, and every field that means "leave this alone" is omitted — an empty
// array would CLEAR the environment's tags or edge groups, and Portainer only
// applies a non-zero group.
func TestEndpointRelationsResource_Payload(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEndpointGroups(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `resource "portainer_endpoint_relations" "r" {
  relation {
    endpoint_id    = 9
    edge_group_ids = [1, 2]
    tag_ids        = [3]
    group_id       = 5
  }
  relation {
    endpoint_id    = 10
    edge_group_ids = []
    tag_ids        = []
    group_id       = 0
  }
  relation {
    endpoint_id    = 11
    edge_group_ids = [1]
  }
  relation {
    endpoint_id = 12
  }
}`,
			Check: func(*terraform.State) error {
				rel := egLastRelations(fake)
				nine := rel["9"]
				if nine["Group"] != float64(5) || len(nine["EdgeGroups"].([]interface{})) != 2 || len(nine["Tags"].([]interface{})) != 1 {
					return fmt.Errorf("relation 9: %v", nine)
				}
				for _, id := range []string{"10", "12"} {
					r, ok := rel[id]
					if !ok || len(r) != 0 {
						return fmt.Errorf("relation %s must be an empty object, got %v (present %v)", id, r, ok)
					}
				}
				eleven := rel["11"]
				if _, ok := eleven["Tags"]; ok {
					return fmt.Errorf("Tags must be omitted when not configured: %v", eleven)
				}
				if _, ok := eleven["Group"]; ok {
					return fmt.Errorf("Group must be omitted when zero: %v", eleven)
				}
				return nil
			},
		}},
	})
}

// TestEndpointRelationsResource_Update re-applies the relations in place and
// keeps the ID.
func TestEndpointRelationsResource_Update(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEndpointGroups(mock)
	var firstID string

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `resource "portainer_endpoint_relations" "r" {
  relation {
    endpoint_id = 9
    group_id    = 5
  }
}`,
				Check: func(s *terraform.State) error {
					firstID = s.RootModule().Resources["portainer_endpoint_relations.r"].Primary.ID
					if !regexp.MustCompile(`^endpoint-relations-\d+$`).MatchString(firstID) {
						return fmt.Errorf("unexpected ID %q", firstID)
					}
					return nil
				},
			},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_endpoint_relations" "r" {
  relation {
    endpoint_id = 9
    group_id    = 6
  }
}`,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_endpoint_relations.r", plancheck.ResourceActionUpdate)},
				},
				Check: func(s *terraform.State) error {
					if id := s.RootModule().Resources["portainer_endpoint_relations.r"].Primary.ID; id != firstID {
						return fmt.Errorf("ID changed from %q to %q", firstID, id)
					}
					if g := egLastRelations(fake)["9"]["Group"]; g != float64(6) {
						return fmt.Errorf("Group: %v", g)
					}
					return nil
				},
			},
		},
	})
}

func TestEndpointRelationsResource_RequiresARelation(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_endpoint_relations" "r" {}`,
			ExpectError: regexp.MustCompile(`relation`),
		}},
	})
}

// TestEndpointRelationsResource_UpgradeFromV300: state written by v3.0.0
// plans no change with this build, with and without the optional fields.
func TestEndpointRelationsResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEndpointGroups(mock)
	config := mockProviderConfig(mock) + `resource "portainer_endpoint_relations" "r" {
  relation {
    endpoint_id    = 9
    edge_group_ids = [1, 2]
    tag_ids        = [3]
    group_id       = 5
  }
  relation {
    endpoint_id = 10
  }
  relation {
    endpoint_id = 11
    tag_ids     = []
  }
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
}
