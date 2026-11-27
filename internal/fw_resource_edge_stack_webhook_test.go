package internal

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

func TestEdgeStackWebhookResource_TriggersOnCreate(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusNoContent, http.StatusOK} {
		mock := NewMockServer(t)
		mock.On("POST", "/edge_stacks/webhooks/abc-123", RespondString(status, "", ""))
		mock.On("POST", "/edge_stacks/webhooks/def-456", RespondString(status, "", ""))

		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{
				{
					Config: mockProviderConfig(mock) + `resource "portainer_edge_stack_webhook" "w" { webhook_id = "abc-123" }`,
					Check:  resource.TestCheckResourceAttr("portainer_edge_stack_webhook.w", "id", "abc-123"),
				},
				{
					Config: mockProviderConfig(mock) + `resource "portainer_edge_stack_webhook" "w" { webhook_id = "def-456" }`,
					ConfigPlanChecks: resource.ConfigPlanChecks{
						PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_edge_stack_webhook.w", plancheck.ResourceActionDestroyBeforeCreate)},
					},
				},
			},
		})
		// Read and delete call nothing: two triggers, nothing else.
		if n := len(mock.Requests()); n != 2 {
			t.Errorf("expected only the two webhook triggers, got %d requests", n)
		}
	}
}

func TestEdgeStackWebhookResource_Error(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/edge_stacks/webhooks/abc", RespondString(http.StatusNotFound, "", "webhook not found"))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_edge_stack_webhook" "w" { webhook_id = "abc" }`,
			ExpectError: regexp.MustCompile(`failed to trigger webhook`),
		}},
	})
}

func TestEdgeStackWebhook_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/edge_stacks/webhooks/abc-123", RespondString(http.StatusNoContent, "", ""))
	config := mockProviderConfig(mock) + `resource "portainer_edge_stack_webhook" "w" { webhook_id = "abc-123" }`
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

func TestAccEdgeStackWebhookResource(t *testing.T) {
	t.Skip("triggering an edge stack webhook needs an edge stack with GitOps updates and edge environments, which a plain CE instance does not have")
}
