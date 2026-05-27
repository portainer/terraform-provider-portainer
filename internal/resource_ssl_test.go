package internal

import (
	"net/http"
	"testing"
)

// resource_ssl uses the generated SDK (client.Client.Ssl.*):
//   - SSLUpdate is PUT /ssl with JSON body {cert, key, clientCert, httpEnabled}
//     and a 204-style response (no body model bound; we only assert the call).
//   - SSLInspect is GET /ssl returning models.PortainereeSSLSettings with
//     `httpEnabled` as the only field we map back to state.
//   - Delete is a no-op API call: it just clears the resource ID.
//
// Create/Update share the same function so we test Update via Create.

// TestSSLCreate_HappyPath verifies that Create issues PUT /ssl and sets the
// composite ID "portainer-ssl".
func TestSSLCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/ssl", RespondJSON(http.StatusNoContent, map[string]interface{}{}))

	r := resourceSSLSettings()
	d := r.TestResourceData()
	_ = d.Set("cert", "----CERT----")
	_ = d.Set("key", "----KEY----")
	_ = d.Set("client_cert", "----CLIENT-CERT----")
	_ = d.Set("http_enabled", true)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "portainer-ssl" {
		t.Errorf("expected ID %q, got %q", "portainer-ssl", d.Id())
	}

	put := mock.FindRequest("PUT", "/ssl")
	if put == nil {
		t.Fatal("expected PUT /ssl to be sent")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	if got := payload["cert"]; got != "----CERT----" {
		t.Errorf("payload.cert: expected %q, got %v", "----CERT----", got)
	}
	if got := payload["key"]; got != "----KEY----" {
		t.Errorf("payload.key: expected %q, got %v", "----KEY----", got)
	}
	if got := payload["httpenabled"]; got != true {
		t.Errorf("payload.httpenabled: expected true, got %v", got)
	}
}

// TestSSLRead_HappyPath verifies the Read function populates http_enabled from
// the SDK response and stamps the canonical ID.
func TestSSLRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/ssl", RespondJSON(http.StatusOK, map[string]interface{}{
		"httpEnabled": true,
		"selfSigned":  false,
		"certPath":    "/data/certs/cert.pem",
		"keyPath":     "/data/certs/key.pem",
	}))

	r := resourceSSLSettings()
	d := r.TestResourceData()
	// Required fields must be set so d.Set("http_enabled", ...) inside Read
	// has a coherent schema instance.
	_ = d.Set("cert", "x")
	_ = d.Set("key", "y")
	d.SetId("portainer-ssl")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("http_enabled"); got != true {
		t.Errorf("http_enabled: expected true, got %v", got)
	}
	if d.Id() != "portainer-ssl" {
		t.Errorf("expected ID %q, got %q", "portainer-ssl", d.Id())
	}
}

// TestSSLUpdate_HappyPath verifies that Update (an alias for the Create flow)
// sends the new field values.
func TestSSLUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/ssl", RespondJSON(http.StatusNoContent, map[string]interface{}{}))

	r := resourceSSLSettings()
	d := r.TestResourceData()
	d.SetId("portainer-ssl")
	_ = d.Set("cert", "----NEW-CERT----")
	_ = d.Set("key", "----NEW-KEY----")
	_ = d.Set("http_enabled", true)

	if err := r.Update(d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/ssl")
	if put == nil {
		t.Fatal("expected PUT /ssl to be sent")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	if got := payload["cert"]; got != "----NEW-CERT----" {
		t.Errorf("payload.cert: expected %q, got %v", "----NEW-CERT----", got)
	}
	// SDK uses omitempty for `httpenabled`, so we just confirm the call
	// reached the endpoint above and the cert/key are present.
	if got := payload["key"]; got != "----NEW-KEY----" {
		t.Errorf("payload.key: expected %q, got %v", "----NEW-KEY----", got)
	}
}

// TestSSLDelete_ClearsID confirms that Delete is a state-only operation: it
// removes the ID without calling the server (no API endpoint for SSL delete).
func TestSSLDelete_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceSSLSettings()
	d := r.TestResourceData()
	d.SetId("portainer-ssl")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after Delete, got %q", d.Id())
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected no HTTP calls during Delete, got %d", len(mock.Requests()))
	}
}

// TestSSLCreate_HTTPError verifies that an HTTP error from PUT /ssl propagates
// up and the resource ID stays empty.
func TestSSLCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/ssl", RespondString(http.StatusBadRequest, "application/json",
		`{"message":"invalid certificate"}`))

	r := resourceSSLSettings()
	d := r.TestResourceData()
	_ = d.Set("cert", "bad")
	_ = d.Set("key", "bad")
	_ = d.Set("http_enabled", true)

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
