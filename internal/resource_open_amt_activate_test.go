package internal

import (
	"net/http"
	"testing"
)

// TestOpenAMTActivateCreate_HappyPath verifies POST to
// /open_amt/{envID}/activate succeeds and sets the synthetic ID
// "openamt-<envID>". Read/Update are Noop, Delete is RemoveFromState.
func TestOpenAMTActivateCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/open_amt/5/activate", RespondString(http.StatusNoContent, "", ""))

	r := resourcePortainerOpenAMTActivate()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 5)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "openamt-5" {
		t.Errorf("expected ID %q, got %q", "openamt-5", d.Id())
	}
	post := mock.FindRequest("POST", "/open_amt/5/activate")
	if post == nil {
		t.Fatal("expected POST recorded")
	}
	// No body expected on activate.
	if len(post.Body) != 0 {
		t.Errorf("expected empty body, got %q", string(post.Body))
	}
}

// TestOpenAMTActivateCreate_HTTPError verifies 4xx surfaces as error.
func TestOpenAMTActivateCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/open_amt/5/activate",
		RespondString(http.StatusServiceUnavailable, "application/json", `{"message":"MPS unreachable"}`))

	r := resourcePortainerOpenAMTActivate()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 5)

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 503, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
