package internal

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// portainer_tls uploads a TLS file (ca/cert/key) as multipart/form-data to
// /upload/tls/{certType}. Read and Update keep the state and destroy only
// forgets, so only the upload is exercised. The ID is
// "upload-{certType}-{basename}".

func tlsConfig(certType, filePath string) string {
	return fmt.Sprintf(`
resource "portainer_tls" "t" {
  certificate = %q
  folder      = "endpoint-7"
  file_path   = %q
}`, certType, filePath)
}

func writeTLSFile(t *testing.T, name, body string) string {
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(body), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return p
}

// TestUploadTLSResource_Multipart: the folder field and the file part reach
// Portainer with the API key.
func TestUploadTLSResource_Multipart(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/upload/tls/ca", RespondString(http.StatusNoContent, "", ""))
	filePath := writeTLSFile(t, "ca.pem", "----CA-CERT----")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + tlsConfig("ca", filePath),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_tls.t", "id", "upload-ca-ca.pem"),
				func(*terraform.State) error {
					req := mock.FindRequest("POST", "/upload/tls/ca")
					if req == nil {
						return fmt.Errorf("expected POST /upload/tls/ca to be sent")
					}
					if got := req.Headers.Get("X-API-Key"); got != "test-api-key" {
						return fmt.Errorf("X-API-Key header: got %q", got)
					}
					_, params, err := mime.ParseMediaType(req.Headers.Get("Content-Type"))
					if err != nil {
						return err
					}
					mr := multipart.NewReader(bytes.NewReader(req.Body), params["boundary"])
					seen := map[string]string{}
					for {
						part, err := mr.NextPart()
						if err == io.EOF {
							break
						}
						if err != nil {
							return fmt.Errorf("multipart parse error: %w", err)
						}
						b, _ := io.ReadAll(part)
						seen[part.FormName()] = string(b)
						if part.FormName() == "file" && part.FileName() != "ca.pem" {
							return fmt.Errorf("file part filename: got %q", part.FileName())
						}
					}
					if seen["folder"] != "endpoint-7" || seen["file"] != "----CA-CERT----" {
						return fmt.Errorf("multipart body mismatch: %v", seen)
					}
					return nil
				},
			),
		}},
	})
}

// TestUploadTLSResource_CertTypes: the URL path is derived from certificate.
func TestUploadTLSResource_CertTypes(t *testing.T) {
	t.Parallel()
	for _, certType := range []string{"ca", "cert", "key"} {
		t.Run(certType, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("POST", "/upload/tls/"+certType, RespondString(http.StatusNoContent, "", ""))
			filePath := writeTLSFile(t, certType+".pem", "x")
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{{
					Config: mockProviderConfig(mock) + tlsConfig(certType, filePath),
					Check:  resource.TestCheckResourceAttr("portainer_tls.t", "id", "upload-"+certType+"-"+certType+".pem"),
				}},
			})
		})
	}
}

func TestUploadTLSResource_HTTPError(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/upload/tls/cert", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad file"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + tlsConfig("cert", writeTLSFile(t, "c.pem", "x")),
			ExpectError: regexp.MustCompile(`upload failed`),
		}},
	})
}

// TestUploadTLSResource_NoAuth: without an API key or a JWT the upload fails
// with a clear error.
func TestUploadTLSResource_NoAuth(t *testing.T) {
	t.Setenv("PORTAINER_API_KEY", "")
	t.Setenv("PORTAINER_USER", "")
	t.Setenv("PORTAINER_PASSWORD", "")
	mock := NewMockServer(t)
	mock.On("POST", "/upload/tls/cert", RespondString(http.StatusNoContent, "", ""))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      fmt.Sprintf("provider \"portainer\" {\n  endpoint = %q\n  max_retries = 0\n}\n", mock.URL) + tlsConfig("cert", writeTLSFile(t, "c.pem", "x")),
			ExpectError: regexp.MustCompile(`(?i)authentication`),
		}},
	})
}

// TestUploadTLSResource_UpdateKeepsState: a changed folder is recorded
// without another upload, as before.
func TestUploadTLSResource_UpdateKeepsState(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/upload/tls/ca", RespondString(http.StatusNoContent, "", ""))
	filePath := writeTLSFile(t, "ca.pem", "x")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + tlsConfig("ca", filePath)},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_tls" "t" {
  certificate = "ca"
  folder      = "endpoint-8"
  file_path   = %q
}`, filePath),
				Check: func(*terraform.State) error {
					if n := gsbCount(mock, "POST", "/upload/tls/ca"); n != 1 {
						return fmt.Errorf("expected a single upload, got %d", n)
					}
					return nil
				},
			},
		},
	})
}

func TestUploadTLSResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	filePath := writeTLSFile(t, "ca.pem", "x")
	newMock := func(t *testing.T) *MockServer {
		m := NewMockServer(t)
		m.On("POST", "/upload/tls/ca", RespondString(http.StatusNoContent, "", ""))
		return m
	}
	gsbUpgradeTest(t, newMock, tlsConfig("ca", filePath))
}
