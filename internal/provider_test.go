package internal

import (
	"strings"
	"testing"
)

// --------------- Endpoint URL Normalization ---------------
// The provider's configureProvider function appends "/api" to the endpoint if missing.
// We test the same logic in isolation here.

func normalizeEndpoint(endpoint string) string {
	if !strings.HasSuffix(endpoint, "/api") {
		endpoint = strings.TrimRight(endpoint, "/") + "/api"
	}
	return endpoint
}

func TestNormalizeEndpoint(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "plain URL without /api",
			input:    "https://portainer.example.com",
			expected: "https://portainer.example.com/api",
		},
		{
			name:     "URL already has /api",
			input:    "https://portainer.example.com/api",
			expected: "https://portainer.example.com/api",
		},
		{
			name:     "URL with trailing slash",
			input:    "https://portainer.example.com/",
			expected: "https://portainer.example.com/api",
		},
		{
			name:     "URL with port",
			input:    "https://localhost:9443",
			expected: "https://localhost:9443/api",
		},
		{
			name:     "URL with port and /api",
			input:    "https://localhost:9443/api",
			expected: "https://localhost:9443/api",
		},
		{
			name:     "URL with subpath",
			input:    "https://example.com/portainer",
			expected: "https://example.com/portainer/api",
		},
		{
			name:     "URL with subpath and /api",
			input:    "https://example.com/portainer/api",
			expected: "https://example.com/portainer/api",
		},
		{
			name:     "URL with multiple trailing slashes",
			input:    "https://example.com///",
			expected: "https://example.com/api",
		},
		{
			name:     "HTTP URL",
			input:    "http://192.168.1.1:9000",
			expected: "http://192.168.1.1:9000/api",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := normalizeEndpoint(tt.input)
			if result != tt.expected {
				t.Errorf("normalizeEndpoint(%q) = %q, want %q", tt.input, result, tt.expected)
			}
		})
	}
}

// --------------- headerTransport ---------------

func TestHeaderTransportNotNil(t *testing.T) {
	// Verify that the headerTransport struct can be instantiated properly.
	headers := map[string]string{
		"X-Custom-Header": "test-value",
		"Authorization":   "Bearer token123",
	}
	ht := &headerTransport{
		Headers: headers,
	}
	if ht.Headers == nil {
		t.Fatal("expected non-nil headers")
	}
	if ht.Headers["X-Custom-Header"] != "test-value" {
		t.Errorf("expected 'test-value', got %q", ht.Headers["X-Custom-Header"])
	}
}

// --------------- APIClient struct ---------------

func TestAPIClientStruct(t *testing.T) {
	client := &APIClient{
		Endpoint: "https://portainer.example.com/api",
		APIKey:   "test-key",
	}
	if client.Endpoint != "https://portainer.example.com/api" {
		t.Errorf("unexpected endpoint: %q", client.Endpoint)
	}
	if client.APIKey != "test-key" {
		t.Errorf("unexpected api key: %q", client.APIKey)
	}
	if client.JWTToken != "" {
		t.Errorf("expected empty JWT token, got %q", client.JWTToken)
	}
}
