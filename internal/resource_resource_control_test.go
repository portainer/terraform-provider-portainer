package internal

import (
	"net/http"
	"testing"
)

// TestResourceControlCreate_DirectID verifies that when resource_control_id is
// provided directly (e.g. from docker_secret), Create skips the lookup and
// PUTs directly to /resource_controls/{id}.
func TestResourceControlCreate_DirectID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/resource_controls/100", RespondString(http.StatusOK, "application/json", `{}`))

	r := resourceResourceControl()
	d := r.TestResourceData()
	_ = d.Set("resource_control_id", 100)
	_ = d.Set("administrators_only", true)
	_ = d.Set("public", false)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "100" {
		t.Errorf("ID: got %q want %q", d.Id(), "100")
	}

	req := mock.FindRequest("PUT", "/resource_controls/100")
	if req == nil {
		t.Fatal("expected PUT /resource_controls/100")
	}
	var payload map[string]interface{}
	if err := req.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["administratorsOnly"]; got != true {
		t.Errorf("administratorsOnly: got %v", got)
	}
	if got := payload["public"]; got != false {
		t.Errorf("public: got %v", got)
	}
}

// TestResourceControlCreate_LookupByStack verifies that without
// resource_control_id, the resource looks up the stack to obtain the
// ResourceControl Id.
func TestResourceControlCreate_LookupByStack(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 42,
		"ResourceControl": map[string]interface{}{
			"Id":                 200,
			"AdministratorsOnly": false,
			"Public":             true,
		},
	}))
	mock.On("PUT", "/resource_controls/200", RespondString(http.StatusOK, "application/json", `{}`))

	r := resourceResourceControl()
	d := r.TestResourceData()
	_ = d.Set("resource_id", "42")
	_ = d.Set("type", 6)
	_ = d.Set("public", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "200" {
		t.Errorf("ID: got %q", d.Id())
	}
	if mock.FindRequest("PUT", "/resource_controls/200") == nil {
		t.Error("expected PUT /resource_controls/200 after stack lookup")
	}
}

// TestResourceControlRead_DirectID verifies that when resource_control_id is
// set, Read does not call any API and just stamps the ID.
func TestResourceControlRead_DirectID(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceResourceControl()
	d := r.TestResourceData()
	_ = d.Set("resource_control_id", 55)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "55" {
		t.Errorf("ID: got %q", d.Id())
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected no HTTP calls, got %d", len(mock.Requests()))
	}
}

// TestResourceControlDelete_HappyPath verifies DELETE is sent and ID is cleared.
func TestResourceControlDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/resource_controls/77", RespondString(http.StatusOK, "application/json", `{}`))

	r := resourceResourceControl()
	d := r.TestResourceData()
	_ = d.Set("resource_control_id", 77)
	d.SetId("77")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
	if mock.FindRequest("DELETE", "/resource_controls/77") == nil {
		t.Error("expected DELETE /resource_controls/77")
	}
}

// TestResourceControlDelete_404 verifies a 404 also clears the ID gracefully.
func TestResourceControlDelete_404(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/resource_controls/88", RespondString(http.StatusNotFound, "application/json", `{}`))

	r := resourceResourceControl()
	d := r.TestResourceData()
	_ = d.Set("resource_control_id", 88)
	d.SetId("88")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should not error on 404, got: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after 404, got %q", d.Id())
	}
}

// TestResourceControlUpdate_HTTPError verifies a non-2xx update is surfaced
// as an error.
func TestResourceControlUpdate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/resource_controls/9", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"oops"}`,
	))

	r := resourceResourceControl()
	d := r.TestResourceData()
	_ = d.Set("resource_control_id", 9)
	d.SetId("9")
	_ = d.Set("public", true)

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on 500, got nil")
	}
}
