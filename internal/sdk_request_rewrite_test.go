package internal

import (
	"bytes"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// buildMultipart constructs a multipart/form-data body. Fields whose value is
// nil are skipped; fields with a string value emit a single text part. The
// special key "" (empty) is reserved by callers as a placeholder for raw
// repeated TagIds entries — see buildMultipartRepeatedTagIDs.
func buildMultipart(fields []struct{ Name, Value string }) (body, contentType string) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, f := range fields {
		_ = w.WriteField(f.Name, f.Value)
	}
	_ = w.Close()
	return buf.String(), w.FormDataContentType()
}

func parsePartsByName(t *testing.T, body, contentType string) map[string][]string {
	t.Helper()
	_, params, err := mimeParseContentType(contentType)
	if err != nil {
		t.Fatalf("parse content-type: %v", err)
	}
	mr := multipart.NewReader(strings.NewReader(body), params["boundary"])
	out := map[string][]string{}
	for {
		p, err := mr.NextRawPart()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("read part: %v", err)
		}
		b, _ := io.ReadAll(p)
		out[p.FormName()] = append(out[p.FormName()], string(b))
	}
	return out
}

// mimeParseContentType is a thin wrapper to keep intent obvious at call sites.
func mimeParseContentType(ct string) (mediaType string, params map[string]string, err error) {
	return mime.ParseMediaType(ct)
}

// TestRewriteTagIDsMultipart_NoTagIds verifies that calling the rewrite on a
// body without TagIds still produces a valid multipart with the other fields
// preserved (it does emit an empty "TagIds=[]" because the caller should have
// short-circuited via the fast path before reaching this function).
func TestRewriteTagIDsMultipart_NoTagIds(t *testing.T) {
	body, ct := buildMultipart([]struct{ Name, Value string }{
		{"Name", "x"}, {"URL", "u"},
	})
	_, params, _ := mimeParseContentType(ct)
	out, newBoundary, err := rewriteTagIDsMultipart([]byte(body), params["boundary"])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parts := parsePartsByName(t, string(out), "multipart/form-data; boundary="+newBoundary)
	if got, want := parts["Name"], []string{"x"}; !equalStrings(got, want) {
		t.Errorf("Name=%v, want %v", got, want)
	}
	if got, want := parts["URL"], []string{"u"}; !equalStrings(got, want) {
		t.Errorf("URL=%v, want %v", got, want)
	}
	// Defensive: rewriteTagIDsMultipart always emits a TagIds part. Callers
	// gate on the fast-path bytes.Contains check.
	if got, want := parts["TagIds"], []string{"[]"}; !equalStrings(got, want) {
		t.Errorf("TagIds=%v, want [\"[]\"]", got)
	}
}

func TestRewriteTagIDsMultipart_SingleTagID(t *testing.T) {
	body, ct := buildMultipart([]struct{ Name, Value string }{
		{"Name", "x"}, {"TagIds", "7"}, {"URL", "u"},
	})
	_, params, _ := mimeParseContentType(ct)
	out, newBoundary, err := rewriteTagIDsMultipart([]byte(body), params["boundary"])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parts := parsePartsByName(t, string(out), "multipart/form-data; boundary="+newBoundary)
	if got, want := parts["TagIds"], []string{"[7]"}; !equalStrings(got, want) {
		t.Errorf("TagIds=%v, want %v", got, want)
	}
	if got, want := parts["Name"], []string{"x"}; !equalStrings(got, want) {
		t.Errorf("Name=%v, want %v", got, want)
	}
	if got, want := parts["URL"], []string{"u"}; !equalStrings(got, want) {
		t.Errorf("URL=%v, want %v", got, want)
	}
}

func TestRewriteTagIDsMultipart_MultipleTagIDs(t *testing.T) {
	body, ct := buildMultipart([]struct{ Name, Value string }{
		{"Name", "x"},
		{"TagIds", "1"},
		{"TagIds", "2"},
		{"TagIds", "3"},
		{"URL", "u"},
	})
	_, params, _ := mimeParseContentType(ct)
	out, newBoundary, err := rewriteTagIDsMultipart([]byte(body), params["boundary"])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	parts := parsePartsByName(t, string(out), "multipart/form-data; boundary="+newBoundary)
	if got, want := parts["TagIds"], []string{"[1,2,3]"}; !equalStrings(got, want) {
		t.Errorf("TagIds=%v, want %v", got, want)
	}
}

