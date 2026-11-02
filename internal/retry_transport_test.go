package internal

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"
)

func fastRetries(t *testing.T) {
	t.Helper()
	old := retryBaseDelay
	retryBaseDelay = time.Millisecond
	t.Cleanup(func() { retryBaseDelay = old })
}

// statusSequence answers with the given status codes in turn, then 200, and
// records each request body.
func statusSequence(t *testing.T, codes ...int) (*httptest.Server, *int32, *[]string) {
	t.Helper()
	var calls int32
	var bodies []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		b, _ := io.ReadAll(r.Body)
		bodies = append(bodies, string(b))
		if int(n) <= len(codes) {
			w.WriteHeader(codes[n-1])
			_, _ = w.Write([]byte("busy"))
			return
		}
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)
	return srv, &calls, &bodies
}

func doRequest(t *testing.T, rt http.RoundTripper, method, url, body string) (*http.Response, error) {
	t.Helper()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req, err := http.NewRequestWithContext(context.Background(), method, url, r)
	if err != nil {
		t.Fatal(err)
	}
	return (&http.Client{Transport: rt}).Do(req)
}

func TestRetryTransport_RetriesStatusCodes(t *testing.T) {
	fastRetries(t)
	cases := []struct {
		method    string
		status    int
		wantCalls int32
	}{
		{http.MethodGet, http.StatusServiceUnavailable, 3},
		{http.MethodGet, http.StatusBadGateway, 3},
		{http.MethodGet, http.StatusGatewayTimeout, 3},
		{http.MethodGet, http.StatusTooManyRequests, 3},
		{http.MethodPut, http.StatusServiceUnavailable, 3},
		{http.MethodDelete, http.StatusTooManyRequests, 3},
		// Portainer may have acted on these already.
		{http.MethodPut, http.StatusBadGateway, 1},
		{http.MethodDelete, http.StatusGatewayTimeout, 1},
		{http.MethodPost, http.StatusServiceUnavailable, 1},
		// Not transient.
		{http.MethodGet, http.StatusInternalServerError, 1},
		{http.MethodGet, http.StatusNotFound, 1},
	}
	for _, tc := range cases {
		t.Run(fmt.Sprintf("%s_%d", tc.method, tc.status), func(t *testing.T) {
			srv, calls, _ := statusSequence(t, tc.status, tc.status)
			resp, err := doRequest(t, &retryTransport{next: http.DefaultTransport, maxRetries: 3}, tc.method, srv.URL, "")
			if err != nil {
				t.Fatalf("request failed: %v", err)
			}
			resp.Body.Close()
			if got := atomic.LoadInt32(calls); got != tc.wantCalls {
				t.Errorf("expected %d calls, got %d", tc.wantCalls, got)
			}
			if tc.wantCalls == 3 && resp.StatusCode != http.StatusOK {
				t.Errorf("expected the final 200, got %d", resp.StatusCode)
			}
		})
	}
}

func TestRetryTransport_StopsAfterMaxRetries(t *testing.T) {
	fastRetries(t)
	srv, calls, _ := statusSequence(t, 503, 503, 503, 503, 503)
	resp, err := doRequest(t, &retryTransport{next: http.DefaultTransport, maxRetries: 2}, http.MethodGet, srv.URL, "")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if resp.StatusCode != http.StatusServiceUnavailable || string(body) != "busy" {
		t.Errorf("expected the last 503 with its body, got %d %q", resp.StatusCode, body)
	}
	if got := atomic.LoadInt32(calls); got != 3 {
		t.Errorf("expected 1 call + 2 retries, got %d", got)
	}
}

func TestRetryTransport_ZeroRetriesDisables(t *testing.T) {
	srv, calls, _ := statusSequence(t, 503)
	resp, err := doRequest(t, &retryTransport{next: http.DefaultTransport}, http.MethodGet, srv.URL, "")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if got := atomic.LoadInt32(calls); got != 1 {
		t.Errorf("expected no retry, got %d calls", got)
	}
}

func TestRetryTransport_ReplaysBody(t *testing.T) {
	fastRetries(t)
	srv, _, bodies := statusSequence(t, 503)
	resp, err := doRequest(t, &retryTransport{next: http.DefaultTransport, maxRetries: 3}, http.MethodPut, srv.URL, `{"name":"web"}`)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if len(*bodies) != 2 || (*bodies)[0] != `{"name":"web"}` || (*bodies)[1] != `{"name":"web"}` {
		t.Errorf("expected the body sent twice, got %q", *bodies)
	}
}

