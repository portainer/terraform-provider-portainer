package internal

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

const testLicenseKey = "2-LICENSE-KEY"

func sha256Hex(s string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(s))) }

// fakeLicenses keeps the attached keys.
type fakeLicenses struct {
	mu        sync.Mutex
	keys      []string
	conflicts []string
}

func (f *fakeLicenses) register(m *MockServer) {
	m.On("POST", "/licenses/add", func(w http.ResponseWriter, r *http.Request) {
		var body LicensePayload
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.keys = append(f.keys, body.Key)
		RespondJSON(http.StatusOK, LicenseResponse{ConflictingKeys: f.conflicts})(w, r)
	})
	m.On("GET", "/licenses", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		out := make([]map[string]string, 0, len(f.keys))
		for _, k := range f.keys {
			out = append(out, map[string]string{"licenseKey": k})
		}
		RespondJSON(http.StatusOK, out)(w, r)
	})
	m.On("POST", "/licenses/remove", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			LicenseKeys []string `json:"licenseKeys"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		kept := []string{}
		for _, k := range f.keys {
			if len(body.LicenseKeys) == 0 || k != body.LicenseKeys[0] {
				kept = append(kept, k)
			}
		}
		f.keys = kept
		RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
	})
}

func (f *fakeLicenses) clear() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.keys = nil
}

// TestLicensesResource_Lifecycle: the POST body, the SHA-256 ID, the removal
// payload on destroy, and import by ID.
func TestLicensesResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := &fakeLicenses{}
	fake.register(mock)
	config := mockProviderConfig(mock) + fmt.Sprintf(`resource "portainer_licenses" "l" { key = %q }`, testLicenseKey)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_licenses.l", "id", sha256Hex(testLicenseKey)),
					resource.TestCheckResourceAttr("portainer_licenses.l", "force", "false"),
					resource.TestCheckResourceAttr("portainer_licenses.l", "conflicting_keys.#", "0"),
					func(*terraform.State) error {
						if k := gsbBody(t, mock, "POST", "/licenses/add")["key"]; k != testLicenseKey {
							return fmt.Errorf("payload.key: got %v", k)
						}
						if q := mock.FindRequest("POST", "/licenses/add").Query; q != "" {
							return fmt.Errorf("expected empty query, got %q", q)
						}
						return nil
					},
				),
			},
			{
				ResourceName:            "portainer_licenses.l",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"key"},
			},
		},
		CheckDestroy: func(*terraform.State) error {
			body := gsbBody(t, mock, "POST", "/licenses/remove")
			if fmt.Sprint(body["licenseKeys"]) != "["+testLicenseKey+"]" {
				return fmt.Errorf("payload.licenseKeys: got %v", body["licenseKeys"])
			}
			return nil
		},
	})
}

// TestLicensesResource_ForceAndConflicts: force=true is a query parameter and
// the conflicting keys are reported.
func TestLicensesResource_ForceAndConflicts(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := &fakeLicenses{conflicts: []string{"OTHER-KEY"}}
	fake.register(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_licenses" "l" {
  key   = %q
  force = true
}`, testLicenseKey),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_licenses.l", "conflicting_keys.0", "OTHER-KEY"),
				func(*terraform.State) error {
					if q := mock.FindRequest("POST", "/licenses/add").Query; q != "force=true" {
						return fmt.Errorf("expected query force=true, got %q", q)
					}
					return nil
				},
			),
		}},
	})
}

// TestLicensesResource_RemovedOutsideTerraform: a license no longer attached
// is planned for creation again, and destroying it sends no removal.
func TestLicensesResource_RemovedOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := &fakeLicenses{}
	fake.register(mock)
	config := mockProviderConfig(mock) + fmt.Sprintf(`resource "portainer_licenses" "l" { key = %q }`, testLicenseKey)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{PreConfig: fake.clear, Config: config, PlanOnly: true, ExpectNonEmptyPlan: true},
			{PreConfig: fake.clear, Config: mockProviderConfig(mock), Check: func(*terraform.State) error {
				if gsbCount(mock, "POST", "/licenses/remove") != 0 {
					return fmt.Errorf("expected no removal of a license that is not attached")
				}
				return nil
			}},
		},
	})
}

// TestLicensesResource_WriteOnlyKey: key_wo reaches Portainer, the ID is its
// SHA-256 and key stays out of state; the read goes by the ID.
func TestLicensesResource_WriteOnlyKey(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := &fakeLicenses{}
	fake.register(mock)
	config := mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_licenses" "l" {
  key_wo         = %q
  key_wo_version = 1
}`, testLicenseKey)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_licenses.l", "id", sha256Hex(testLicenseKey)),
					resource.TestCheckNoResourceAttr("portainer_licenses.l", "key"),
					func(*terraform.State) error {
						if k := gsbBody(t, mock, "POST", "/licenses/add")["key"]; k != testLicenseKey {
							return fmt.Errorf("key: expected the write-only value, got %v", k)
						}
						return nil
					},
				),
			},
			{Config: config, PlanOnly: true},
		},
	})
}

func TestLicensesResource_Errors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/licenses/add", RespondString(http.StatusBadRequest, "application/json", `{"message":"invalid key"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + `resource "portainer_licenses" "l" { key = "bad" }`, ExpectError: regexp.MustCompile(`failed to attach license`)},
			{Config: mockProviderConfig(mock) + `resource "portainer_licenses" "l" {}`, ExpectError: regexp.MustCompile(`Missing Attribute Configuration|Invalid Attribute Combination`)},
		},
	})
}

func TestLicensesResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer {
		m := NewMockServer(t)
		(&fakeLicenses{}).register(m)
		return m
	}
	gsbUpgradeTest(t, newMock,
		fmt.Sprintf(`resource "portainer_licenses" "l" { key = %q }`, testLicenseKey),
		fmt.Sprintf(`
resource "portainer_licenses" "l" {
  key_wo         = %q
  key_wo_version = 1
  force          = true
}`, testLicenseKey),
	)
}
