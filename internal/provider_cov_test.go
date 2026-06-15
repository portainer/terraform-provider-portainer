package internal

import (
	"bytes"
	"context"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// --- Provider sanity ---------------------------------------------------------

func TestProvider_InternalValidate_Cov(t *testing.T) {
	if err := Provider().InternalValidate(); err != nil {
		t.Fatalf("provider failed InternalValidate: %v", err)
	}
}

// --- configureProvider -------------------------------------------------------

func newProviderData(t *testing.T, raw map[string]interface{}) *schema.ResourceData {
	t.Helper()
	return schema.TestResourceDataRaw(t, Provider().Schema, raw)
}

func TestConfigureProvider_APIKeyMode_Cov(t *testing.T) {
	d := newProviderData(t, map[string]interface{}{
		"endpoint": "https://portainer.example.com",
		"api_key":  "ptr_abc",
	})

	out, diags := configureProvider(context.Background(), d)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	client, ok := out.(*APIClient)
	if !ok {
		t.Fatalf("expected *APIClient, got %T", out)
	}
	if client.APIKey != "ptr_abc" {
		t.Errorf("expected APIKey to be set, got %q", client.APIKey)
	}
	if client.JWTToken != "" {
		t.Errorf("expected empty JWT in api-key mode, got %q", client.JWTToken)
	}
	// '/api' appended to endpoint.
	if client.Endpoint != "https://portainer.example.com/api" {
		t.Errorf("expected /api appended, got %q", client.Endpoint)
	}
}

func TestConfigureProvider_EndpointAlreadyHasAPI_Cov(t *testing.T) {
	d := newProviderData(t, map[string]interface{}{
		"endpoint": "https://portainer.example.com/api",
		"api_key":  "ptr_abc",
	})

	out, diags := configureProvider(context.Background(), d)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	client := out.(*APIClient)
	if client.Endpoint != "https://portainer.example.com/api" {
		t.Errorf("expected endpoint unchanged, got %q", client.Endpoint)
	}
}

func TestConfigureProvider_CustomHeaders_Cov(t *testing.T) {
	d := newProviderData(t, map[string]interface{}{
		"endpoint": "https://portainer.example.com",
		"api_key":  "ptr_abc",
		"custom_headers": map[string]interface{}{
			"CF-Access-Client-Id": "id123",
		},
	})

	out, diags := configureProvider(context.Background(), d)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	client := out.(*APIClient)
	if client.CustomHeaders["CF-Access-Client-Id"] != "id123" {
		t.Errorf("expected custom header captured, got %v", client.CustomHeaders)
	}
}

func TestConfigureProvider_UserPasswordMode_Cov(t *testing.T) {
	mock := NewMockServer(t)
	// configureProvider posts to <endpoint>/auth ; endpoint gets /api appended,
	// so the path the dispatcher sees (after stripping /api) is "/auth".
	mock.On("POST", "/auth", RespondJSON(http.StatusOK, map[string]interface{}{
		"jwt": "jwt-from-server",
	}))

	d := newProviderData(t, map[string]interface{}{
		"endpoint":     mock.URL,
		"api_user":     "admin",
		"api_password": "secret",
	})

	out, diags := configureProvider(context.Background(), d)
	if diags.HasError() {
		t.Fatalf("unexpected diagnostics: %v", diags)
	}
	client := out.(*APIClient)
	if client.JWTToken != "jwt-from-server" {
		t.Errorf("expected JWT from server, got %q", client.JWTToken)
	}

	req := mock.FindRequest("POST", "/auth")
	if req == nil {
		t.Fatal("expected POST /auth recorded")
	}
	var payload map[string]string
	if err := req.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode auth body: %v", err)
	}
	if payload["Username"] != "admin" || payload["Password"] != "secret" {
		t.Errorf("unexpected auth payload: %v", payload)
	}
}

func TestConfigureProvider_UserPasswordAuthFails_Cov(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/auth", RespondString(http.StatusUnauthorized, "application/json", `{"message":"invalid creds"}`))

	d := newProviderData(t, map[string]interface{}{
		"endpoint":     mock.URL,
		"api_user":     "admin",
		"api_password": "wrong",
	})

	_, diags := configureProvider(context.Background(), d)
	if !diags.HasError() {
		t.Fatal("expected diagnostics on auth failure")
	}
}

