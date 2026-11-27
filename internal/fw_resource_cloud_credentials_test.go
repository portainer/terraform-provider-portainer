package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakeCloudCredentials stores one credential (ID 18) and masks the keys in
// masked on read, as Portainer does with secrets.
type fakeCloudCredentials struct {
	mu     sync.Mutex
	cred   map[string]interface{}
	masked map[string]bool
}

func (f *fakeCloudCredentials) register(m *MockServer) {
	m.On("POST", "/cloud/credentials", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.cred = body
		f.mu.Unlock()
		RespondJSON(http.StatusOK, map[string]interface{}{"id": 18})(w, r)
	})
	m.On("PUT", "/cloud/credentials/18", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.cred = body
		f.mu.Unlock()
		RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
	})
	m.On("GET", "/cloud/credentials/18", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.cred == nil {
			RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`)(w, r)
			return
		}
		creds := map[string]interface{}{}
		for k, v := range f.cred["credentials"].(map[string]interface{}) {
			if f.masked[k] {
				v = "******"
			}
			creds[k] = v
		}
		RespondJSON(http.StatusOK, map[string]interface{}{"id": 18, "name": f.cred["name"], "provider": f.cred["provider"], "credentials": creds})(w, r)
	})
	m.On("DELETE", "/cloud/credentials/18", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.cred = nil
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
}

func cloudCredentialsConfig(name, access string) string {
	return fmt.Sprintf(`
resource "portainer_cloud_credentials" "c" {
  cloud_provider = "aws"
  name           = %q
  credentials = {
    accessKeyId     = %q
    secretAccessKey = "SECRET"
  }
}`, name, access)
}

// TestCloudCredentialsResource_Lifecycle: the POST and PUT payloads (in the
// body, never as headers), masked secrets keeping the configured value, a
// changed plain value showing as drift, import and delete.
func TestCloudCredentialsResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := &fakeCloudCredentials{masked: map[string]bool{"secretAccessKey": true}}
	fake.register(mock)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + cloudCredentialsConfig("ci-aws", "AKIA"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_cloud_credentials.c", "id", "18"),
					func(*terraform.State) error {
						body := gsbBody(t, mock, "POST", "/cloud/credentials")
						creds, _ := body["credentials"].(map[string]interface{})
						if body["provider"] != "aws" || body["name"] != "ci-aws" || creds["accessKeyId"] != "AKIA" {
							return fmt.Errorf("payload mismatch: %v", body)
						}
						return nil
					},
				),
			},
			{
				Config: mockProviderConfig(mock) + cloudCredentialsConfig("prod-aws", "AKIA-PROD"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_cloud_credentials.c", "credentials.secretAccessKey", "SECRET"),
					func(*terraform.State) error {
						req := mock.FindRequest("PUT", "/cloud/credentials/18")
						for h := range req.Headers {
							if strings.Contains(strings.ToLower(h), "credential") {
								return fmt.Errorf("credentials must not be sent as a request header")
							}
						}
						body := gsbBody(t, mock, "PUT", "/cloud/credentials/18")
						if body["name"] != "prod-aws" {
							return fmt.Errorf("unexpected PUT body: %v", body)
						}
						return nil
					},
				),
			},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.cred["credentials"].(map[string]interface{})["accessKeyId"] = "CHANGED"
					fake.mu.Unlock()
				},
				Config:             mockProviderConfig(mock) + cloudCredentialsConfig("prod-aws", "AKIA-PROD"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{Config: mockProviderConfig(mock) + cloudCredentialsConfig("prod-aws", "AKIA-PROD")},
			{
				ResourceName:            "portainer_cloud_credentials.c",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"credentials"},
			},
		},
		CheckDestroy: func(*terraform.State) error {
			if gsbCount(mock, "DELETE", "/cloud/credentials/18") != 1 {
				return fmt.Errorf("expected DELETE /cloud/credentials/18")
			}
			return nil
		},
	})
}

// TestCloudCredentialsResource_GoneOutsideTerraform: a 404 drops it from
// state (planned again), and deleting one already gone succeeds.
func TestCloudCredentialsResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := &fakeCloudCredentials{}
	fake.register(mock)
	config := mockProviderConfig(mock) + cloudCredentialsConfig("ci-aws", "AKIA")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.cred = nil
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestCloudCredentialsResource_Errors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/cloud/credentials", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + cloudCredentialsConfig("x", "y"), ExpectError: regexp.MustCompile(`failed to create cloud credential`)},
			{Config: mockProviderConfig(mock) + cloudCredentialsConfig("", "y"), ExpectError: regexp.MustCompile(`at least 1`)},
		},
	})
}

func TestCloudCredentialsResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer {
		m := NewMockServer(t)
		(&fakeCloudCredentials{masked: map[string]bool{"secretAccessKey": true}}).register(m)
		return m
	}
	gsbUpgradeTest(t, newMock, cloudCredentialsConfig("ci-aws", "AKIA"))
}
