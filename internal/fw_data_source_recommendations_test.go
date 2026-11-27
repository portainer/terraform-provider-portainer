package internal

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// TestRecommendationsDataSource_FiltersListButNotSummary pins the
// distinction: the filters narrow the listing, the summary is over everything.
func TestRecommendationsDataSource_FiltersListButNotSummary(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/recommendations", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"typeId": "backup-not-configured", "title": "Configure backups",
			"description": "No backup schedule", "category": "resilience", "severity": "high",
			"actionLabel": "Configure", "actionUrl": "/settings/backup",
		},
	}))
	mock.On("GET", "/recommendations/summary", RespondJSON(http.StatusOK, map[string]interface{}{
		"total": 9, "totalTypes": 4,
		"severityCounts": map[string]int{"high": 2, "low": 7},
		"categoryCounts": map[string]int{"resilience": 3, "security": 6},
	}))
	const ds = "data.portainer_recommendations.r"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_recommendations" "r" { severity = "high" }`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(ds, "id", "portainer-recommendations"),
				resource.TestCheckResourceAttr(ds, "recommendations.#", "1"),
				resource.TestCheckResourceAttr(ds, "recommendations.0.type_id", "backup-not-configured"),
				resource.TestCheckResourceAttr(ds, "recommendations.0.action_url", "/settings/backup"),
				resource.TestCheckResourceAttr(ds, "total", "9"),
				resource.TestCheckResourceAttr(ds, "total_types", "4"),
				resource.TestCheckResourceAttr(ds, "severity_counts.%", "2"),
				resource.TestCheckResourceAttr(ds, "severity_counts.low", "7"),
				resource.TestCheckResourceAttr(ds, "category_counts.security", "6"),
				func(*terraform.State) error {
					req := mock.FindRequest("GET", "/recommendations")
					if req == nil || req.Query != "severity=high" {
						return fmt.Errorf("expected only the severity filter in the query, got %+v", req)
					}
					return nil
				},
			),
		}},
	})
}
