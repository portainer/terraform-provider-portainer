package internal

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"testing"

	httptransport "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	portainer "github.com/portainer/client-api-go/v2/pkg/client"
)

// MockServer wraps httptest.Server, records every incoming request, and routes
// requests to handlers registered via On().
//
// Usage:
//
//	mock := NewMockServer(t)
//	mock.On("POST", "/cloud/gitcredentials", RespondJSON(200, map[string]any{...}))
//	client := mock.Client()
//	// ... drive the resource under test ...
//	reqs := mock.Requests()  // inspect what the resource sent
//
// The mock auto-closes via t.Cleanup. Any request that does not match a
// registered route returns 404 with a descriptive body — that makes test
// failures actionable.
type MockServer struct {
	*httptest.Server

	mu       sync.Mutex
	requests []*RecordedRequest
	routes   []route
}

// RecordedRequest captures the request a resource sent to the mock.
// Body is the raw request body bytes; Headers is the cloned header map.
type RecordedRequest struct {
	Method  string
	Path    string
	Query   string
	Body    []byte
	Headers http.Header
}

type route struct {
	method  string
	path    string
	handler http.HandlerFunc
}

// NewMockServer starts an httptest.Server and registers cleanup with t.
func NewMockServer(t *testing.T) *MockServer {
	t.Helper()
	m := &MockServer{}
	m.Server = httptest.NewServer(http.HandlerFunc(m.dispatch))
	t.Cleanup(m.Close)
	return m
}

// On registers a handler for a given method+path combination.
// path is matched exactly. If you need prefix matching, write a custom
// handler and call mock.Server.Handler directly.
func (m *MockServer) On(method, path string, handler http.HandlerFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.routes = append(m.routes, route{method: method, path: path, handler: handler})
}

// Requests returns a copy of all recorded requests in arrival order.
func (m *MockServer) Requests() []*RecordedRequest {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]*RecordedRequest, len(m.requests))
	copy(out, m.requests)
	return out
}

// Client returns an *APIClient pointed at the mock server with a dummy API key.
//
// Two flavors of resources coexist in this provider:
//
//  1. Direct-HTTP resources call client.DoRequest("POST", "/cloud/...", ...).
//     The Endpoint here is m.URL (no "/api" suffix), so the final URL has no
//     prefix and the dispatcher sees paths like "/cloud/gitcredentials".
//
//  2. SDK-based resources call client.Client.Tags.TagCreate(...) which routes
//     through a swagger transport with basePath="/api", so the dispatcher
//     sees "/api/tags".
//
// To let tests register handlers with one consistent path style, the
// dispatcher strips a leading "/api" before matching. Tests always write
// mock.On("POST", "/tags", ...) regardless of which flavor the resource uses.
func (m *MockServer) Client() *APIClient {
	u, _ := url.Parse(m.URL)
	sdkTransport := httptransport.New(u.Host, "/api", []string{u.Scheme})
	authInfo := httptransport.APIKeyAuth("X-API-Key", "header", "test-api-key")
	sdkTransport.DefaultAuthentication = authInfo

	return &APIClient{
		Endpoint:   m.URL,
		APIKey:     "test-api-key",
		HTTPClient: http.Client{},
		Client:     portainer.New(sdkTransport, strfmt.Default),
		AuthInfo:   authInfo,
	}
}

func (m *MockServer) dispatch(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	_ = r.Body.Close()

	// Normalize: strip a leading "/api" so tests can register handlers with
	// the same paths regardless of whether the resource uses direct HTTP
	// (no /api prefix) or the SDK transport (which adds it).
	path := r.URL.Path
	switch {
	case strings.HasPrefix(path, "/api/"):
		path = path[len("/api"):]
	case path == "/api":
		path = "/"
	}

	rec := &RecordedRequest{
		Method:  r.Method,
		Path:    path,
		Query:   r.URL.RawQuery,
		Body:    body,
		Headers: r.Header.Clone(),
	}
	m.mu.Lock()
	m.requests = append(m.requests, rec)
	routes := append([]route(nil), m.routes...)
	m.mu.Unlock()

	// Restore the body so the matched handler can re-read it if needed.
	r.Body = io.NopCloser(bytes.NewReader(body))

	for _, rt := range routes {
		if rt.method == r.Method && rt.path == path {
			rt.handler(w, r)
			return
		}
	}

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(http.StatusNotFound)
	_, _ = w.Write([]byte("MockServer: no route registered for " + r.Method + " " + path))
}

// RespondJSON returns a handler that writes the given status code and JSON body.
// body is marshaled with encoding/json — pass map[string]any or any struct.
func RespondJSON(status int, body interface{}) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(body)
	}
}

// RespondString returns a handler that writes the given status code and raw body.
// Useful for testing error paths where the server returns plain text.
func RespondString(status int, contentType, body string) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		if contentType != "" {
			w.Header().Set("Content-Type", contentType)
		}
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}
}

// FindRequest returns the first recorded request matching method+path, or nil.
func (m *MockServer) FindRequest(method, path string) *RecordedRequest {
	for _, r := range m.Requests() {
		if r.Method == method && r.Path == path {
			return r
		}
	}
	return nil
}

// DecodeJSON parses the recorded request body as JSON into v.
// Returns an error if the body is empty or invalid JSON.
func (r *RecordedRequest) DecodeJSON(v interface{}) error {
	if len(r.Body) == 0 {
		return io.EOF
	}
	return json.NewDecoder(strings.NewReader(string(r.Body))).Decode(v)
}
