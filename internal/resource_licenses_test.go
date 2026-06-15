package internal

import (
	"crypto/sha256"
	"fmt"
	"net/http"
	"testing"
)

// resource_licenses (Portainer BE-only) uses client.DoRequest:
//   - Create POSTs /licenses/add (optionally with ?force=true) and decodes
//     {"conflictingKeys":[...]}. The resource ID is sha256(key) hex-encoded.
//   - Read GETs /licenses and clears the ID when the configured key is no
//     longer present in the response array.
//   - Delete POSTs /licenses/remove with {"licenseKeys":[...]}.

const testLicenseKey = "PORTAINER-LICENSE-XXX"

func sha256Hex(s string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(s)))
}

// TestLicensesCreate_HappyPath verifies the POST body, the sha256-based ID,
// and that an empty conflictingKeys list is propagated to state.
func TestLicensesCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/licenses/add", RespondJSON(http.StatusOK, map[string]interface{}{
		"conflictingKeys": []string{},
	}))

	r := resourceLicenses()
	d := r.TestResourceData()
	_ = d.Set("key", testLicenseKey)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	want := sha256Hex(testLicenseKey)
	if d.Id() != want {
		t.Errorf("expected ID %q (sha256 of key), got %q", want, d.Id())
	}

	post := mock.FindRequest("POST", "/licenses/add")
	if post == nil {
		t.Fatal("expected POST /licenses/add to be sent")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode POST body: %v", err)
	}
	if got := payload["key"]; got != testLicenseKey {
		t.Errorf("payload.key: expected %q, got %v", testLicenseKey, got)
	}
	// No force => no ?force=true on the query string.
	if post.Query != "" {
		t.Errorf("expected empty query, got %q", post.Query)
	}
}

// TestLicensesCreate_ForceFlag verifies that force=true is reflected in the
// query string of the create request.
func TestLicensesCreate_ForceFlag(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/licenses/add", RespondJSON(http.StatusOK, map[string]interface{}{
		"conflictingKeys": []string{"OTHER-KEY"},
	}))

	r := resourceLicenses()
	d := r.TestResourceData()
	_ = d.Set("key", testLicenseKey)
	_ = d.Set("force", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	post := mock.FindRequest("POST", "/licenses/add")
	if post == nil {
		t.Fatal("expected POST /licenses/add to be sent")
	}
	if post.Query != "force=true" {
		t.Errorf("expected query %q, got %q", "force=true", post.Query)
	}

	conflicts := d.Get("conflicting_keys").([]interface{})
	if len(conflicts) != 1 || conflicts[0] != "OTHER-KEY" {
		t.Errorf("conflicting_keys: expected [OTHER-KEY], got %v", conflicts)
	}
}

// TestLicensesRead_KeyPresent verifies Read keeps the ID when the configured
// key is in the returned list.
func TestLicensesRead_KeyPresent(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/licenses", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"licenseKey": testLicenseKey},
		{"licenseKey": "ANOTHER-KEY"},
	}))

	r := resourceLicenses()
	d := r.TestResourceData()
	_ = d.Set("key", testLicenseKey)
	d.SetId(sha256Hex(testLicenseKey))

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() == "" {
		t.Error("expected ID to remain set when key is present in /licenses")
	}
}

// TestLicensesRead_KeyAbsent_ClearsID covers drift detection: when our key
// is gone from /licenses, the resource ID must be cleared.
func TestLicensesRead_KeyAbsent_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/licenses", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"licenseKey": "OTHER-KEY-ONLY"},
	}))

	r := resourceLicenses()
	d := r.TestResourceData()
	_ = d.Set("key", testLicenseKey)
	d.SetId(sha256Hex(testLicenseKey))

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared when key is absent, got %q", d.Id())
	}
}

// TestLicensesDelete_HappyPath verifies the remove POST payload.
func TestLicensesDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/licenses/remove", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceLicenses()
	d := r.TestResourceData()
	_ = d.Set("key", testLicenseKey)
	d.SetId(sha256Hex(testLicenseKey))

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	post := mock.FindRequest("POST", "/licenses/remove")
	if post == nil {
		t.Fatal("expected POST /licenses/remove to be sent")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode POST body: %v", err)
	}
	keys, ok := payload["licenseKeys"].([]interface{})
	if !ok || len(keys) != 1 || keys[0] != testLicenseKey {
		t.Errorf("payload.licenseKeys: expected [%q], got %v", testLicenseKey, payload["licenseKeys"])
	}

	if d.Id() != "" {
		t.Errorf("expected ID cleared after delete, got %q", d.Id())
	}
}

// TestLicensesCreate_HTTPError verifies error propagation.
func TestLicensesCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/licenses/add", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"invalid license"}`,
	))

	r := resourceLicenses()
	d := r.TestResourceData()
	_ = d.Set("key", "BAD")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
