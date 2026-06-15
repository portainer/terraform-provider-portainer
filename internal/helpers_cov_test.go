package internal

import (
	"context"
	"net/http"
	"reflect"
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
