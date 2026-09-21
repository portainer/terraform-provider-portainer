package internal

import (
	"context"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// --- parseManifest -----------------------------------------------------------

func TestParseManifest_Cov(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantErr  bool
		wantKind string // expected value at key "kind"
	}{
		{
			name:     "valid JSON",
			input:    `{"kind":"Pod","apiVersion":"v1"}`,
			wantKind: "Pod",
		},
		{
			name: "valid YAML",
			input: `kind: Service
apiVersion: v1
`,
			wantKind: "Service",
		},
		{
			name:    "invalid",
			input:   "\tnot: : valid: yaml: [",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseManifest(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (parsed=%v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got["kind"] != tt.wantKind {
				t.Errorf("kind: expected %q, got %v", tt.wantKind, got["kind"])
			}
		})
	}
}

// --- toIntSlice --------------------------------------------------------------

func TestToIntSlice_Cov(t *testing.T) {
	in := []interface{}{1, 2, 3}
	got := toIntSlice(in)
	want := []int{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("expected %v, got %v", want, got)
	}

	if got := toIntSlice([]interface{}{}); len(got) != 0 {
		t.Errorf("expected empty slice, got %v", got)
	}
}

// --- splitAndTrimCSV ---------------------------------------------------------

func TestSplitAndTrimCSV_Cov(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want []string
	}{
		{"normal", "a,b,c", []string{"a", "b", "c"}},
		{"trims whitespace", " a , b ,c ", []string{"a", "b", "c"}},
		{"drops empties", "a,,b,  ,c", []string{"a", "b", "c"}},
		{"empty string", "", []string{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := splitAndTrimCSV(tt.in)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

// --- contains ----------------------------------------------------------------

func TestContains_Cov(t *testing.T) {
	arr := []string{"x", "y", "z"}
	if !contains(arr, "y") {
		t.Error("expected contains to find 'y'")
	}
	if contains(arr, "q") {
		t.Error("expected contains to NOT find 'q'")
	}
	if contains(nil, "anything") {
		t.Error("expected contains(nil, ...) to be false")
	}
}

// --- mustMap -----------------------------------------------------------------

func TestMustMap_Cov(t *testing.T) {
	// nil -> empty map
	if m := mustMap(nil); m == nil || len(m) != 0 {
		t.Errorf("expected empty non-nil map for nil input, got %v", m)
	}
	// already a map -> returned as-is
	src := map[string]interface{}{"a": 1}
	if m := mustMap(src); !reflect.DeepEqual(m, src) {
		t.Errorf("expected %v, got %v", src, m)
	}
	// wrong type -> empty map
	if m := mustMap("not a map"); m == nil || len(m) != 0 {
		t.Errorf("expected empty map for wrong-typed input, got %v", m)
	}
}

// --- setAuthHeader -----------------------------------------------------------

func TestSetAuthHeader_Cov(t *testing.T) {
	newReq := func() *http.Request {
		req, _ := http.NewRequest(http.MethodGet, "http://example.com", nil)
		return req
	}

	// API key takes precedence and sets X-API-Key.
	req := newReq()
	if err := setAuthHeader(req, &APIClient{APIKey: "k", JWTToken: "j"}); err != nil {
		t.Fatalf("api-key: unexpected error: %v", err)
	}
	if got := req.Header.Get("X-API-Key"); got != "k" {
		t.Errorf("X-API-Key: expected %q, got %q", "k", got)
	}
	if got := req.Header.Get("Authorization"); got != "" {
		t.Errorf("Authorization should be empty when api key is set, got %q", got)
	}

	// JWT fallback sets a Bearer Authorization header.
	req = newReq()
	if err := setAuthHeader(req, &APIClient{JWTToken: "j"}); err != nil {
		t.Fatalf("jwt: unexpected error: %v", err)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer j" {
		t.Errorf("Authorization: expected %q, got %q", "Bearer j", got)
	}

	// Neither credential -> error, no headers set.
	req = newReq()
	if err := setAuthHeader(req, &APIClient{}); err == nil {
		t.Fatal("expected error when no credentials are configured, got nil")
	}
}

// --- removeFromStateContext --------------------------------------------------

func TestRemoveFromStateContext_Cov(t *testing.T) {
	r := &schema.Resource{Schema: map[string]*schema.Schema{}}
	d := r.TestResourceData()
	d.SetId("123")

	diags := removeFromStateContext(context.Background(), d, nil)
	if diags != nil {
		t.Errorf("expected nil diagnostics, got %v", diags)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// --- apiGET helpers ----------------------------------------------------------

func TestApiGET_Cov(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/things", RespondString(http.StatusOK, "application/json", `{"ok":true}`))

	client := mock.Client()
	body, err := apiGET(mock.URL+"/api/things", client.APIKey, client)
	if err != nil {
		t.Fatalf("apiGET failed: %v", err)
	}
	if string(body) != "{\"ok\":true}" {
		t.Errorf("unexpected body: %s", body)
	}

	// X-API-Key header was sent.
	req := mock.FindRequest("GET", "/things")
	if req == nil {
		t.Fatal("expected GET /things to be recorded")
	}
	if req.Headers.Get("X-API-Key") != "test-api-key" {
		t.Errorf("expected X-API-Key header, got %q", req.Headers.Get("X-API-Key"))
	}
}

func TestApiGET_NoAuth_Cov(t *testing.T) {
	mock := NewMockServer(t)
	client := mock.Client()
	client.APIKey = ""
	client.JWTToken = ""

	_, err := apiGET(mock.URL+"/api/things", "", client)
	if err == nil {
		t.Fatal("expected error when no auth method provided")
	}
}

func TestApiGET_JWT_Cov(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/things", RespondString(http.StatusOK, "application/json", `{}`))

	client := mock.Client()
	client.APIKey = ""
	client.JWTToken = "jwt-token-123"

	if _, err := apiGET(mock.URL+"/api/things", "", client); err != nil {
		t.Fatalf("apiGET with JWT failed: %v", err)
	}
	req := mock.FindRequest("GET", "/things")
	if req == nil || req.Headers.Get("Authorization") != "Bearer jwt-token-123" {
		t.Errorf("expected Bearer auth header, got %v", req)
	}
}

func TestApiGETWithCode_Cov(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/items", RespondString(http.StatusTeapot, "application/json", `boom`))

	client := mock.Client()
	body, code, err := apiGETWithCode(mock.URL+"/api/items", client.APIKey, client)
	if err != nil {
		t.Fatalf("apiGETWithCode failed: %v", err)
	}
	if code != http.StatusTeapot {
		t.Errorf("expected status %d, got %d", http.StatusTeapot, code)
	}
	if string(body) != "boom" {
		t.Errorf("unexpected body: %s", body)
	}
}

func TestApiGETWithCode_NoAuth_Cov(t *testing.T) {
	mock := NewMockServer(t)
	client := mock.Client()
	client.JWTToken = ""

	_, _, err := apiGETWithCode(mock.URL+"/api/items", "", client)
	if err == nil {
		t.Fatal("expected error when no auth method provided")
	}
}

func TestApiGETCtx_Cov(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/ctx", RespondString(http.StatusOK, "application/json", `ctxbody`))

	client := mock.Client()
	body, err := apiGETCtx(context.Background(), mock.URL+"/api/ctx", client.APIKey, client)
	if err != nil {
		t.Fatalf("apiGETCtx failed: %v", err)
	}
	if string(body) != "ctxbody" {
		t.Errorf("unexpected body: %s", body)
	}
}

func TestApiGETCtx_NoAuth_Cov(t *testing.T) {
	mock := NewMockServer(t)
	client := mock.Client()
	client.APIKey = ""
	client.JWTToken = ""

	_, err := apiGETCtx(context.Background(), mock.URL+"/api/ctx", "", client)
	if err == nil {
		t.Fatal("expected error when no auth method provided")
	}
}

func TestApiGETWithCodeCtx_Cov(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/ctxcode", RespondString(http.StatusCreated, "application/json", `made`))

	client := mock.Client()
	body, code, err := apiGETWithCodeCtx(context.Background(), mock.URL+"/api/ctxcode", client.APIKey, client)
	if err != nil {
		t.Fatalf("apiGETWithCodeCtx failed: %v", err)
	}
	if code != http.StatusCreated {
		t.Errorf("expected %d, got %d", http.StatusCreated, code)
	}
	if string(body) != "made" {
		t.Errorf("unexpected body: %s", body)
	}
}

func TestApiGETWithCodeCtx_NoAuth_Cov(t *testing.T) {
	mock := NewMockServer(t)
	client := mock.Client()
	client.JWTToken = ""

	_, _, err := apiGETWithCodeCtx(context.Background(), mock.URL+"/api/ctxcode", "", client)
	if err == nil {
		t.Fatal("expected error when no auth method provided")
	}
}

func TestApiPOSTWithCodeCtx_Cov(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/create", RespondString(http.StatusOK, "application/json", `posted`))

	client := mock.Client()
	body, code, err := apiPOSTWithCodeCtx(context.Background(), mock.URL+"/api/create", client.APIKey, client, []byte(`{"a":1}`))
	if err != nil {
		t.Fatalf("apiPOSTWithCodeCtx failed: %v", err)
	}
	if code != http.StatusOK {
		t.Errorf("expected 200, got %d", code)
	}
	if string(body) != "posted" {
		t.Errorf("unexpected body: %s", body)
	}

	req := mock.FindRequest("POST", "/create")
	if req == nil {
		t.Fatal("expected POST /create recorded")
	}
	if req.Headers.Get("Content-Type") != "application/json" {
		t.Errorf("expected JSON content-type, got %q", req.Headers.Get("Content-Type"))
	}
	if string(req.Body) != `{"a":1}` {
		t.Errorf("unexpected payload: %s", req.Body)
	}
}

func TestApiPOSTWithCodeCtx_NoAuth_Cov(t *testing.T) {
	mock := NewMockServer(t)
	client := mock.Client()
	client.JWTToken = ""

	_, _, err := apiPOSTWithCodeCtx(context.Background(), mock.URL+"/api/create", "", client, []byte(`{}`))
	if err == nil {
		t.Fatal("expected error when no auth method provided")
	}
}

func TestApiPUTWithCodeCtx_Cov(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/update", RespondString(http.StatusOK, "application/json", `put`))

	client := mock.Client()
	body, code, err := apiPUTWithCodeCtx(context.Background(), mock.URL+"/api/update", client.APIKey, client, []byte(`{"b":2}`))
	if err != nil {
		t.Fatalf("apiPUTWithCodeCtx failed: %v", err)
	}
	if code != http.StatusOK {
		t.Errorf("expected 200, got %d", code)
	}
	if string(body) != "put" {
		t.Errorf("unexpected body: %s", body)
	}

	req := mock.FindRequest("PUT", "/update")
	if req == nil || string(req.Body) != `{"b":2}` {
		t.Errorf("unexpected PUT request: %v", req)
	}
}

func TestApiPUTWithCodeCtx_NoAuth_Cov(t *testing.T) {
	mock := NewMockServer(t)
	client := mock.Client()
	client.JWTToken = ""

	_, _, err := apiPUTWithCodeCtx(context.Background(), mock.URL+"/api/update", "", client, []byte(`{}`))
	if err == nil {
		t.Fatal("expected error when no auth method provided")
	}
}

// TestDoJSONWithHeaders_AuthWinsOverCallerHeaders pins the authentication
// invariant of doJSONWithHeaders: a caller's header map cannot weaken or
// replace the credential the provider is configured with. Ordering alone is not
// enough — setAuthHeader sets X-API-Key or Authorization depending on the
// configuration, so a caller value in the branch it does not take would
// otherwise survive. Both are dropped from the caller's map instead.
func TestDoJSONWithHeaders_AuthWinsOverCallerHeaders(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/thing", RespondString(http.StatusNoContent, "", ""))

	client := mock.Client()
	headers := map[string]string{
		"X-Setup-Token": "token-123",
		"X-API-Key":     "", // a caller trying to blank the credential
		"Authorization": "Bearer attacker",
	}
	if err := doJSONWithHeaders(context.Background(), client, http.MethodPost,
		client.Endpoint+"/thing", map[string]string{"a": "b"}, nil, headers); err != nil {
		t.Fatalf("request failed: %v", err)
	}

	req := mock.FindRequest("POST", "/thing")
	if req == nil {
		t.Fatal("expected the request to reach the server")
	}
	if got := req.Headers.Get("X-API-Key"); got != client.APIKey || got == "" {
		t.Errorf("X-API-Key must survive a colliding caller header, got %q", got)
	}
	if got := req.Headers.Get("Authorization"); got != "" {
		t.Errorf("a caller must not be able to inject an Authorization header, got %q", got)
	}
	if got := req.Headers.Get("X-Setup-Token"); got != "token-123" {
		t.Errorf("a non-auth caller header must still be sent, got %q", got)
	}
	if got := req.Headers.Get("Content-Type"); got != "application/json" {
		t.Errorf("Content-Type: got %q", got)
	}
}

// TestRedactURL_HidesCredentialsInQuery covers the second layer of the fix:
// even a caller that does report an *apiStatusError verbatim must not spill a
// credential that travelled in the query string.
func TestRedactURL_HidesCredentialsInQuery(t *testing.T) {
	err := &apiStatusError{
		Method:     "GET",
		URL:        "https://portainer.example.com/api/omni/serviceaccount/validate?endpoint=https%3A%2F%2Fomni.example.com&serviceAccountKey=super-secret",
		StatusCode: 401,
		Body:       `{"message":"invalid service account key"}`,
	}

	got := err.Error()
	if strings.Contains(got, "super-secret") {
		t.Errorf("the credential must be redacted, got %q", got)
	}
	if !strings.Contains(got, "REDACTED") {
		t.Errorf("the parameter should still be visible as redacted, got %q", got)
	}
	// The rest of the message has to stay useful.
	for _, want := range []string{"omni/serviceaccount/validate", "401", "invalid service account key"} {
		if !strings.Contains(got, want) {
			t.Errorf("expected %q to survive redaction, got %q", want, got)
		}
	}
}

// TestRedactURL_LeavesOrdinaryURLsAlone keeps the redaction from mangling the
// errors that carry no secret at all.
func TestRedactURL_LeavesOrdinaryURLsAlone(t *testing.T) {
	raw := "https://portainer.example.com/api/stacks/5?endpointId=1"
	if got := redactURL(raw); got != raw {
		t.Errorf("a URL with no sensitive parameter must be untouched, got %q", got)
	}
	if got := redactURL("://not a url"); got != "://not a url" {
		t.Errorf("an unparsable URL must be returned as-is, got %q", got)
	}
}
