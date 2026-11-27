package internal

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestAutoUpdatesDataSource covers the update history, including a run still
// in flight, whose finish timestamp is not set yet.
func TestAutoUpdatesDataSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/auto_updates", RespondJSON(http.StatusOK, map[string]interface{}{
		"autoUpdates": []map[string]interface{}{
			{"version": "2.45.0", "status": "completed", "startedAtUnix": 1756000000, "doneAtUnix": 1756000600},
			{"version": "2.45.1", "status": "inProgress", "startedAtUnix": 1756100000, "doneAtUnix": 0},
		},
	}))
	const ds = "data.portainer_auto_updates.u"
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_auto_updates" "u" {}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(ds, "id", "portainer-auto-updates"),
				resource.TestCheckResourceAttr(ds, "auto_updates.#", "2"),
				resource.TestCheckResourceAttr(ds, "auto_updates.0.version", "2.45.0"),
				resource.TestCheckResourceAttr(ds, "auto_updates.0.status", "completed"),
				resource.TestCheckResourceAttr(ds, "auto_updates.0.done_at", "1756000600"),
				resource.TestCheckResourceAttr(ds, "auto_updates.1.status", "inProgress"),
				resource.TestCheckResourceAttr(ds, "auto_updates.1.done_at", "0"),
			),
		}},
	})
}

// TestAutoUpdatesDataSource_EmptyHistory: an instance that never
// auto-updated reads as an empty list rather than an error.
func TestAutoUpdatesDataSource_EmptyHistory(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/auto_updates", RespondJSON(http.StatusOK, map[string]interface{}{}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_auto_updates" "u" {}`,
			Check:  resource.TestCheckResourceAttr("data.portainer_auto_updates.u", "auto_updates.#", "0"),
		}},
	})
}
