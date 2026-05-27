package internal

import (
	"bytes"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// resource_tls uploads a TLS file (ca/cert/key) via multipart/form-data POST
// to /upload/tls/{certType}. Read and Update are schema.Noop and Delete is
// schema.RemoveFromState (state-only) — so only Create is exercised here.
//
// The composite ID encodes the certificate type and basename:
//   "upload-{certType}-{basename}".
//
// Not covered (would expand the test surface without commensurate value):
//   - Reading the uploaded file contents back; we only assert the field is
//     present in the multipart body. Verifying byte equality would require
//     re-implementing the producer side.
//   - Error paths where os.Open fails (e.g. missing file). The behavior is
//     a simple wrapped error; tested implicitly by relying on t.TempDir().

// extractMultipartBoundary pulls the boundary= token out of a recorded
// Content-Type header so the test can parse the body the resource sent.
func extractMultipartBoundary(t *testing.T, ct string) string {
	t.Helper()
	const prefix = "boundary="
	idx := strings.Index(ct, prefix)
	if idx == -1 {
		t.Fatalf("Content-Type %q has no boundary parameter", ct)
	}
	return ct[idx+len(prefix):]
}

// TestTLSUploadCreate_HappyPath writes a temp file, runs Create, and verifies
// the resulting multipart body carries the expected fields and the resource
// ID encodes the certificate type and filename.
func TestTLSUploadCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/upload/tls/ca", RespondJSON(http.StatusOK, map[string]interface{}{}))

	dir := t.TempDir()
	filePath := filepath.Join(dir, "ca.pem")
	if err := os.WriteFile(filePath, []byte("----CA-CERT----"), 0o600); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	r := resourcePortainerUploadTLS()
	d := r.TestResourceData()
	_ = d.Set("certificate", "ca")
	_ = d.Set("folder", "endpoint-7")
	_ = d.Set("file_path", filePath)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	wantID := "upload-ca-ca.pem"
	if d.Id() != wantID {
		t.Errorf("expected ID %q, got %q", wantID, d.Id())
	}

	post := mock.FindRequest("POST", "/upload/tls/ca")
	if post == nil {
		t.Fatal("expected POST /upload/tls/ca to be sent")
	}

	// Confirm the auth header was set.
	if got := post.Headers.Get("X-API-Key"); got != "test-api-key" {
		t.Errorf("X-API-Key header: expected %q, got %q", "test-api-key", got)
	}

	// Parse the multipart body and assert form fields and the file part.
	boundary := extractMultipartBoundary(t, post.Headers.Get("Content-Type"))
	mr := multipart.NewReader(bytes.NewReader(post.Body), boundary)
	sawFolder := false
	sawFile := false
	for {
		part, err := mr.NextPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("multipart parse error: %v", err)
		}
		switch part.FormName() {
		case "folder":
			b, _ := io.ReadAll(part)
			if string(b) != "endpoint-7" {
				t.Errorf("folder field: expected %q, got %q", "endpoint-7", string(b))
			}
			sawFolder = true
		case "file":
			if part.FileName() != "ca.pem" {
				t.Errorf("file part filename: expected %q, got %q", "ca.pem", part.FileName())
			}
			b, _ := io.ReadAll(part)
			if string(b) != "----CA-CERT----" {
				t.Errorf("file part contents: expected %q, got %q", "----CA-CERT----", string(b))
			}
			sawFile = true
		}
	}
	if !sawFolder {
		t.Error("expected multipart body to contain 'folder' field")
	}
	if !sawFile {
		t.Error("expected multipart body to contain 'file' part")
	}
}

// TestTLSUploadCreate_DifferentCertTypes ensures the URL path is derived from
// the certificate type (cert, key, ca).
func TestTLSUploadCreate_DifferentCertTypes(t *testing.T) {
	for _, certType := range []string{"cert", "key", "ca"} {
		t.Run(certType, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("POST", "/upload/tls/"+certType, RespondJSON(http.StatusOK, map[string]interface{}{}))

			dir := t.TempDir()
			filePath := filepath.Join(dir, certType+".pem")
			if err := os.WriteFile(filePath, []byte("body"), 0o600); err != nil {
				t.Fatalf("write temp file: %v", err)
			}

			r := resourcePortainerUploadTLS()
			d := r.TestResourceData()
			_ = d.Set("certificate", certType)
			_ = d.Set("folder", "endpoint-1")
			_ = d.Set("file_path", filePath)

			if err := r.Create(d, mock.Client()); err != nil {
				t.Fatalf("Create failed: %v", err)
			}
			if mock.FindRequest("POST", "/upload/tls/"+certType) == nil {
				t.Errorf("expected POST /upload/tls/%s to be sent", certType)
			}
		})
	}
}

// TestTLSUploadCreate_HTTPError verifies a 4xx response surfaces as an error
// and the resource ID stays empty.
func TestTLSUploadCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/upload/tls/cert", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"invalid TLS data"}`,
	))

	dir := t.TempDir()
	filePath := filepath.Join(dir, "cert.pem")
	if err := os.WriteFile(filePath, []byte("body"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	r := resourcePortainerUploadTLS()
	d := r.TestResourceData()
	_ = d.Set("certificate", "cert")
	_ = d.Set("folder", "endpoint-1")
	_ = d.Set("file_path", filePath)

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestTLSUploadCreate_NoAuth confirms the create returns a clear error when
// the client has neither an API key nor a JWT.
func TestTLSUploadCreate_NoAuth(t *testing.T) {
	mock := NewMockServer(t)
	client := mock.Client()
	client.APIKey = ""
	client.JWTToken = ""

	dir := t.TempDir()
	filePath := filepath.Join(dir, "cert.pem")
	if err := os.WriteFile(filePath, []byte("body"), 0o600); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	r := resourcePortainerUploadTLS()
	d := r.TestResourceData()
	_ = d.Set("certificate", "cert")
	_ = d.Set("folder", "endpoint-1")
	_ = d.Set("file_path", filePath)

	err := r.Create(d, client)
	if err == nil {
		t.Fatal("expected an authentication error, got nil")
	}
	if !strings.Contains(err.Error(), "authentication") {
		t.Errorf("expected error to mention authentication, got %v", err)
	}
}
