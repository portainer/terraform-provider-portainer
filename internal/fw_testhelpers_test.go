package internal

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// testProtoV5Factories runs the provider as Terraform sees it, for
// terraform-plugin-testing.
var testProtoV5Factories = map[string]func() (tfprotov5.ProviderServer, error){
	"portainer": func() (tfprotov5.ProviderServer, error) {
		return NewProviderServer(context.Background())
	},
}

// mockProviderConfig is a provider block pointing at a MockServer, for unit
// tests that run real Terraform against canned API responses.
func mockProviderConfig(m *MockServer) string {
	return fmt.Sprintf(`
provider "portainer" {
  endpoint    = %q
  api_key     = "test-api-key"
  max_retries = 0
}
`, m.URL)
}

// accProviderConfig is the provider block of the acceptance tests, which run
// against a real Portainer (TF_ACC=1, PORTAINER_ENDPOINT, PORTAINER_API_KEY).
const accProviderConfig = `
provider "portainer" {
  endpoint        = %q
  api_key         = %q
  skip_ssl_verify = true
}
`

func accProvider() string {
	return fmt.Sprintf(accProviderConfig, os.Getenv("PORTAINER_ENDPOINT"), os.Getenv("PORTAINER_API_KEY"))
}

// accIsBE reports whether the acceptance tests run against Portainer Business
// Edition (PORTAINER_ACC_BE=1).
func accIsBE() bool { return os.Getenv("PORTAINER_ACC_BE") == "1" }

// accRequireBE skips an acceptance test of a Business Edition API on a CE
// instance; what names the feature in the skip message.
func accRequireBE(t *testing.T, what string) {
	t.Helper()
	if !accIsBE() {
		t.Skip(what + " is a Portainer Business Edition API; set PORTAINER_ACC_BE=1 to run against a BE instance")
	}
}

// accPreCheck skips an acceptance test without a Portainer to run against.
// The test instances use a self-signed certificate; the variable also covers
// the post-test destroy, which terraform-plugin-testing runs without the
// provider block of an import step.
func accPreCheck(t *testing.T) {
	t.Helper()
	if os.Getenv("PORTAINER_ENDPOINT") == "" || os.Getenv("PORTAINER_API_KEY") == "" {
		t.Skip("PORTAINER_ENDPOINT and PORTAINER_API_KEY must be set for acceptance tests")
	}
	if os.Getenv("PORTAINER_SKIP_SSL_VERIFY") == "" {
		t.Setenv("PORTAINER_SKIP_SSL_VERIFY", "true")
	}
}

// previousRelease is the last release served by SDKv2 alone. Each migrated
// resource is applied with it, then planned with this build: the plan has to
// be empty, or the migration changed the state or the plan of existing users.
var previousRelease = map[string]resource.ExternalProvider{
	"portainer": {Source: "portainer/portainer", VersionConstraint: "3.0.0"},
}

// accRequireVersion skips an acceptance test on a server older than min
// (e.g. "3.1.0"); what names the feature in the skip message.
func accRequireVersion(t *testing.T, min, what string) {
	t.Helper()
	client, err := newAPIClient(context.Background(), providerConfig{
		Endpoint: os.Getenv("PORTAINER_ENDPOINT"), APIKey: os.Getenv("PORTAINER_API_KEY"), SkipSSLVerify: true,
	})
	if err != nil {
		t.Fatalf("acceptance client: %v", err)
	}
	var version struct {
		ServerVersion string `json:"ServerVersion"`
	}
	if err := doJSON(context.Background(), client, "GET", client.Endpoint+"/system/version", nil, &version); err != nil {
		t.Fatalf("reading the server version: %v", err)
	}
	if versionLess(version.ServerVersion, min) {
		t.Skipf("%s needs Portainer %s or later; the server is %s", what, min, version.ServerVersion)
	}
}
