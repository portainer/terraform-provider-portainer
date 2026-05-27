package internal

import (
	"net/http"
	"testing"
)

// TestOpenAMTCreate_HappyPath exercises the Create path against a mock
// server. Verifies that the POST is sent to a relative "/open_amt" path
// (and not to "Endpoint+Endpoint+/open_amt" — a previous bug where the
// resource manually prefixed client.Endpoint in addition to DoRequest's
// own prefixing).
func TestOpenAMTCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/open_amt", RespondString(http.StatusNoContent, "", ""))

	r := resourceOpenAMT()
	d := r.TestResourceData()
	_ = d.Set("cert_file_content", "PEMBLOCK")
	_ = d.Set("cert_file_name", "cert.pem")
	_ = d.Set("cert_file_password", "secret")
	_ = d.Set("domain_name", "example.org")
	_ = d.Set("enabled", true)
	_ = d.Set("mpspassword", "mpspw")
	_ = d.Set("mpsserver", "https://mps.example.org")
	_ = d.Set("mpsuser", "admin")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "openamt-enabled" {
		t.Errorf("expected ID %q, got %q", "openamt-enabled", d.Id())
	}

	post := mock.FindRequest("POST", "/open_amt")
	if post == nil {
		t.Fatal("expected POST /open_amt to be recorded")
	}

	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["domainName"] != "example.org" {
		t.Errorf("domainName: expected example.org, got %v", payload["domainName"])
	}
	if payload["enabled"] != true {
		t.Errorf("enabled: expected true, got %v", payload["enabled"])
	}
	if payload["mpsserver"] != "https://mps.example.org" {
		t.Errorf("mpsserver: expected mps URL, got %v", payload["mpsserver"])
	}
}

// TestOpenAMTCreate_HTTPError verifies that a non-204 response surfaces
// as an error.
func TestOpenAMTCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/open_amt", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"invalid configuration"}`,
	))

	r := resourceOpenAMT()
	d := r.TestResourceData()
	_ = d.Set("enabled", true)

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestOpenAMTRead_Noop verifies Read is a no-op.
func TestOpenAMTRead_Noop(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceOpenAMT()
	d := r.TestResourceData()
	d.SetId("openamt-enabled")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read should be a no-op, got error: %v", err)
	}
}

// TestOpenAMTDelete_ClearsID verifies Delete simply clears the ID locally
// (no remote call).
func TestOpenAMTDelete_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceOpenAMT()
	d := r.TestResourceData()
	d.SetId("openamt-enabled")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected Delete to make no HTTP calls, got %d", len(mock.Requests()))
	}
}
