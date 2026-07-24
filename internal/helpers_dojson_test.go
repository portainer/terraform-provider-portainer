package internal

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

// TestDoJSON_PostWithBodyDecodesResponse covers the common write path: a JSON
// body is marshaled and sent with Content-Type + auth header, and the response
// is decoded into out.
func TestDoJSON_PostWithBodyDecodesResponse(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/things", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":   7,
		"name": "created",
	}))

	var out struct {
		ID   int    `json:"id"`
		Name string `json:"name"`
	}
	body := map[string]interface{}{"name": "created"}
	err := doJSON(context.Background(), mock.Client(), http.MethodPost, mock.Client().Endpoint+"/things", body, &out)
	if err != nil {
		t.Fatalf("doJSON returned error: %v", err)
	}
	if out.ID != 7 || out.Name != "created" {
		t.Errorf("response not decoded: %+v", out)
	}

	req := mock.FindRequest("POST", "/things")
	if req == nil {
		t.Fatal("expected POST /things to be recorded")
	}
	if got := req.Headers.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type: expected application/json, got %q", got)
	}
	if got := req.Headers.Get("X-API-Key"); got != "test-api-key" {
		t.Errorf("expected auth header to be set, got %q", got)
	}
	var sent map[string]interface{}
	if err := json.Unmarshal(req.Body, &sent); err != nil {
		t.Fatalf("request body not valid JSON: %v", err)
	}
	if sent["name"] != "created" {
		t.Errorf("request body not marshaled correctly: %v", sent)
	}
}

// TestDoJSON_GetNoBody covers a read with no request body and no Content-Type.
func TestDoJSON_GetNoBody(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/things/1", RespondJSON(http.StatusOK, map[string]interface{}{"name": "x"}))

	var out map[string]interface{}
	if err := doJSON(context.Background(), mock.Client(), http.MethodGet, mock.Client().Endpoint+"/things/1", nil, &out); err != nil {
		t.Fatalf("doJSON returned error: %v", err)
	}
	if out["name"] != "x" {
		t.Errorf("decode failed: %v", out)
	}
	req := mock.FindRequest("GET", "/things/1")
	if req == nil {
		t.Fatal("expected GET /things/1 to be recorded")
	}
	if got := req.Headers.Get("Content-Type"); got != "" {
		t.Errorf("expected no Content-Type on bodyless request, got %q", got)
	}
}

// TestDoJSON_NilOutSkipsDecode covers an action-style call where the caller does
// not care about the response payload.
func TestDoJSON_NilOutSkipsDecode(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/things/1/associate", RespondJSON(http.StatusOK, map[string]interface{}{"ignored": true}))

	if err := doJSON(context.Background(), mock.Client(), http.MethodPut, mock.Client().Endpoint+"/things/1/associate", nil, nil); err != nil {
		t.Fatalf("doJSON returned error: %v", err)
	}
	if mock.FindRequest("PUT", "/things/1/associate") == nil {
		t.Fatal("expected PUT to be recorded")
	}
}

// TestDoJSON_ErrorStatusIncludesBody covers the >= 400 path: the error must
// carry the status code and the raw response body for diagnostics.
func TestDoJSON_ErrorStatusIncludesBody(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/things", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad input"}`))

	var out map[string]interface{}
	err := doJSON(context.Background(), mock.Client(), http.MethodPost, mock.Client().Endpoint+"/things", map[string]interface{}{"x": 1}, &out)
	if err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if !strings.Contains(err.Error(), "400") {
		t.Errorf("error should include status code: %v", err)
	}
	if !strings.Contains(err.Error(), "bad input") {
		t.Errorf("error should include response body: %v", err)
	}
}

// TestDoJSON_NoAuthConfigured covers the guard that fails before any HTTP call
// when neither an API key nor a JWT token is configured.
func TestDoJSON_NoAuthConfigured(t *testing.T) {
	client := &APIClient{
		Endpoint:   "http://127.0.0.1:0",
		HTTPClient: http.Client{},
	}
	err := doJSON(context.Background(), client, http.MethodGet, client.Endpoint+"/things", nil, nil)
	if err == nil {
		t.Fatal("expected error when no auth is configured, got nil")
	}
	if !strings.Contains(err.Error(), "authentication") {
		t.Errorf("expected authentication error, got %v", err)
	}
}
