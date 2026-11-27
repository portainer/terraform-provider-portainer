package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func debugLogMock(t *testing.T) (*MockServer, *fakeToggle) {
	mock := NewMockServer(t)
	f := &fakeToggle{}
	mock.On("PUT", "/support/debug_log", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		v, _ := body["debugLogEnabled"].(bool)
		f.set(v)
		w.WriteHeader(http.StatusNoContent)
	})
	mock.On("GET", "/support/debug_log", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		RespondJSON(http.StatusOK, map[string]interface{}{"debugLogEnabled": f.value})(w, r)
	})
	return mock, f
}

// TestSupportDebugLogResource_Lifecycle: the flag goes out as debugLogEnabled,
// the ID follows it, drift is read back, and destroy switches it off.
func TestSupportDebugLogResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock, fake := debugLogMock(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `resource "portainer_support_debug_log" "d" { enabled = true }`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_support_debug_log.d", "id", "true"),
					func(*terraform.State) error {
						if v := gsbBody(t, mock, "PUT", "/support/debug_log")["debugLogEnabled"]; v != true {
							return fmt.Errorf("payload.debugLogEnabled: expected true, got %v", v)
						}
						return nil
					},
				),
			},
			{
				Config: mockProviderConfig(mock) + `resource "portainer_support_debug_log" "d" { enabled = false }`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_support_debug_log.d", "id", "false"),
					func(*terraform.State) error {
						if v := gsbBody(t, mock, "PUT", "/support/debug_log")["debugLogEnabled"]; v != false {
							return fmt.Errorf("payload.debugLogEnabled: expected false, got %v", v)
						}
						return nil
					},
				),
			},
			{
				PreConfig:          func() { fake.set(true) },
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_support_debug_log.d", "enabled", "true"),
					resource.TestCheckResourceAttr("portainer_support_debug_log.d", "id", "true"),
				),
			},
		},
		CheckDestroy: func(*terraform.State) error {
			if v := gsbBody(t, mock, "PUT", "/support/debug_log")["debugLogEnabled"]; v != false {
				return fmt.Errorf("delete should send debugLogEnabled=false, got %v", v)
			}
			return nil
		},
	})
}

func TestSupportDebugLogResource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("PUT", "/support/debug_log", RespondString(http.StatusForbidden, "application/json", `{"message":"forbidden"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `resource "portainer_support_debug_log" "d" { enabled = true }`,
			ExpectError: regexp.MustCompile(`failed to set debug log`),
		}},
	})
}

func TestSupportDebugLogResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer { m, _ := debugLogMock(t); return m }
	gsbUpgradeTest(t, newMock,
		`resource "portainer_support_debug_log" "d" { enabled = true }`,
		`resource "portainer_support_debug_log" "d" { enabled = false }`,
	)
}

// TestAccSupportDebugLogResource runs against a real (BE) Portainer.
func TestAccSupportDebugLogResource(t *testing.T) {
	accRequireBE(t, "the support debug log")
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: accProvider() + `resource "portainer_support_debug_log" "d" { enabled = true }`,
				Check:  resource.TestCheckResourceAttr("portainer_support_debug_log.d", "id", "true"),
			},
			{
				Config: accProvider() + `resource "portainer_support_debug_log" "d" { enabled = false }`,
				Check:  resource.TestCheckResourceAttr("portainer_support_debug_log.d", "enabled", "false"),
			},
		},
	})
}