func TestConfigureProvider_MutualExclusion_Cov(t *testing.T) {
	d := newProviderData(t, map[string]interface{}{
		"endpoint": "https://portainer.example.com",
		"api_key":  "ptr_abc",
		"api_user": "admin",
	})

	_, diags := configureProvider(context.Background(), d)
	if !diags.HasError() {
		t.Fatal("expected error when api_key and api_user are both set")
	}
}

func TestConfigureProvider_UserWithoutPassword_Cov(t *testing.T) {
	d := newProviderData(t, map[string]interface{}{
		"endpoint": "https://portainer.example.com",
		"api_user": "admin",
		// no api_password
	})

	_, diags := configureProvider(context.Background(), d)
	if !diags.HasError() {
		t.Fatal("expected error when api_user is set without api_password")
	}
}

func TestConfigureProvider_PasswordWithoutUser_Cov(t *testing.T) {
	d := newProviderData(t, map[string]interface{}{
		"endpoint":     "https://portainer.example.com",
		"api_password": "secret",
		// no api_user
	})

	_, diags := configureProvider(context.Background(), d)
	if !diags.HasError() {
		t.Fatal("expected error when api_password is set without api_user")
	}
}

// --- headerTransport.RoundTrip ----------------------------------------------

// captureRoundTripper records the request it receives so the test can assert
// on the headers headerTransport injected.
type captureRoundTripper struct {
	got *http.Request
}

func (c *captureRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	c.got = req
	return &http.Response{
		StatusCode: http.StatusOK,
		Body:       http.NoBody,
		Header:     make(http.Header),
	}, nil
}

func TestHeaderTransport_RoundTrip_Cov(t *testing.T) {
	cap := &captureRoundTripper{}
	ht := &headerTransport{
		Transport: cap,
		Headers: map[string]string{
			"X-Custom":  "value1",
			"X-Another": "value2",
		},
	}

	req, _ := http.NewRequest(http.MethodGet, "https://example.com/foo", nil)
	resp, err := ht.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
	if cap.got.Header.Get("X-Custom") != "value1" {
		t.Errorf("expected X-Custom header injected, got %q", cap.got.Header.Get("X-Custom"))
	}
	if cap.got.Header.Get("X-Another") != "value2" {
		t.Errorf("expected X-Another header injected, got %q", cap.got.Header.Get("X-Another"))
	}
}

// --- (*APIClient).DoRequest --------------------------------------------------

