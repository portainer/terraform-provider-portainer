package internal

import (
	"net/http"
	"testing"
)

// TestSupportDebugLogCreate_HappyPath_Enabled verifies that Create issues a
// PUT to /support/debug_log with debugLogEnabled=true and sets the resource
// ID to the stringified boolean.
func TestSupportDebugLogCreate_HappyPath_Enabled(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/support/debug_log", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	r := resourcePortainerSupportDebugLog()
	d := r.TestResourceData()
	_ = d.Set("enabled", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "true" {
		t.Errorf("expected ID %q, got %q", "true", d.Id())
	}

	req := mock.FindRequest("PUT", "/support/debug_log")
	if req == nil {
		t.Fatal("expected PUT /support/debug_log")
	}
	var payload map[string]interface{}
	if err := req.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["debugLogEnabled"]; got != true {
		t.Errorf("payload.debugLogEnabled: expected true, got %v", got)
	}
}

// TestSupportDebugLogCreate_HappyPath_Disabled verifies the false branch:
// the same endpoint and payload key with debugLogEnabled=false produces
// ID "false".
func TestSupportDebugLogCreate_HappyPath_Disabled(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/support/debug_log", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	r := resourcePortainerSupportDebugLog()
	d := r.TestResourceData()
	_ = d.Set("enabled", false)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "false" {
		t.Errorf("expected ID %q, got %q", "false", d.Id())
	}

	req := mock.FindRequest("PUT", "/support/debug_log")
	if req == nil {
		t.Fatal("expected PUT /support/debug_log")
	}
	var payload map[string]interface{}
	if err := req.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["debugLogEnabled"]; got != false {
		t.Errorf("payload.debugLogEnabled: expected false, got %v", got)
	}
}

// TestSupportDebugLogCreate_HTTPError verifies that a 4xx/5xx is surfaced
// as a Go error and the resource ID stays empty.
func TestSupportDebugLogCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/support/debug_log", RespondString(
		http.StatusForbidden, "application/json",
		`{"message":"admin only"}`,
	))

	r := resourcePortainerSupportDebugLog()
	d := r.TestResourceData()
	_ = d.Set("enabled", true)

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error on 403, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestSupportDebugLogRead_PopulatesState verifies Read decodes the response
// into the enabled field and sets the ID accordingly.
func TestSupportDebugLogRead_PopulatesState(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/support/debug_log", RespondJSON(http.StatusOK, map[string]interface{}{
		"debugLogEnabled": true,
	}))

	r := resourcePortainerSupportDebugLog()
	d := r.TestResourceData()
	d.SetId("false") // simulating a drift: state says false, API says true

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("enabled"); got != true {
		t.Errorf("enabled: expected true, got %v", got)
	}
	if d.Id() != "true" {
		t.Errorf("expected ID refreshed to %q, got %q", "true", d.Id())
	}
}

// TestSupportDebugLogDelete_DisablesViaPUT verifies that Delete is implemented
// as a PUT with debugLogEnabled=false (the API has no DELETE for this action).
func TestSupportDebugLogDelete_DisablesViaPUT(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/support/debug_log", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	r := resourcePortainerSupportDebugLog()
	d := r.TestResourceData()
	d.SetId("true")
	_ = d.Set("enabled", true)

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	req := mock.FindRequest("PUT", "/support/debug_log")
	if req == nil {
		t.Fatal("expected PUT /support/debug_log on Delete")
	}
	var payload map[string]interface{}
	if err := req.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["debugLogEnabled"]; got != false {
		t.Errorf("Delete should send debugLogEnabled=false, got %v", got)
	}
}
