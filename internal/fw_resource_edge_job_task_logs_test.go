package internal

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

const edgeJobTaskLogsHCL = `resource "portainer_edge_job_task_logs" "l" {
  edge_job_id = 1
  task_id     = "1_2"
}`

// TestEdgeJobTaskLogsResource_CollectAndClear: create asks for collection,
// destroy clears the logs.
func TestEdgeJobTaskLogsResource_CollectAndClear(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/edge_jobs/1/tasks/1_2/logs", RespondString(http.StatusNoContent, "", ""))
	mock.On("DELETE", "/edge_jobs/1/tasks/1_2/logs", RespondString(http.StatusNoContent, "", ""))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + edgeJobTaskLogsHCL,
			Check:  resource.TestCheckResourceAttr("portainer_edge_job_task_logs.l", "id", "1/1_2/logs"),
		}},
	})
	if mock.FindRequest("DELETE", "/edge_jobs/1/tasks/1_2/logs") == nil {
		t.Error("expected the logs to be cleared on destroy")
	}
}

func TestEdgeJobTaskLogsResource_Errors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/edge_jobs/1/tasks/1_2/logs", RespondString(http.StatusBadRequest, "", "no such task"))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + edgeJobTaskLogsHCL,
			ExpectError: regexp.MustCompile(`failed to request log collection for task 1_2 of edge job 1`),
		}},
	})

	// Logs already gone are fine on destroy.
	mock2 := NewMockServer(t)
	mock2.On("POST", "/edge_jobs/1/tasks/1_2/logs", RespondString(http.StatusNoContent, "", ""))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps:                    []resource.TestStep{{Config: mockProviderConfig(mock2) + edgeJobTaskLogsHCL}},
	})
}

func TestEdgeJobTaskLogs_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/edge_jobs/1/tasks/1_2/logs", RespondString(http.StatusNoContent, "", ""))
	mock.On("DELETE", "/edge_jobs/1/tasks/1_2/logs", RespondString(http.StatusNoContent, "", ""))
	config := mockProviderConfig(mock) + edgeJobTaskLogsHCL
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

func TestAccEdgeJobTaskLogsResource(t *testing.T) {
	t.Skip("collecting edge job task logs needs an edge job running on an edge environment, which a plain CE instance does not have")
}