func TestDoRequest_HappyPath_Cov(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/widgets", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 1}))

	client := mock.Client()
	resp, err := client.DoRequest("POST", "/api/widgets", nil, map[string]interface{}{"name": "w"})
	if err != nil {
		t.Fatalf("DoRequest failed: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}

	req := mock.FindRequest("POST", "/widgets")
	if req == nil {
		t.Fatal("expected POST /widgets recorded")
	}
	if req.Headers.Get("X-API-Key") != "test-api-key" {
		t.Errorf("expected X-API-Key header, got %q", req.Headers.Get("X-API-Key"))
	}
	if req.Headers.Get("Content-Type") != "application/json" {
		t.Errorf("expected default JSON content-type, got %q", req.Headers.Get("Content-Type"))
	}
	var payload map[string]interface{}
	if err := req.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["name"] != "w" {
		t.Errorf("unexpected payload: %v", payload)
	}
}

func TestDoRequest_NilBodyAndCustomHeaders_Cov(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/ping", RespondString(http.StatusOK, "application/json", `pong`))

	client := mock.Client()
	resp, err := client.DoRequest("GET", "/api/ping", map[string]string{"Content-Type": "text/plain", "X-Extra": "e"}, nil)
	if err != nil {
		t.Fatalf("DoRequest failed: %v", err)
	}
	defer resp.Body.Close()

	req := mock.FindRequest("GET", "/ping")
	if req == nil {
		t.Fatal("expected GET /ping recorded")
	}
	// Custom Content-Type honored (not overwritten with application/json).
	if req.Headers.Get("Content-Type") != "text/plain" {
		t.Errorf("expected custom content-type honored, got %q", req.Headers.Get("Content-Type"))
	}
	if req.Headers.Get("X-Extra") != "e" {
		t.Errorf("expected X-Extra header, got %q", req.Headers.Get("X-Extra"))
	}
}

func TestDoRequest_JWTAuth_Cov(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/secured", RespondString(http.StatusOK, "application/json", `{}`))

	client := mock.Client()
	client.APIKey = ""
	client.JWTToken = "jwt-xyz"

	resp, err := client.DoRequest("GET", "/api/secured", nil, nil)
	if err != nil {
		t.Fatalf("DoRequest failed: %v", err)
	}
	defer resp.Body.Close()

	req := mock.FindRequest("GET", "/secured")
	if req == nil || req.Headers.Get("Authorization") != "Bearer jwt-xyz" {
		t.Errorf("expected Bearer auth header, got %v", req)
	}
}

func TestDoRequest_NonGenericError_Cov(t *testing.T) {
	// Invalid method triggers http.NewRequest error.
	client := &APIClient{Endpoint: "http://example.com", APIKey: "k"}
	resp, err := client.DoRequest("BAD METHOD WITH SPACES", "/x", nil, nil)
	if err == nil {
		resp.Body.Close()
		t.Fatal("expected error from invalid request construction")
	}
}

// --- (*APIClient).DoMultipartRequest ----------------------------------------

func TestDoMultipartRequest_HappyPath_Cov(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/upload", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 7, "Name": "f"}))

	client := mock.Client()
	body := bytes.NewBufferString("--boundary--")
	var out struct {
		Id   int    `json:"Id"`
		Name string `json:"Name"`
	}
	err := client.DoMultipartRequest("POST", mock.URL+"/api/upload",
		body, map[string]string{"Content-Type": "multipart/form-data; boundary=boundary"}, &out)
	if err != nil {
		t.Fatalf("DoMultipartRequest failed: %v", err)
	}
	if out.Id != 7 || out.Name != "f" {
		t.Errorf("unexpected decoded output: %+v", out)
	}

	req := mock.FindRequest("POST", "/upload")
	if req == nil || req.Headers.Get("X-API-Key") != "test-api-key" {
		t.Errorf("expected X-API-Key on multipart request, got %v", req)
	}
}

func TestDoMultipartRequest_NoOut_Cov(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/upload2", RespondString(http.StatusNoContent, "", ""))

	client := mock.Client()
	if err := client.DoMultipartRequest("POST", mock.URL+"/api/upload2",
		bytes.NewBufferString("x"), nil, nil); err != nil {
		t.Fatalf("DoMultipartRequest with nil out failed: %v", err)
	}
}

func TestDoMultipartRequest_JWTAuth_Cov(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/upload3", RespondString(http.StatusOK, "application/json", `{}`))

	client := mock.Client()
	client.APIKey = ""
	client.JWTToken = "jwt-mp"

	if err := client.DoMultipartRequest("POST", mock.URL+"/api/upload3",
		bytes.NewBufferString("x"), nil, nil); err != nil {
		t.Fatalf("DoMultipartRequest failed: %v", err)
	}
	req := mock.FindRequest("POST", "/upload3")
	if req == nil || req.Headers.Get("Authorization") != "Bearer jwt-mp" {
		t.Errorf("expected Bearer auth, got %v", req)
	}
}

func TestDoMultipartRequest_HTTPError_Cov(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/uploadbad", RespondString(http.StatusInternalServerError, "application/json", `{"message":"kaboom"}`))

	client := mock.Client()
	err := client.DoMultipartRequest("POST", mock.URL+"/api/uploadbad",
		bytes.NewBufferString("x"), nil, nil)
	if err == nil {
		t.Fatal("expected error on non-2xx response")
	}
}

func TestDoMultipartRequest_BadURL_Cov(t *testing.T) {
	client := &APIClient{APIKey: "k"}
	err := client.DoMultipartRequest("POST", "://bad-url",
		bytes.NewBufferString("x"), nil, nil)
	if err == nil {
		t.Fatal("expected error from invalid URL")
	}
}
