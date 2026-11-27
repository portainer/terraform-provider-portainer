package internal

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// APIClient is a simple client struct to store connection information.
type APIClient struct {
	Endpoint      string
	APIKey        string
	JWTToken      string
	CustomHeaders map[string]string
	HTTPClient    http.Client
}

type headerTransport struct {
	Transport http.RoundTripper
	Headers   map[string]string
}

func (t *headerTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	for k, v := range t.Headers {
		req.Header.Set(k, v)
	}
	return t.Transport.RoundTrip(req)
}

// DoRequest is a reusable method for making API requests
func (c *APIClient) DoRequest(ctx context.Context, method, path string, headers map[string]string, body interface{}) (*http.Response, error) {
	var buf io.Reader
	if body != nil {
		jsonBody, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		buf = bytes.NewBuffer(jsonBody)
	}

	req, err := http.NewRequestWithContext(ctx, method, fmt.Sprintf("%s%s", c.Endpoint, path), buf)
	if err != nil {
		return nil, err
	}

	if _, ok := headers["Content-Type"]; !ok {
		req.Header.Set("Content-Type", "application/json")
	}

	if c.APIKey != "" {
		req.Header.Set("X-API-Key", c.APIKey)
	} else if c.JWTToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.JWTToken)
	}

	for k, v := range headers {
		req.Header.Set(k, v)
	}

	return c.HTTPClient.Do(req)
}

// providerConfig is the provider block with the environment defaults applied
// (see frameworkProvider.Configure).
type providerConfig struct {
	Endpoint       string
	APIKey         string
	User           string
	Password       string
	SkipSSLVerify  bool
	CustomHeaders  map[string]string
	MaxRetries     int
	RequestTimeout string
}

// newAPIClient builds the client every resource and data source shares, and
// logs in with user and password when no API key is given.
func newAPIClient(ctx context.Context, cfg providerConfig) (*APIClient, error) {
	endpoint := cfg.Endpoint
	apiKey := cfg.APIKey
	user := cfg.User
	password := cfg.Password
	customHeaders := cfg.CustomHeaders

	if apiKey != "" && (user != "" || password != "") {
		return nil, fmt.Errorf("you must specify either 'api_key' or 'api_user'/'api_password', not both")
	}

	if (user == "" && password != "") || (user != "" && password == "") {
		return nil, fmt.Errorf("'api_user' and 'api_password' must be specified together")
	}

	transport := &http.Transport{
		// Honor HTTP_PROXY / HTTPS_PROXY / NO_PROXY like http.DefaultTransport,
		// so the provider works behind corporate or userspace proxies.
		Proxy: http.ProxyFromEnvironment,
		TLSClientConfig: &tls.Config{
			InsecureSkipVerify: cfg.SkipSSLVerify,
		},
	}

	// Wrap transport with custom headers if provided
	var transportWithCustomHeaders http.RoundTripper = transport
	if len(customHeaders) > 0 {
		transportWithCustomHeaders = &headerTransport{
			Transport: transport,
			Headers:   customHeaders,
		}
	}

	var requestTimeout time.Duration
	if cfg.RequestTimeout != "" {
		var err error
		if requestTimeout, err = time.ParseDuration(cfg.RequestTimeout); err != nil {
			return nil, fmt.Errorf("invalid request_timeout %q: %w", cfg.RequestTimeout, err)
		}
	}
	transportWithRetry := &retryTransport{
		next:       transportWithCustomHeaders,
		maxRetries: cfg.MaxRetries,
		timeout:    requestTimeout,
	}

	httpClient := &http.Client{
		Transport: transportWithRetry,
	}

	if !strings.HasSuffix(endpoint, "/api") {
		endpoint = strings.TrimRight(endpoint, "/") + "/api"
	}

	client := &APIClient{
		Endpoint:      endpoint,
		APIKey:        apiKey,
		JWTToken:      "",
		CustomHeaders: customHeaders,
		HTTPClient:    *httpClient,
	}

	// Authenticate via user/password and fetch JWT if api_key is not used
	if apiKey == "" && user != "" && password != "" {
		authBody := map[string]string{
			"Username": user,
			"Password": password,
		}
		payload, _ := json.Marshal(authBody)
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint+"/auth", bytes.NewBuffer(payload))
		if err != nil {
			return nil, fmt.Errorf("failed to create auth request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")

		resp, err := httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("failed to authenticate using username/password: %w", err)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			respBody, _ := io.ReadAll(resp.Body)
			return nil, fmt.Errorf("authentication failed: %s", string(respBody))
		}

		var authResp struct {
			JWT string `json:"jwt"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
			return nil, fmt.Errorf("failed to parse authentication response: %w", err)
		}
		client.JWTToken = authResp.JWT
	}

	return client, nil
}