// TestTagIDsRewriteTransport_OnlyPOSTEndpoints verifies the middleware leaves
// unrelated requests alone (different path, different method, or different
// content type).
func TestTagIDsRewriteTransport_OnlyPOSTEndpoints(t *testing.T) {
	cases := []struct {
		name        string
		method      string
		path        string
		contentType string
		body        string
		wantTouched bool
	}{
		{
			name:        "POST /endpoints multipart with TagIds → touched",
			method:      http.MethodPost,
			path:        "/api/endpoints",
			contentType: "", // filled in by build below
			wantTouched: true,
		},
		{
			name:        "PUT /endpoints/1 JSON → not touched",
			method:      http.MethodPut,
			path:        "/api/endpoints/1",
			contentType: "application/json",
			body:        `{"tagIDs":[1,2]}`,
			wantTouched: false,
		},
		{
			name:        "GET /endpoints → not touched",
			method:      http.MethodGet,
			path:        "/api/endpoints",
			contentType: "",
			wantTouched: false,
		},
		{
			name:        "POST /endpoints urlencoded → not touched",
			method:      http.MethodPost,
			path:        "/api/endpoints",
			contentType: "application/x-www-form-urlencoded",
			body:        "TagIds=1&TagIds=2",
			wantTouched: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var received []byte
			var receivedCT string
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				received, _ = io.ReadAll(r.Body)
				receivedCT = r.Header.Get("Content-Type")
				w.WriteHeader(http.StatusOK)
			}))
			defer srv.Close()

			var (
				reqBody     io.Reader
				contentType string
			)
			if tc.contentType == "" && tc.method == http.MethodPost {
				body, ct := buildMultipart([]struct{ Name, Value string }{
					{"Name", "x"}, {"TagIds", "1"}, {"TagIds", "2"},
				})
				reqBody = strings.NewReader(body)
				contentType = ct
			} else {
				if tc.body != "" {
					reqBody = strings.NewReader(tc.body)
				}
				contentType = tc.contentType
			}

			req, _ := http.NewRequest(tc.method, srv.URL+tc.path, reqBody)
			if contentType != "" {
				req.Header.Set("Content-Type", contentType)
			}

			rt := &tagIDsRewriteTransport{next: http.DefaultTransport}
			resp, err := rt.RoundTrip(req)
			if err != nil {
				t.Fatalf("RoundTrip: %v", err)
			}
			_ = resp.Body.Close()

			if tc.wantTouched {
				if !strings.Contains(string(received), `name="TagIds"`+"\r\n\r\n[1,2]") {
					t.Errorf("expected merged TagIds=[1,2] in received body, got:\n%s", received)
				}
				if !strings.HasPrefix(receivedCT, "multipart/form-data; boundary=") {
					t.Errorf("expected multipart Content-Type, got %q", receivedCT)
				}
			} else {
				if tc.body != "" && string(received) != tc.body {
					t.Errorf("body should pass through untouched.\nwant: %q\ngot : %q", tc.body, received)
				}
				if tc.contentType != "" && receivedCT != tc.contentType {
					t.Errorf("Content-Type should pass through untouched; want %q got %q", tc.contentType, receivedCT)
				}
			}
		})
	}
}

// TestTagIDsRewriteTransport_PreservesOtherParts ensures the rewrite leaves
// non-TagIds fields intact when the request is the target shape.
func TestTagIDsRewriteTransport_PreservesOtherParts(t *testing.T) {
	body, ct := buildMultipart([]struct{ Name, Value string }{
		{"EndpointCreationType", "2"},
		{"Name", "my-server"},
		{"URL", "tcp://x:9001"},
		{"GroupID", "1"},
		{"TLS", "true"},
		{"TagIds", "7"},
		{"TagIds", "8"},
	})

	var received []byte
	var receivedCT string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received, _ = io.ReadAll(r.Body)
		receivedCT = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	req, _ := http.NewRequest(http.MethodPost, srv.URL+"/api/endpoints", strings.NewReader(body))
	req.Header.Set("Content-Type", ct)

	rt := &tagIDsRewriteTransport{next: http.DefaultTransport}
	resp, err := rt.RoundTrip(req)
	if err != nil {
		t.Fatalf("RoundTrip: %v", err)
	}
	_ = resp.Body.Close()

	parts := parsePartsByName(t, string(received), receivedCT)
	if got, want := parts["Name"], []string{"my-server"}; !equalStrings(got, want) {
		t.Errorf("Name=%v, want %v", got, want)
	}
	if got, want := parts["TagIds"], []string{"[7,8]"}; !equalStrings(got, want) {
		t.Errorf("TagIds=%v, want %v", got, want)
	}
	if got, want := parts["EndpointCreationType"], []string{"2"}; !equalStrings(got, want) {
		t.Errorf("EndpointCreationType=%v, want %v", got, want)
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
