package internal

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"syscall"
	"time"
)

// retryBaseDelay is the first backoff step; each retry doubles it. A variable
// so tests can shorten it.
var retryBaseDelay = time.Second

// retryMaxDelay caps a single backoff step, including a server's Retry-After.
const retryMaxDelay = 30 * time.Second

// retryTransport retries requests that failed transiently and optionally
// limits how long a single attempt may take.
//
// A retry is only made when it cannot apply a change twice:
//   - a refused connection never reached Portainer, so any method is retried;
//   - 429 and 503 mean Portainer turned the request away, so idempotent
//     methods (GET, HEAD, OPTIONS, PUT, DELETE) are retried;
//   - 502, 504 and a dropped connection leave it open whether Portainer acted
//     on the request (a proxy in front of Portainer 2.39 answers 504 while the
//     stack deployment goes on), so only safe methods (GET, HEAD, OPTIONS) are
//     retried.
//
// A request whose body cannot be replayed is never retried.
type retryTransport struct {
	next       http.RoundTripper
	maxRetries int
	// timeout limits one attempt, including reading the response body.
	// Zero means no limit; the Terraform operation timeout still applies
	// through the request context.
	timeout time.Duration
}

func (t *retryTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	replayable := req.Body == nil || req.Body == http.NoBody || req.GetBody != nil

	for attempt := 0; ; attempt++ {
		r := req
		if attempt > 0 {
			r = req.Clone(req.Context())
			if req.GetBody != nil {
				body, err := req.GetBody()
				if err != nil {
					return nil, err
				}
				r.Body = body
			}
		}

		resp, err := t.roundTripOnce(r)
		if err == nil && attempt < t.maxRetries && isSafeMethod(req.Method) {
			resp, err = bufferBody(resp)
		}
		if !replayable || attempt >= t.maxRetries || req.Context().Err() != nil || !shouldRetry(req.Method, resp, err) {
			return resp, err
		}

		delay := retryDelay(attempt, resp)
		if resp != nil {
			log.Printf("[DEBUG] portainer: %s %s returned %d, retrying in %s (%d/%d)", req.Method, req.URL.Path, resp.StatusCode, delay, attempt+1, t.maxRetries)
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		} else {
			log.Printf("[DEBUG] portainer: %s %s failed: %v, retrying in %s (%d/%d)", req.Method, req.URL.Path, err, delay, attempt+1, t.maxRetries)
		}
		if err := sleepCtx(req.Context(), delay); err != nil {
			return nil, err
		}
	}
}

func (t *retryTransport) roundTripOnce(req *http.Request) (*http.Response, error) {
	if t.timeout <= 0 {
		return t.next.RoundTrip(req)
	}
	ctx, cancel := context.WithTimeout(req.Context(), t.timeout)
	resp, err := t.next.RoundTrip(req.WithContext(ctx))
	if err != nil {
		cancel()
		return nil, err
	}
	// The deadline has to cover reading the body too, so it is released only
	// when the caller closes it.
	resp.Body = &cancelOnClose{ReadCloser: resp.Body, cancel: cancel}
	return resp, nil
}

type cancelOnClose struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (c *cancelOnClose) Close() error {
	err := c.ReadCloser.Close()
	c.cancel()
	return err
}

func isSafeMethod(method string) bool {
	return method == http.MethodGet || method == http.MethodHead || method == http.MethodOptions
}

// bufferBody reads the response body of a safe request while a retry is
// still possible. A connection dropped after the headers only fails while the
// body is read, which would happen in the caller, past any retry; here it
// becomes the attempt's error, which shouldRetry judges like any dropped
// connection.
func bufferBody(resp *http.Response) (*http.Response, error) {
	data, err := io.ReadAll(resp.Body)
	_ = resp.Body.Close()
	if err != nil {
		return nil, err
	}
	resp.Body = io.NopCloser(bytes.NewReader(data))
	resp.ContentLength = int64(len(data))
	return resp, nil
}

func shouldRetry(method string, resp *http.Response, err error) bool {
	safe := isSafeMethod(method)
	idempotent := safe || method == http.MethodPut || method == http.MethodDelete

	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return false
		}
		if errors.Is(err, syscall.ECONNREFUSED) {
			return true
		}
		return safe && (errors.Is(err, syscall.ECONNRESET) || errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF))
	}

	switch resp.StatusCode {
	case http.StatusTooManyRequests, http.StatusServiceUnavailable:
		return idempotent
	case http.StatusBadGateway, http.StatusGatewayTimeout:
		return safe
	}
	return false
}

// retryDelay doubles retryBaseDelay per attempt, or follows a Retry-After
// given in seconds; both are capped at retryMaxDelay.
func retryDelay(attempt int, resp *http.Response) time.Duration {
	delay := retryBaseDelay << attempt
	if resp != nil {
		if secs, err := strconv.Atoi(resp.Header.Get("Retry-After")); err == nil && secs >= 0 {
			delay = time.Duration(secs) * time.Second
		}
	}
	if delay > retryMaxDelay || delay < 0 {
		delay = retryMaxDelay
	}
	return delay
}
