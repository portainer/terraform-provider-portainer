package internal

import (
	"context"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestErrorCaptureTransport_NonError_NoCapture verifies that 2xx responses
// pass through untouched and do not populate the capture.
func TestErrorCaptureTransport_NonError_NoCapture(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"ok":true}`))
	}))
	defer srv.Close()

	rt := &errorCaptureTransport{next: http.DefaultTransport}
	ctx, capture := withErrorCapture(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if capture.Status != 0 || capture.Body != nil {
		t.Errorf("capture should be empty for 2xx; got status=%d body=%q", capture.Status, capture.Body)
	}
	body, _ := io.ReadAll(resp.Body)
	if string(body) != `{"ok":true}` {
		t.Errorf("response body not preserved; got %q", body)
	}
}

// TestErrorCaptureTransport_ErrorBody_CapturesAndReinjects verifies that non-2xx
// responses populate the capture AND the response body remains readable by the
// downstream consumer (the SDK reader).
func TestErrorCaptureTransport_ErrorBody_CapturesAndReinjects(t *testing.T) {
	payload := `{"message":"Invalid request payload","details":"Invalid environment name"}`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(payload))
	}))
	defer srv.Close()

	rt := &errorCaptureTransport{next: http.DefaultTransport}
	ctx, capture := withErrorCapture(context.Background())
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, srv.URL, nil)

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	if capture.Status != http.StatusBadRequest {
		t.Errorf("capture status: want 400, got %d", capture.Status)
	}
	if string(capture.Body) != payload {
		t.Errorf("capture body mismatch:\nwant: %q\ngot : %q", payload, capture.Body)
	}

	// Downstream consumer must still see the original body.
	body, _ := io.ReadAll(resp.Body)
	if string(body) != payload {
		t.Errorf("re-injected body mismatch:\nwant: %q\ngot : %q", payload, body)
	}
}

// TestErrorCaptureTransport_NoCaptureKey_PassesThrough verifies that when a
// caller did not opt into withErrorCapture(), a non-2xx response is left alone
// and the body is still readable.
func TestErrorCaptureTransport_NoCaptureKey_PassesThrough(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`boom`))
	}))
	defer srv.Close()

	rt := &errorCaptureTransport{next: http.DefaultTransport}
	req, _ := http.NewRequest(http.MethodGet, srv.URL, nil)

	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	if string(body) != "boom" {
		t.Errorf("body not preserved for opt-out caller: got %q", body)
	}
}

// TestDecorateSDKError_AttachesBody verifies the helper appends the captured
// Portainer payload to the original SDK error.
func TestDecorateSDKError_AttachesBody(t *testing.T) {
	base := errors.New("[POST /endpoints][400] endpointCreateBadRequest")
	cap := &capturedErrorBody{Status: 400, Body: []byte(`{"details":"reason"}`)}

	got := decorateSDKError(base, cap)
	if !strings.Contains(got.Error(), "endpointCreateBadRequest") {
		t.Errorf("decorated error should preserve original message; got %q", got.Error())
	}
	if !strings.Contains(got.Error(), `{"details":"reason"}`) {
		t.Errorf("decorated error should include captured body; got %q", got.Error())
	}
	if !errors.Is(got, base) {
		t.Errorf("decorated error should wrap the original (errors.Is must hold)")
	}
}

// TestDecorateSDKError_NoCapture_NoChange verifies the helper is a no-op when
// the capture is empty (e.g. transport-level error before a response arrived).
func TestDecorateSDKError_NoCapture_NoChange(t *testing.T) {
	base := errors.New("some sdk err")
	cap := &capturedErrorBody{}

	got := decorateSDKError(base, cap)
	if got.Error() != "some sdk err" {
		t.Errorf("expected unchanged error, got %q", got.Error())
	}
}

// TestDecorateSDKError_TrimsBody verifies leading/trailing whitespace in the
// captured body is trimmed so the decorated error reads cleanly on one line.
func TestDecorateSDKError_TrimsBody(t *testing.T) {
	base := errors.New("err")
	cap := &capturedErrorBody{Status: 500, Body: []byte("  payload\n\n")}

	got := decorateSDKError(base, cap)
	if !strings.HasSuffix(got.Error(), ": payload") {
		t.Errorf("expected trimmed body suffix; got %q", got.Error())
	}
}
