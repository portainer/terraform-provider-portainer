package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// setAuthHeader sets the Portainer authentication header on req from the
// client's configured credentials: the API key (X-API-Key) is preferred, and
// the JWT bearer token is used as a fallback. It returns an error when neither
// credential is configured. This centralizes the auth-selection block that was
// previously duplicated across every direct-HTTP call site.
func setAuthHeader(req *http.Request, client *APIClient) error {
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}
	return nil
}

// apiStatusError is returned by doJSON when the Portainer API responds with a
// status code >= 400. It preserves the status code and raw body so callers can
// react to specific statuses (most commonly 404, via isAPINotFound) instead of
// string-matching the error text.
type apiStatusError struct {
	Method     string
	URL        string
	StatusCode int
	Body       string
}

// sensitiveQueryParams are query parameter names whose value must never reach
// an error message. Some Portainer endpoints take a credential in the query
// string, and an error carrying the full URL would put it into Terraform
// diagnostics and, for any caller that stores the message, into state.
var sensitiveQueryParams = map[string]bool{
	"serviceaccountkey": true,
	"apikey":            true,
	"api_key":           true,
	"password":          true,
	"token":             true,
	"secret":            true,
	"key":               true,
}

// redactURL replaces the value of any sensitive query parameter with REDACTED,
// leaving the rest of the URL readable so the error still says what failed.
func redactURL(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil || parsed.RawQuery == "" {
		return raw
	}
	query := parsed.Query()
	redacted := false
	for name := range query {
		if sensitiveQueryParams[strings.ToLower(name)] {
			query.Set(name, "REDACTED")
			redacted = true
		}
	}
	if !redacted {
		return raw
	}
	parsed.RawQuery = query.Encode()
	return parsed.String()
}

func (e *apiStatusError) Error() string {
	return fmt.Sprintf("%s %s failed with status %d: %s", e.Method, redactURL(e.URL), e.StatusCode, e.Body)
}

// isAPINotFound reports whether err is (or wraps) an apiStatusError with a 404
// status. Read handlers use it to drop a resource from state when the backing
// object no longer exists, replacing the hand-rolled StatusNotFound check.
func isAPINotFound(err error) bool {
	var se *apiStatusError
	return errors.As(err, &se) && se.StatusCode == http.StatusNotFound
}

// doJSON performs an authenticated request against the Portainer API and
// decodes a successful (2xx/3xx) JSON response into out. It centralizes the
// request/auth/status/decode boilerplate that every direct-HTTP resource used
// to repeat by hand: building the request, selecting the auth header,
// JSON-marshaling the body, checking the status code, and reading the response
// body (for both decode and error reporting).
//
// urlStr must be the fully-formed URL — callers build it from client.Endpoint
// plus any path and query string. body may be nil (e.g. for GET/DELETE or
// action-style PUT/POST calls); when non-nil it is JSON-marshaled and the
// Content-Type header is set. out may be nil when the caller does not care
// about the response payload; otherwise a non-empty response body is
// unmarshaled into it. A status >= 400 is returned as an error that includes
// the method, URL and raw response body.
func doJSON(ctx context.Context, client *APIClient, method, urlStr string, body, out interface{}) error {
	return doJSONWithHeaders(ctx, client, method, urlStr, body, out, nil)
}

// authHeaders are the headers that carry the client's credentials. A caller of
// doJSONWithHeaders may not set them: setAuthHeader picks one of the two
// depending on how the provider is configured, so ordering alone would not stop
// a caller-supplied value from surviving in the other one.
var authHeaders = map[string]bool{
	http.CanonicalHeaderKey("X-API-Key"):     true,
	http.CanonicalHeaderKey("Authorization"): true,
}

// doJSONWithHeaders is doJSON plus request headers the Portainer API expects
// outside the body: X-Setup-Token, which POST /restore requires on an
// uninitialised instance, or a Content-Type other than application/json (the
// Kubernetes merge patch).
//
// Authentication is not negotiable here: headers naming a credential are
// dropped from the caller's map, so no call site can weaken or replace the
// authentication the provider is configured with, whether by accident or not.
func doJSONWithHeaders(ctx context.Context, client *APIClient, method, urlStr string, body, out interface{}, headers map[string]string) error {
	return doJSONRequest(ctx, client, method, urlStr, body, out, headers, true)
}

// doJSONPublic is doJSONWithHeaders for the public endpoints an uninitialised
// instance answers (e.g. /users/admin/init): the configured credentials are
// sent when there are any, and their absence is not an error.
func doJSONPublic(ctx context.Context, client *APIClient, method, urlStr string, body, out interface{}, headers map[string]string) error {
	return doJSONRequest(ctx, client, method, urlStr, body, out, headers, false)
}

func doJSONRequest(ctx context.Context, client *APIClient, method, urlStr string, body, out interface{}, headers map[string]string, requireAuth bool) error {
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("failed to marshal %s %s request body: %w", method, urlStr, err)
		}
		reader = bytes.NewReader(raw)
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, reader)
	if err != nil {
		return fmt.Errorf("failed to build %s %s request: %w", method, urlStr, err)
	}
	for k, v := range headers {
		if authHeaders[http.CanonicalHeaderKey(k)] {
			continue
		}
		req.Header.Set(k, v)
	}
	if err := setAuthHeader(req, client); err != nil && requireAuth {
		return err
	}
	if body != nil && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", "application/json")
	}
	return sendDecode(client, req, out)
}

// multipartFile is a file part of a multipart/form-data request.
type multipartFile struct {
	Field    string
	Filename string
	Content  string
}

