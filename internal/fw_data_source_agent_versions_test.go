package internal

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

func TestAgentVersionsDataSource_Sorts(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/agent_versions", RespondJSON(http.StatusOK, []string{"2.21.0", "2.19.0", "2.20.0"}))
	const ds = "data.portainer_agent_versions.v"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_agent_versions" "v" {}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(ds, "id", "portainer-agent-versions"),
				resource.TestCheckResourceAttr(ds, "versions.#", "3"),
				resource.TestCheckResourceAttr(ds, "versions.0", "2.19.0"),
				resource.TestCheckResourceAttr(ds, "versions.2", "2.21.0"),
			),
		}},
	})
}

// TestAccAgentVersionsDataSource reads the agent versions of a real Portainer.
func TestAccAgentVersionsDataSource(t *testing.T) {
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: accProvider() + `data "portainer_agent_versions" "v" {}`,
			Check:  resource.TestCheckResourceAttr("data.portainer_agent_versions.v", "id", "portainer-agent-versions"),
		}},
	})
}
