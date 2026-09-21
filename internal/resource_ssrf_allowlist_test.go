package internal

import (
	"net/http"
	"strings"
	"testing"
)

// =========================================================================
// SSRF allow list and automatic updates (Business Edition).
// =========================================================================

// TestSSRFAllowList_WritesModeAsInteger pins the translation this resource
// exists to do: the configuration names the mode, Portainer stores an integer.
func TestSSRFAllowList_WritesModeAsInteger(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/allowlist/0", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/allowlist/0", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 0, "Mode": 2, "Entries": []string{"https://hooks.example.com", "https://registry.example.com"},
	}))

	r := resourceSSRFAllowList()
	d := r.TestResourceData()
	_ = d.Set("mode", "enforce")
	_ = d.Set("entries", []interface{}{"https://hooks.example.com", "https://registry.example.com"})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload struct {
		Mode    int      `json:"Mode"`
		Entries []string `json:"Entries"`
	}
	if err := mock.FindRequest("PUT", "/allowlist/0").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if payload.Mode != 2 {
		t.Errorf("mode enforce must be sent as 2, got %d", payload.Mode)
	}
	if len(payload.Entries) != 2 || payload.Entries[0] != "https://hooks.example.com" {
		t.Errorf("the entries were not sent in order: %v", payload.Entries)
	}

	if got := d.Get("mode"); got != "enforce" {
		t.Errorf("mode: expected the name to be read back, got %v", got)
	}
}

// TestSSRFAllowList_ModeNamesRoundTrip checks every mode both ways, since a
// wrong mapping would silently misreport a security control.
func TestSSRFAllowList_ModeNamesRoundTrip(t *testing.T) {
	for name, want := range map[string]int{"off": 0, "audit": 1, "enforce": 2} {
		mock := NewMockServer(t)
		mock.On("PUT", "/allowlist/0", RespondJSON(http.StatusOK, map[string]interface{}{}))
		mock.On("GET", "/allowlist/0", RespondJSON(http.StatusOK, map[string]interface{}{
			"Mode": want, "Entries": []string{},
		}))

		r := resourceSSRFAllowList()
		d := r.TestResourceData()
		_ = d.Set("mode", name)
		_ = d.Set("entries", []interface{}{})

		if err := rcCreate(r, d, mock.Client()); err != nil {
			t.Fatalf("%s: Create failed: %v", name, err)
		}
		var payload struct {
			Mode int `json:"Mode"`
		}
		if err := mock.FindRequest("PUT", "/allowlist/0").DecodeJSON(&payload); err != nil {
			t.Fatalf("%s: payload is not JSON: %v", name, err)
		}
		if payload.Mode != want {
			t.Errorf("%s: expected mode %d, got %d", name, want, payload.Mode)
		}
		if got := d.Get("mode"); got != name {
			t.Errorf("%s: expected the name back, got %v", name, got)
		}
	}
}

// TestSSRFAllowList_EmptyListIsSent covers the combination worth being
// deliberate about: an empty list under `enforce` blocks every outbound proxy
// request, so the provider must send it rather than quietly omitting it.
func TestSSRFAllowList_EmptyListIsSent(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/allowlist/0", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/allowlist/0", RespondJSON(http.StatusOK, map[string]interface{}{"Mode": 2}))

	r := resourceSSRFAllowList()
	d := r.TestResourceData()
	_ = d.Set("mode", "enforce")
	_ = d.Set("entries", []interface{}{})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("PUT", "/allowlist/0").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	entries, ok := payload["Entries"].([]interface{})
	if !ok || len(entries) != 0 {
		t.Errorf("expected an explicit empty list, got %v", payload["Entries"])
	}
}

// TestSSRFAllowList_UnknownModeIsReported guards the mapping in the other
// direction. A mode this provider does not know about must be said out loud:
// folding it onto "off" would report a control as disabled when it is not.
func TestSSRFAllowList_UnknownModeIsReported(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/allowlist/0", RespondJSON(http.StatusOK, map[string]interface{}{
		"Mode": 7, "Entries": []string{},
	}))

	r := resourceSSRFAllowList()
	d := r.TestResourceData()
	d.SetId("portainer-ssrf-allowlist")

	err := rcRead(r, d, mock.Client())
	if err == nil {
		t.Fatal("an unrecognised SSRF mode must fail the read")
	}
	if !strings.Contains(err.Error(), "unrecognised SSRF mode") {
		t.Errorf("the error should name the unknown mode, got: %v", err)
	}
}

// TestSSRFAllowList_DeleteLeavesControlInPlace pins the destroy semantics.
// There is no endpoint to remove the list, and turning a security control off
// as a side effect of no longer managing it would be the wrong failure mode.
func TestSSRFAllowList_DeleteLeavesControlInPlace(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceSSRFAllowList()
	d := r.TestResourceData()
	d.SetId("portainer-ssrf-allowlist")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("destroy must not call Portainer, got %d request(s)", len(mock.Requests()))
	}
	if d.Id() != "" {
		t.Error("the resource must be removed from state")
	}
}

// TestDataSourceAutoUpdates_ReadsHistory covers the update history, including
// a run still in flight, whose finish timestamp is not set yet.
func TestDataSourceAutoUpdates_ReadsHistory(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/auto_updates", RespondJSON(http.StatusOK, map[string]interface{}{
		"autoUpdates": []map[string]interface{}{
			{"version": "2.45.0", "status": "completed", "startedAtUnix": 1756000000, "doneAtUnix": 1756000600},
			{"version": "2.45.1", "status": "inProgress", "startedAtUnix": 1756100000, "doneAtUnix": 0},
		},
	}))

	ds := dataSourceAutoUpdates()
	d := ds.TestResourceData()

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	updates := d.Get("auto_updates").([]interface{})
	if len(updates) != 2 {
		t.Fatalf("expected two runs, got %d", len(updates))
	}
	done := updates[0].(map[string]interface{})
	if done["version"] != "2.45.0" || done["status"] != "completed" || done["done_at"] != 1756000600 {
		t.Errorf("the finished run was not read: %+v", done)
	}
	running := updates[1].(map[string]interface{})
	if running["status"] != "inProgress" || running["done_at"] != 0 {
		t.Errorf("a run still in flight must report no finish time: %+v", running)
	}
}

// TestDataSourceAutoUpdates_EmptyHistory covers an instance that has never
// auto-updated, which must read as an empty list rather than an error.
func TestDataSourceAutoUpdates_EmptyHistory(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/auto_updates", RespondJSON(http.StatusOK, map[string]interface{}{}))

	ds := dataSourceAutoUpdates()
	d := ds.TestResourceData()

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("auto_updates").([]interface{}); len(got) != 0 {
		t.Errorf("expected an empty list, got %v", got)
	}
}
