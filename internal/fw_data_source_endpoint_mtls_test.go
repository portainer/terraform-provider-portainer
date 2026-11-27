package internal

import (
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// TestEndpointMTLSCertificateDataSources_ReadBothEndpoints covers the pair:
// the certificate an agent connects with, and the one whose rejection
// explains why it cannot.
func TestEndpointMTLSCertificateDataSources_ReadBothEndpoints(t *testing.T) {
	t.Parallel()
	cert := map[string]interface{}{
		"Subject":            map[string]interface{}{"CommonName": "agent-12", "Organization": []string{"acme"}},
		"Issuer":             map[string]interface{}{"CommonName": "portainer-edge-ca"},
		"SHA256Fingerprint":  "de:ad",
		"ValidNotAfter":      "2027-01-01T00:00:00Z",
		"SignatureAlgorithm": "SHA256-RSA",
		"PublicKey":          map[string]interface{}{"Algorithm": "ECDSA", "Size": 256},
	}
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/12/mtls_certificate", RespondJSON(http.StatusOK, map[string]interface{}{"MTLSCertificate": cert}))
	mock.On("GET", "/endpoints/12/mtls_certificate_error", RespondJSON(http.StatusOK, map[string]interface{}{"MTLSCertificate": cert}))

	checks := make([]resource.TestCheckFunc, 0, 14)
	checks = append(checks,
		resource.TestCheckResourceAttr("data.portainer_endpoint_mtls_certificate.a", "id", "portainer-endpoint-mtls-certificate-12"),
		resource.TestCheckResourceAttr("data.portainer_endpoint_mtls_certificate_error.r", "id", "portainer-endpoint-mtls-certificate-error-12"),
	)
	for _, name := range []string{"data.portainer_endpoint_mtls_certificate.a", "data.portainer_endpoint_mtls_certificate_error.r"} {
		checks = append(checks,
			resource.TestCheckResourceAttr(name, "configured", "true"),
			resource.TestCheckResourceAttr(name, "common_name", "agent-12"),
			resource.TestCheckResourceAttr(name, "organization.0", "acme"),
			resource.TestCheckResourceAttr(name, "issuer_common_name", "portainer-edge-ca"),
			resource.TestCheckResourceAttr(name, "public_key_size", "256"),
			resource.TestCheckResourceAttr(name, "key_usages.#", "0"),
		)
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
data "portainer_endpoint_mtls_certificate" "a" { endpoint_id = 12 }
data "portainer_endpoint_mtls_certificate_error" "r" { endpoint_id = 12 }
`,
			Check: resource.ComposeTestCheckFunc(checks...),
		}},
	})
}

// TestEndpointMTLSCertificateDataSource_NoneHeld covers an environment that
// never presented a certificate (an empty answer or a 404), which is the
// normal state without edge mTLS.
func TestEndpointMTLSCertificateDataSource_NoneHeld(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/12/mtls_certificate", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/endpoints/12/mtls_certificate_error", RespondJSON(http.StatusNotFound, map[string]string{"message": "not found"}))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
data "portainer_endpoint_mtls_certificate" "a" { endpoint_id = 12 }
data "portainer_endpoint_mtls_certificate_error" "r" { endpoint_id = 12 }
`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("data.portainer_endpoint_mtls_certificate.a", "configured", "false"),
				resource.TestCheckResourceAttr("data.portainer_endpoint_mtls_certificate.a", "common_name", ""),
				resource.TestCheckResourceAttr("data.portainer_endpoint_mtls_certificate_error.r", "configured", "false"),
			),
		}},
	})
}

func TestEndpointMTLSCertificateDataSource_Error(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/12/mtls_certificate_error", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + `data "portainer_endpoint_mtls_certificate_error" "r" { endpoint_id = 12 }`,
			ExpectError: regexp.MustCompile(`failed to read the rejected mTLS certificate of environment 12`),
		}},
	})
}