func TestRetryTransport_NonReplayableBodyIsNotRetried(t *testing.T) {
	fastRetries(t)
	srv, calls, _ := statusSequence(t, 503)
	// An io.Reader http.NewRequest cannot rewind leaves GetBody nil.
	req, _ := http.NewRequestWithContext(context.Background(), http.MethodPut, srv.URL, io.MultiReader(strings.NewReader("x")))
	resp, err := (&http.Client{Transport: &retryTransport{next: http.DefaultTransport, maxRetries: 3}}).Do(req)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	resp.Body.Close()
	if got := atomic.LoadInt32(calls); got != 1 {
		t.Errorf("expected no retry without GetBody, got %d calls", got)
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestRetryTransport_ConnectionErrors(t *testing.T) {
	fastRetries(t)
	refused := fmt.Errorf("dial tcp: %w", syscall.ECONNREFUSED)
	reset := fmt.Errorf("read tcp: %w", syscall.ECONNRESET)
	cases := []struct {
		name      string
		method    string
		err       error
		wantCalls int
	}{
		{"refused POST is retried", http.MethodPost, refused, 4},
		{"reset GET is retried", http.MethodGet, reset, 4},
		{"reset PUT is not retried", http.MethodPut, reset, 1},
		{"EOF POST is not retried", http.MethodPost, io.ErrUnexpectedEOF, 1},
		{"other error is not retried", http.MethodGet, errors.New("x509: certificate signed by unknown authority"), 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			rt := &retryTransport{maxRetries: 3, next: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return nil, tc.err
			})}
			req, _ := http.NewRequestWithContext(context.Background(), tc.method, "http://portainer.invalid/api", strings.NewReader("{}"))
			resp, err := rt.RoundTrip(req)
			if resp != nil {
				resp.Body.Close()
			}
			if !errors.Is(err, tc.err) {
				t.Errorf("expected the transport error back, got %v", err)
			}
			if calls != tc.wantCalls {
				t.Errorf("expected %d calls, got %d", tc.wantCalls, calls)
			}
		})
	}
}

func TestRetryTransport_HonoursRetryAfter(t *testing.T) {
	if d := retryDelay(0, &http.Response{Header: http.Header{"Retry-After": []string{"7"}}}); d != 7*time.Second {
		t.Errorf("expected Retry-After 7s, got %s", d)
	}
	if d := retryDelay(0, &http.Response{Header: http.Header{"Retry-After": []string{"3600"}}}); d != retryMaxDelay {
		t.Errorf("expected Retry-After capped at %s, got %s", retryMaxDelay, d)
	}
	if d := retryDelay(2, nil); d != 4*retryBaseDelay {
		t.Errorf("expected exponential backoff 4x base, got %s", d)
	}
	if d := retryDelay(40, nil); d != retryMaxDelay {
		t.Errorf("expected backoff capped at %s, got %s", retryMaxDelay, d)
	}
}

func TestRetryTransport_CancelledDuringBackoff(t *testing.T) {
	old := retryBaseDelay
	retryBaseDelay = time.Hour
	t.Cleanup(func() { retryBaseDelay = old })

	srv, _, _ := statusSequence(t, 503)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)

	start := time.Now()
	resp, err := (&retryTransport{next: http.DefaultTransport, maxRetries: 3}).RoundTrip(req)
	if resp != nil {
		resp.Body.Close()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected the context deadline, got %v", err)
	}
	if time.Since(start) > 5*time.Second {
		t.Error("the backoff did not stop when the context ended")
	}
}

func TestRetryTransport_RequestTimeout(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		select {
		case <-r.Context().Done():
		case <-time.After(2 * time.Second):
		}
	}))
	t.Cleanup(srv.Close)

	resp, err := doRequest(t, &retryTransport{next: http.DefaultTransport, maxRetries: 3, timeout: 50 * time.Millisecond}, http.MethodGet, srv.URL, "")
	if resp != nil {
		resp.Body.Close()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected the request timeout, got %v", err)
	}
	if got := atomic.LoadInt32(&calls); got != 1 {
		t.Errorf("a timed-out request must not be retried, got %d calls", got)
	}
}

func TestRetryTransport_RequestTimeoutCoversBodyRead(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	resp, err := doRequest(t, &retryTransport{next: http.DefaultTransport, timeout: time.Second}, http.MethodGet, srv.URL, "")
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil || string(body) != "ok" {
		t.Errorf("expected the body to be readable after RoundTrip, got %q, %v", body, err)
	}
}

func TestConfigureProvider_RetryAndTimeout(t *testing.T) {
	d := newProviderData(t, map[string]interface{}{
		"endpoint":        "https://portainer.example.com",
		"api_key":         "ptr_abc",
		"max_retries":     5,
		"request_timeout": "90s",
	})
	out, diags := configureProvider(context.Background(), d)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	rt, ok := out.(*APIClient).HTTPClient.Transport.(*retryTransport)
	if !ok {
		t.Fatalf("expected the HTTP client to use retryTransport, got %T", out.(*APIClient).HTTPClient.Transport)
	}
	if rt.maxRetries != 5 || rt.timeout != 90*time.Second {
		t.Errorf("expected maxRetries 5 and timeout 90s, got %d and %s", rt.maxRetries, rt.timeout)
	}
}

func TestValidateDuration(t *testing.T) {
	for _, v := range []string{"", "90s", "2m", "1h30m"} {
		if _, errs := validateDuration(v, "request_timeout"); len(errs) != 0 {
			t.Errorf("%q: unexpected errors %v", v, errs)
		}
	}
	for _, v := range []string{"90", "soon", "-1s"} {
		if _, errs := validateDuration(v, "request_timeout"); len(errs) == 0 {
			t.Errorf("%q: expected a validation error", v)
		}
	}
}
