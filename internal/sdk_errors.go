package internal

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
)

type errorBodyKey struct{}

type capturedErrorBody struct {
	Status int
	Body   []byte
}

func withErrorCapture(ctx context.Context) (context.Context, *capturedErrorBody) {
	c := &capturedErrorBody{}
	return context.WithValue(ctx, errorBodyKey{}, c), c
}

type errorCaptureTransport struct {
	next http.RoundTripper
}

func (t *errorCaptureTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.next.RoundTrip(req)
	if err != nil || resp == nil || resp.StatusCode < 400 {
		return resp, err
	}
	capture, ok := req.Context().Value(errorBodyKey{}).(*capturedErrorBody)
	if !ok {
		return resp, nil
	}
	body, readErr := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	resp.Body = io.NopCloser(bytes.NewReader(body))
	if readErr == nil {
		capture.Status = resp.StatusCode
		capture.Body = body
	}
	return resp, nil
}

func decorateSDKError(err error, capture *capturedErrorBody) error {
	if err == nil || capture == nil || len(capture.Body) == 0 {
		return err
	}
	return fmt.Errorf("%w: %s", err, bytes.TrimSpace(capture.Body))
}
