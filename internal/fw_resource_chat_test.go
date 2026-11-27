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

func chatMock(t *testing.T) *MockServer {
	mock := NewMockServer(t)
	mock.On("POST", "/chat", RespondJSON(http.StatusOK, map[string]interface{}{
		"message": "Here is the manifest you asked for.",
		"yaml":    "apiVersion: v1\nkind: Pod\n",
	}))
	return mock
}

// TestChatResource_Lifecycle: the request payload, the stored responses and
// the ID; a changed message is stored without another request.
func TestChatResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := chatMock(t)
	cfg := func(msg string) string {
		return mockProviderConfig(mock) + `
resource "portainer_chat" "c" {
  context        = "kubernetes"
  environment_id = 7
  message        = "` + msg + `"
  model          = "gpt-4"
}
`
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: cfg("Generate a pod manifest"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_chat.c", "id", "chat-7"),
					resource.TestCheckResourceAttr("portainer_chat.c", "response_message", "Here is the manifest you asked for."),
					resource.TestCheckResourceAttr("portainer_chat.c", "response_yaml", "apiVersion: v1\nkind: Pod\n"),
					func(_ *terraform.State) error {
						req := mock.FindRequest("POST", "/chat")
						var payload map[string]interface{}
						if req == nil || req.DecodeJSON(&payload) != nil {
							return fmt.Errorf("expected a JSON POST /chat")
						}
						if payload["context"] != "kubernetes" || payload["message"] != "Generate a pod manifest" ||
							payload["model"] != "gpt-4" || payload["environmentID"] != float64(7) {
							return fmt.Errorf("unexpected payload %v", payload)
						}
						return nil
					},
				),
			},
			{
				Config: cfg("something else"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_chat.c", plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_chat.c", "message", "something else"),
					resource.TestCheckResourceAttr("portainer_chat.c", "response_message", "Here is the manifest you asked for."),
					func(_ *terraform.State) error {
						n := 0
						for _, r := range mock.Requests() {
							if r.Method == http.MethodPost {
								n++
							}
						}
						if n != 1 {
							return fmt.Errorf("an update must not send the chat again, got %d POSTs", n)
						}
						return nil
					},
				),
			},
		},
	})
}

func TestChatResource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/chat", RespondString(http.StatusBadGateway, "application/json", `{"message":"upstream LLM unavailable"}`))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_chat" "c" {` + "\n" + `context = "k8s"` + "\n" + `environment_id = 2` + "\n" + `message = "x"` + "\n}",
			ExpectError: regexp.MustCompile(`failed to send chat`),
		}},
	})
}

// TestChatResource_UpgradeFromV300: a chat applied by v3.0.0, with the model
// set and left to its default, plans no change with this build.
func TestChatResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := chatMock(t)
	config := mockProviderConfig(mock) + `
resource "portainer_chat" "default_model" {
  context        = "docker"
  environment_id = 42
  message        = "hello"
}
resource "portainer_chat" "model" {
  context        = "docker"
  environment_id = 43
  message        = "hello"
  model          = "gpt-4"
}
`
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
