package internal

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"gopkg.in/yaml.v3"
)

// removeFromStateContext is the context-aware equivalent of schema.RemoveFromState
// (which has no *Context variant in plugin-sdk v2). It clears the ID so the
// resource is removed from state, used as DeleteContext for action-style
// resources that have nothing to delete server-side.
func removeFromStateContext(_ context.Context, d *schema.ResourceData, _ interface{}) diag.Diagnostics {
	d.SetId("")
	return nil
}

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

// setFields applies multiple d.Set calls and returns the first error encountered,
// wrapping it with the offending field name. It replaces the repeated
// `_ = d.Set(...)` pattern in Read handlers so schema mismatches surface as a
// diagnostic instead of being silently dropped. d.Set only errors on a schema
// type mismatch (a provider bug), so in practice this never fails at runtime —
// it just makes those failures visible during development and testing.
func setFields(d *schema.ResourceData, fields map[string]interface{}) error {
	for k, v := range fields {
		if err := d.Set(k, v); err != nil {
			return fmt.Errorf("failed to set %q: %w", k, err)
		}
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

func (e *apiStatusError) Error() string {
	return fmt.Sprintf("%s %s failed with status %d: %s", e.Method, e.URL, e.StatusCode, e.Body)
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
	if err := setAuthHeader(req, client); err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

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

func apiGET(url string, apiKey string, client *APIClient) ([]byte, error) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
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

func apiGETWithCode(url string, apiKey string, client *APIClient) ([]byte, int, error) {
	req, _ := http.NewRequest(http.MethodGet, url, nil)
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
