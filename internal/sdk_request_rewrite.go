package internal

import (
	"bytes"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
)

// The Portainer Go SDK (client-api-go) serializes the int64 slice TagIds as
// repeated multipart form fields (one "TagIds=N" part per id), which is what
// go-openapi's runtime emits for an array form parameter with an empty
// collection format. Portainer's POST /endpoints handler does not accept that
// shape and rejects the request with
//
//	HTTP 400 {"message":"Invalid request payload","details":"Invalid TagIds parameter"}
//
// It only accepts a single TagIds part whose value is a JSON-array string,
// e.g. "TagIds=[1,2,3]" — which is what the manual multipart code from
// terraform-provider-portainer v1.23.0 used to send.
//
// tagIDsRewriteTransport sits in the HTTP transport chain and, on POST
// /endpoints requests using multipart/form-data, merges all repeated TagIds
// parts into a single bracketed part. All other parts (including TLS file
// uploads) pass through with their headers and bodies untouched.

type tagIDsRewriteTransport struct {
	next http.RoundTripper
}

// affects reports whether this request is one we know carries multipart
// TagIds and which Portainer rejects in the SDK's emitted shape.
func (t *tagIDsRewriteTransport) affects(req *http.Request) bool {
	if req.Method != http.MethodPost {
		return false
	}
	p := req.URL.Path
	return p == "/endpoints" || strings.HasSuffix(p, "/api/endpoints")
}

func (t *tagIDsRewriteTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	if req.Body == nil || !t.affects(req) {
		return t.next.RoundTrip(req)
	}
	mediaType, params, err := mime.ParseMediaType(req.Header.Get("Content-Type"))
	if err != nil || mediaType != "multipart/form-data" {
		return t.next.RoundTrip(req)
	}
	boundary := params["boundary"]
	if boundary == "" {
		return t.next.RoundTrip(req)
	}

	body, err := io.ReadAll(req.Body)
	_ = req.Body.Close()
	if err != nil {
		return nil, err
	}

	// Fast path: nothing to rewrite if no TagIds part is present.
	if !bytes.Contains(body, []byte(`name="TagIds"`)) {
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		return t.next.RoundTrip(req)
	}

	rewritten, newBoundary, rewriteErr := rewriteTagIDsMultipart(body, boundary)
	if rewriteErr != nil {
		// On any parse/rewrite failure, fall through with the original body.
		// The Portainer error (if any) will then reach the caller via the
		// error-capture transport and surface the underlying cause.
		req.Body = io.NopCloser(bytes.NewReader(body))
		req.ContentLength = int64(len(body))
		return t.next.RoundTrip(req)
	}

	req.Body = io.NopCloser(bytes.NewReader(rewritten))
	req.ContentLength = int64(len(rewritten))
	req.Header.Set("Content-Type", "multipart/form-data; boundary="+newBoundary)
	return t.next.RoundTrip(req)
}

// rewriteTagIDsMultipart parses the multipart body and returns a new one with
// all repeated TagIds parts merged into a single "TagIds=[N1,N2,...]" part.
// The new boundary is chosen by mime/multipart.Writer and returned to the
// caller so the Content-Type header can be updated to match.
func rewriteTagIDsMultipart(body []byte, boundary string) ([]byte, string, error) {
	mr := multipart.NewReader(bytes.NewReader(body), boundary)
	type rawPart struct {
		header textproto.MIMEHeader
		body   []byte
	}
	var (
		nonTagParts []rawPart
		tagValues   []string
	)
	for {
		p, nextErr := mr.NextRawPart()
		if nextErr == io.EOF {
			break
		}
		if nextErr != nil {
			return nil, "", fmt.Errorf("parse multipart: %w", nextErr)
		}
		partBody, readErr := io.ReadAll(p)
		if readErr != nil {
			return nil, "", fmt.Errorf("read part: %w", readErr)
		}
		if p.FormName() == "TagIds" {
			v := strings.TrimSpace(string(partBody))
			if v != "" {
				tagValues = append(tagValues, v)
			}
			continue
		}
		hdr := make(textproto.MIMEHeader, len(p.Header))
		for k, v := range p.Header {
			hdr[k] = append([]string(nil), v...)
		}
		nonTagParts = append(nonTagParts, rawPart{header: hdr, body: partBody})
	}

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, part := range nonTagParts {
		writer, err := w.CreatePart(part.header)
		if err != nil {
			return nil, "", fmt.Errorf("write part: %w", err)
		}
		if _, err := writer.Write(part.body); err != nil {
			return nil, "", fmt.Errorf("write part body: %w", err)
		}
	}
	bracketed := "[" + strings.Join(tagValues, ",") + "]"
	if err := w.WriteField("TagIds", bracketed); err != nil {
		return nil, "", fmt.Errorf("write merged TagIds: %w", err)
	}
	if err := w.Close(); err != nil {
		return nil, "", fmt.Errorf("close multipart writer: %w", err)
	}
	return buf.Bytes(), w.Boundary(), nil
}
