package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

type fakeSSRF struct {
	mu   sync.Mutex
	list map[string]interface{}
}

func ssrfMock(t *testing.T) (*MockServer, *fakeSSRF) {
	mock := NewMockServer(t)
	f := &fakeSSRF{list: map[string]interface{}{"Entries": []string{}, "Mode": 0}}
	mock.On("PUT", "/allowlist/0", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.list = body
		f.mu.Unlock()
		RespondJSON(http.StatusOK, body)(w, r)
	})
	mock.On("GET", "/allowlist/0", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		RespondJSON(http.StatusOK, f.list)(w, r)
	})
	return mock, f
}

// TestSSRFAllowListResource_ModesRoundTrip pins the translation of the mode
// names onto Portainer's integers, both ways, and that the entries go out in
// order.
func TestSSRFAllowListResource_ModesRoundTrip(t *testing.T) {
	t.Parallel()
	for name, want := range map[string]float64{"off": 0, "audit": 1, "enforce": 2} {
		t.Run(name, func(t *testing.T) {
			mock, _ := ssrfMock(t)
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{{
					Config: mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_ssrf_allowlist" "s" {
  mode    = %q
  entries = ["https://a.example.com", "https://b.example.com"]
}`, name),
					Check: resource.ComposeTestCheckFunc(
						resource.TestCheckResourceAttr("portainer_ssrf_allowlist.s", "mode", name),
						resource.TestCheckResourceAttr("portainer_ssrf_allowlist.s", "id", "portainer-ssrf-allowlist"),
						func(*terraform.State) error {
							body := gsbBody(t, mock, "PUT", "/allowlist/0")
							if body["Mode"] != want {
								return fmt.Errorf("mode %s must be sent as %v, got %v", name, want, body["Mode"])
							}
							if fmt.Sprint(body["Entries"]) != "[https://a.example.com https://b.example.com]" {
								return fmt.Errorf("the entries were not sent in order: %v", body["Entries"])
							}
							return nil
						},
					),
				}},
			})
		})
	}
}

// TestSSRFAllowListResource_EmptyListIsSent: an empty list with enforce
// blocks everything, so it is sent explicitly rather than omitted; destroy
// leaves the control in place.
func TestSSRFAllowListResource_EmptyListIsSent(t *testing.T) {
	t.Parallel()
	mock, _ := ssrfMock(t)
	config := mockProviderConfig(mock) + `
resource "portainer_ssrf_allowlist" "s" {
  mode    = "enforce"
  entries = []
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: func(*terraform.State) error {
					entries, ok := gsbBody(t, mock, "PUT", "/allowlist/0")["Entries"].([]interface{})
					if !ok || len(entries) != 0 {
						return fmt.Errorf("expected an explicit empty list")
					}
					return nil
				},
			},
			{ResourceName: "portainer_ssrf_allowlist.s", ImportState: true, ImportStateId: "portainer-ssrf-allowlist", ImportStateVerify: true},
		},
		CheckDestroy: func(*terraform.State) error {
			if n := gsbCount(mock, "PUT", "/allowlist/0"); n != 1 {
				return fmt.Errorf("destroy must not call Portainer, got %d updates", n)
			}
			return nil
		},
	})
}

// TestSSRFAllowListResource_UnknownModeIsReported: silently mapping an unknown
// mode onto "off" would misreport a security control.
func TestSSRFAllowListResource_UnknownModeIsReported(t *testing.T) {
	t.Parallel()
	mock, f := ssrfMock(t)
	config := mockProviderConfig(mock) + `
resource "portainer_ssrf_allowlist" "s" {
  mode    = "audit"
  entries = ["https://a.example.com"]
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					f.mu.Lock()
					f.list = map[string]interface{}{"Entries": []string{}, "Mode": 7}
					f.mu.Unlock()
				},
				Config:      config,
				ExpectError: regexp.MustCompile(`unrecognised SSRF mode 7`),
			},
			{
				PreConfig: func() {
					f.mu.Lock()
					f.list = map[string]interface{}{"Entries": []string{}, "Mode": 1}
					f.mu.Unlock()
				},
				Config: config,
			},
		},
	})
}

func TestSSRFAllowListResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer { m, _ := ssrfMock(t); return m }
	gsbUpgradeTest(t, newMock,
		`resource "portainer_ssrf_allowlist" "s" {
  mode    = "audit"
  entries = ["https://a.example.com"]
}`,
		`resource "portainer_ssrf_allowlist" "s" {
  mode    = "off"
  entries = []
}`,
	)
}