// doMultipart is doJSON for multipart/form-data requests. The fields (name,
// value pairs) are written in order, followed by the files, each with a
// content type sniffed from its content.
func doMultipart(ctx context.Context, client *APIClient, method, urlStr string, fields [][2]string, files []multipartFile, out interface{}) error {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, f := range fields {
		if err := w.WriteField(f[0], f[1]); err != nil {
			return fmt.Errorf("failed to build %s %s request: %w", method, urlStr, err)
		}
	}
	for _, f := range files {
		h := make(textproto.MIMEHeader)
		h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, f.Field, f.Filename))
		h.Set("Content-Type", http.DetectContentType([]byte(f.Content)))
		part, err := w.CreatePart(h)
		if err != nil {
			return fmt.Errorf("failed to build %s %s request: %w", method, urlStr, err)
		}
		if _, err := io.WriteString(part, f.Content); err != nil {
			return fmt.Errorf("failed to build %s %s request: %w", method, urlStr, err)
		}
	}
	if err := w.Close(); err != nil {
		return fmt.Errorf("failed to build %s %s request: %w", method, urlStr, err)
	}

	req, err := http.NewRequestWithContext(ctx, method, urlStr, bytes.NewReader(buf.Bytes()))
	if err != nil {
		return fmt.Errorf("failed to build %s %s request: %w", method, urlStr, err)
	}
	if err := setAuthHeader(req, client); err != nil {
		return err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	return sendDecode(client, req, out)
}

// sendDecode sends req and decodes a successful JSON response into out; a
// status >= 400 is returned as an *apiStatusError.
func sendDecode(client *APIClient, req *http.Request, out interface{}) error {
	method, urlStr := req.Method, req.URL.String()
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to perform %s %s request: %w", method, urlStr, err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return &apiStatusError{Method: method, URL: urlStr, StatusCode: resp.StatusCode, Body: string(respBody)}
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("failed to decode %s %s response: %w", method, urlStr, err)
		}
	}
	return nil
}

func parseManifest(manifest string) (map[string]interface{}, error) {
	var parsed map[string]interface{}

	// Try JSON
	if err := json.Unmarshal([]byte(manifest), &parsed); err == nil {
		return parsed, nil
	}

	// Try YAML
	if err := yaml.Unmarshal([]byte(manifest), &parsed); err == nil {
		return parsed, nil
	}

	return nil, fmt.Errorf("manifest is neither valid JSON nor YAML")
}

func toIntSlice(raw []interface{}) []int {
	res := make([]int, len(raw))
	for i, v := range raw {
		res[i] = v.(int)
	}
	return res
}

func splitAndTrimCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		t := strings.TrimSpace(p)
		if t != "" {
			out = append(out, t)
		}
	}
	return out
}

func contains(arr []string, v string) bool {
	for _, x := range arr {
		if x == v {
			return true
		}
	}
	return false
}

func mustMap(v interface{}) map[string]interface{} {
	if v == nil {
		m := make(map[string]interface{})
		return m
	}
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

// Context-aware API helpers (used by resources that support timeouts).

func apiGETCtx(ctx context.Context, url string, apiKey string, client *APIClient) ([]byte, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if client.APIKey != "" {
		req.Header.Set("X-API-Key", client.APIKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return nil, fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// apiGETRaw fetches a response body and, unlike apiGETCtx, fails on a non-2xx
// status instead of handing the error body back as if it were data. It returns
// the same *apiStatusError doJSON does, so isAPINotFound works on its result.
//
// Use this for endpoints that answer with something other than a JSON document
// the caller can decode into a struct - raw text, or a payload whose shape is
// not fixed - where a silently returned error page would otherwise be stored
// in state.
func apiGETRaw(ctx context.Context, client *APIClient, urlStr string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, urlStr, nil)
	if err != nil {
		return nil, err
	}
	if err := setAuthHeader(req, client); err != nil {
		return nil, err
	}

	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, &apiStatusError{StatusCode: resp.StatusCode, Body: string(body)}
	}
	return body, nil
}

func apiGETWithCodeCtx(ctx context.Context, url string, apiKey string, client *APIClient) ([]byte, int, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return nil, 0, fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}

func apiPOSTWithCodeCtx(ctx context.Context, url string, apiKey string, client *APIClient, payload []byte) ([]byte, int, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(payload))
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return nil, 0, fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}

func apiPUTWithCodeCtx(ctx context.Context, url string, apiKey string, client *APIClient, payload []byte) ([]byte, int, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodPut, url, bytes.NewBuffer(payload))
	if apiKey != "" {
		req.Header.Set("X-API-Key", apiKey)
	} else if client.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+client.JWTToken)
	} else {
		return nil, 0, fmt.Errorf("no valid authentication method provided (api_key or jwt token)")
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := client.HTTPClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	return body, resp.StatusCode, nil
}

// sleepCtx waits for d, or returns early with the context's error once ctx is
// done, so a user-configured wait does not outlive a cancelled or timed-out
// apply.
func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// validateDuration accepts an empty string or a non-negative Go duration such
// as "90s" or "2m".
func validateDuration(v interface{}, key string) ([]string, []error) {
	s, _ := v.(string)
	if s == "" {
		return nil, nil
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return nil, []error{fmt.Errorf("%q must be a duration such as \"90s\" or \"2m\": %w", key, err)}
	}
	if d < 0 {
		return nil, []error{fmt.Errorf("%q must not be negative", key)}
	}
	return nil, nil
}
