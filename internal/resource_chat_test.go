package internal

import (
	"net/http"
	"testing"
)

// TestChatCreate_HappyPath verifies that resourcePortainerChat POSTs the
// message payload to /chat, parses the AI response, and writes the response
// fields plus a stable ID derived from environment_id into state.
func TestChatCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/chat", RespondJSON(http.StatusOK, map[string]interface{}{
		"message": "Here is the manifest you asked for.",
		"yaml":    "apiVersion: v1\nkind: Pod\n",
	}))

	r := resourcePortainerChat()
	d := r.TestResourceData()
	_ = d.Set("context", "kubernetes")
	_ = d.Set("environment_id", 7)
	_ = d.Set("message", "Generate a pod manifest")
	_ = d.Set("model", "gpt-4")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "chat-7" {
		t.Errorf("expected ID %q, got %q", "chat-7", d.Id())
	}
	if got := d.Get("response_message"); got != "Here is the manifest you asked for." {
		t.Errorf("response_message: got %v", got)
	}
	if got := d.Get("response_yaml"); got != "apiVersion: v1\nkind: Pod\n" {
		t.Errorf("response_yaml: got %v", got)
	}

	req := mock.FindRequest("POST", "/chat")
	if req == nil {
		t.Fatal("expected POST /chat")
	}
	var payload map[string]interface{}
	if err := req.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if got := payload["context"]; got != "kubernetes" {
		t.Errorf("payload.context: got %v", got)
	}
	if got := payload["message"]; got != "Generate a pod manifest" {
		t.Errorf("payload.message: got %v", got)
	}
	if got := payload["model"]; got != "gpt-4" {
		t.Errorf("payload.model: got %v", got)
	}
	// JSON numbers decode as float64.
	if got := payload["environmentID"]; got != float64(7) {
		t.Errorf("payload.environmentID: expected 7, got %v", got)
	}
}

// TestChatCreate_IDFromEnvironment verifies the deterministic ID format
// embeds the environment_id, which lets Terraform distinguish chat actions
// per environment when the same chat resource block is reused.
func TestChatCreate_IDFromEnvironment(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/chat", RespondJSON(http.StatusOK, map[string]interface{}{
		"message": "ok",
		"yaml":    "",
	}))

	r := resourcePortainerChat()
	d := r.TestResourceData()
	_ = d.Set("context", "docker")
	_ = d.Set("environment_id", 42)
	_ = d.Set("message", "hello")
	_ = d.Set("model", "gpt-3.5-turbo")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "chat-42" {
		t.Errorf("expected ID %q, got %q", "chat-42", d.Id())
	}
}

// TestChatCreate_HTTPError verifies that an error response surfaces as a
// Go error and leaves the resource ID empty.
func TestChatCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/chat", RespondString(
		http.StatusBadGateway, "application/json",
		`{"message":"upstream LLM unavailable"}`,
	))

	r := resourcePortainerChat()
	d := r.TestResourceData()
	_ = d.Set("context", "k8s")
	_ = d.Set("environment_id", 2)
	_ = d.Set("message", "x")

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error on 502, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
