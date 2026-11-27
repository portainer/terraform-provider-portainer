package internal

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestEndpointsSummaryDataSource_HealthCounters verifies the nested byHealth
// and byPlatformType objects are flattened, the nested counters winning.
func TestEndpointsSummaryDataSource_HealthCounters(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/summary", RespondJSON(http.StatusOK, map[string]interface{}{
		"total": 12, "unassigned": 2, "up": 1,
		"byHealth":       map[string]interface{}{"up": 9, "down": 3, "heartbeat": 5, "outdated": 1},
		"byPlatformType": map[string]interface{}{"docker": 7, "kubernetes": 4, "podman": 1},
		"byGroup": []map[string]interface{}{
			{"groupId": 1, "groupName": "Unassigned", "count": 2},
			{"groupId": 2, "groupName": "production", "count": 10},
		},
	}))

	name := "data.portainer_endpoints_summary.s"
	checks := make([]resource.TestCheckFunc, 0, 14)
	checks = append(checks,
		resource.TestCheckResourceAttr(name, "id", "portainer-endpoints-summary"),
		resource.TestCheckResourceAttr(name, "by_group.#", "2"),
		resource.TestCheckResourceAttr(name, "by_group.1.group_name", "production"),
		resource.TestCheckResourceAttr(name, "by_group.1.count", "10"),
		resource.TestCheckResourceAttr(name, "azure", "0"),
	)
	for field, want := range map[string]string{
		"total": "12", "up": "9", "down": "3", "heartbeat": "5", "outdated": "1",
		"unassigned": "2", "docker": "7", "kubernetes": "4", "podman": "1",
	} {
		checks = append(checks, resource.TestCheckResourceAttr(name, field, want))
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_endpoints_summary" "s" {}`,
			Check:  resource.ComposeTestCheckFunc(checks...),
		}},
	})
}

// TestEndpointsSummaryDataSource_TopLevelFallback: without byHealth the
// top-level counters are used.
func TestEndpointsSummaryDataSource_TopLevelFallback(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/summary", RespondJSON(http.StatusOK, map[string]interface{}{
		"total": 3, "up": 2, "down": 1, "outdated": 1,
	}))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_endpoints_summary" "s" {}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("data.portainer_endpoints_summary.s", "up", "2"),
				resource.TestCheckResourceAttr("data.portainer_endpoints_summary.s", "down", "1"),
				resource.TestCheckResourceAttr("data.portainer_endpoints_summary.s", "by_group.#", "0"),
			),
		}},
	})
}

func TestAccEndpointsSummaryDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck: func() { accPreCheck(t) },
		Steps: []resource.TestStep{{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Config:                   accProvider() + `data "portainer_endpoints_summary" "s" {}`,
			Check:                    resource.TestCheckResourceAttrSet("data.portainer_endpoints_summary.s", "total"),
		}},
	})
}
