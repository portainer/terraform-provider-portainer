package internal

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"sync"
	"testing"
)

// dkFake is a small Portainer stand-in for the Docker and stack resources:
// handlers are matched by method and a path pattern (with "/api" stripped),
// and every request is recorded.
type dkFake struct {
	srv    *httptest.Server
	mu     sync.Mutex
	reqs   []*RecordedRequest
	routes []dkRoute
}

type dkRoute struct {
	method  string
	pattern *regexp.Regexp
	handler func(w http.ResponseWriter, r *http.Request, body []byte, m []string)
}

func newDkFake(t *testing.T) *dkFake {
	t.Helper()
	f := &dkFake{}
	f.srv = httptest.NewServer(http.HandlerFunc(f.dispatch))
	t.Cleanup(f.srv.Close)
	return f
}

// on registers a handler for method and a path pattern such as
// `/stacks/(\d+)`; m holds the submatches.
func (f *dkFake) on(method, pattern string, h func(w http.ResponseWriter, r *http.Request, body []byte, m []string)) {
	f.mu.Lock()
	defer f.mu.Unlock()
	// Later registrations win, so a test can override a default handler.
	f.routes = append([]dkRoute{{method: method, pattern: regexp.MustCompile("^" + pattern + "$"), handler: h}}, f.routes...)
}

// json registers a fixed JSON answer.
func (f *dkFake) json(method, pattern string, status int, body interface{}) {
	f.on(method, pattern, func(w http.ResponseWriter, _ *http.Request, _ []byte, _ []string) {
		dkWriteJSON(w, status, body)
	})
}

func dkWriteJSON(w http.ResponseWriter, status int, body interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

func (f *dkFake) dispatch(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	p := strings.TrimPrefix(r.URL.Path, "/api")
	f.mu.Lock()
	f.reqs = append(f.reqs, &RecordedRequest{Method: r.Method, Path: p, Query: r.URL.RawQuery, Body: body, Headers: r.Header.Clone()})
	routes := append([]dkRoute(nil), f.routes...)
	f.mu.Unlock()
	for _, rt := range routes {
		if rt.method != r.Method {
			continue
		}
		if m := rt.pattern.FindStringSubmatch(p); m != nil {
			rt.handler(w, r, body, m)
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
	_, _ = fmt.Fprintf(w, "dkFake: no route for %s %s", r.Method, p)
}

// requests returns the recorded requests matching method and an exact path.
func (f *dkFake) requests(method, path string) []*RecordedRequest {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []*RecordedRequest
	for _, r := range f.reqs {
		if r.Method == method && r.Path == path {
			out = append(out, r)
		}
	}
	return out
}

// last returns the last request matching method and path, failing the test
// without one.
func (f *dkFake) last(t *testing.T, method, path string) *RecordedRequest {
	t.Helper()
	reqs := f.requests(method, path)
	if len(reqs) == 0 {
		t.Fatalf("no %s %s was sent", method, path)
	}
	return reqs[len(reqs)-1]
}

// body decodes a recorded request body.
func dkBody(t *testing.T, r *RecordedRequest) map[string]interface{} {
	t.Helper()
	var out map[string]interface{}
	if err := json.Unmarshal(r.Body, &out); err != nil {
		t.Fatalf("decoding the body of %s %s: %v (%s)", r.Method, r.Path, err, r.Body)
	}
	return out
}

// provider is the provider block pointing at the fake.
func (f *dkFake) provider() string {
	return fmt.Sprintf(`
provider "portainer" {
  endpoint    = %q
  api_key     = "test-api-key"
  max_retries = 0
}
`, f.srv.URL)
}
