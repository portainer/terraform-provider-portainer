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

func sslMock(t *testing.T) (*MockServer, *sync.Mutex, *bool) {
	mock := NewMockServer(t)
	var mu sync.Mutex
	httpEnabled := false
	mock.On("PUT", "/ssl", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		mu.Lock()
		httpEnabled, _ = body["httpenabled"].(bool)
		mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	mock.On("GET", "/ssl", func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		RespondJSON(http.StatusOK, map[string]interface{}{"httpEnabled": httpEnabled})(w, r)
	})
	return mock, &mu, &httpEnabled
}

func sslConfig(cert string, httpEnabled bool, extra string) string {
	return fmt.Sprintf(`
resource "portainer_ssl" "s" {
  cert         = %q
  key          = "----KEY----"
  http_enabled = %t
  %s
}`, cert, httpEnabled, extra)
}

// TestSSLResource_Lifecycle: PUT /ssl carries the certificate and
// http_enabled (false included: Portainer keeps the current setting when it
// is missing); an update sends the new values; destroy only forgets.
func TestSSLResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock, _, _ := sslMock(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + sslConfig("----CERT----", true, ""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_ssl.s", "id", "portainer-ssl"),
					func(*terraform.State) error {
						body := gsbBody(t, mock, "PUT", "/ssl")
						if body["cert"] != "----CERT----" || body["key"] != "----KEY----" || body["httpenabled"] != true {
							return fmt.Errorf("payload mismatch: %v", body)
						}
						if _, ok := body["clientCert"]; ok {
							return fmt.Errorf("an unset client_cert must be omitted")
						}
						return nil
					},
				),
			},
			{
				Config: mockProviderConfig(mock) + sslConfig("----NEW-CERT----", false, `client_cert = "----CLIENT----"`),
				Check: func(*terraform.State) error {
					body := gsbBody(t, mock, "PUT", "/ssl")
					if v, ok := body["httpenabled"]; !ok || v != false {
						return fmt.Errorf("payload.httpenabled: expected false, got %v (present=%v)", v, ok)
					}
					if body["cert"] != "----NEW-CERT----" || body["clientCert"] != "----CLIENT----" {
						return fmt.Errorf("payload mismatch: %v", body)
					}
					return nil
				},
			},
		},
		CheckDestroy: func(*terraform.State) error {
			if n := gsbCount(mock, "PUT", "/ssl"); n != 2 {
				return fmt.Errorf("destroy must not call Portainer, got %d updates", n)
			}
			return nil
		},
	})
}

// TestSSLResource_ReadDrift: http_enabled is read back from GET /ssl.
func TestSSLResource_ReadDrift(t *testing.T) {
	t.Parallel()
	mock, mu, httpEnabled := sslMock(t)
	config := mockProviderConfig(mock) + sslConfig("c", false, "")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					mu.Lock()
					*httpEnabled = true
					mu.Unlock()
				},
				RefreshState:       true,
				ExpectNonEmptyPlan: true,
				Check:              resource.TestCheckResourceAttr("portainer_ssl.s", "http_enabled", "true"),
			},
		},
	})
}

func TestSSLResource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("PUT", "/ssl", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad cert"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + sslConfig("c", true, ""),
			ExpectError: regexp.MustCompile(`failed to update SSL settings`),
		}},
	})
}

func TestSSLResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer { m, _, _ := sslMock(t); return m }
	gsbUpgradeTest(t, newMock,
		sslConfig("c", true, ""),
		sslConfig("c", false, `client_cert = "cc"`),
	)
}
