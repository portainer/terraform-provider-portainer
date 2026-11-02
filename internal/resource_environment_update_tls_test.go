package internal

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/go-cty/cty"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// These tests cover Update of a directly connected environment:
//   - false TLS flags have to reach Portainer, which the SDK payload model
//     (omitempty bools) silently dropped, so TLS could never be switched off;
//   - a changed certificate has to be uploaded to the environment's TLS folder,
//     because PUT /endpoints/{id} cannot carry file content;
//   - removing every tag has to send an empty list, while tags and policies the
//     configuration does not touch are left alone (null).

// environmentTLSUpdate returns the ResourceData Terraform hands to Update when
// the configuration differs from the stored TLS environment by changes. It goes
// through a real diff so HasChange reports what a plan would.
func environmentTLSUpdate(t *testing.T, changes map[string]interface{}) *schema.ResourceData {
	t.Helper()
	r := resourceEnvironment()
	state := &terraform.InstanceState{
		ID: "7",
		Attributes: map[string]string{
			"id":                     "7",
			"name":                   "prod",
			"type":                   "1",
			"group_id":               "1",
			"environment_address":    "tcp://docker.example:2376",
			"tls_enabled":            "true",
			"tls_skip_verify":        "false",
			"tls_skip_client_verify": "false",
			"tls_ca_cert":            "OLD-CA",
			"tls_cert":               "OLD-CERT",
			"tls_key":                "OLD-KEY",
			"tag_ids.#":              "1",
			"tag_ids.0":              "3",
		},
	}
	config := map[string]interface{}{
		"name":                   "prod",
		"type":                   1,
		"group_id":               1,
		"environment_address":    "tcp://docker.example:2376",
		"tls_enabled":            true,
		"tls_skip_verify":        false,
		"tls_skip_client_verify": false,
		"tls_ca_cert":            "OLD-CA",
		"tls_cert":               "OLD-CERT",
		"tls_key":                "OLD-KEY",
		"tag_ids":                []interface{}{3},
	}
	for k, v := range changes {
		config[k] = v
	}

	diff, err := r.Diff(context.Background(), state, terraform.NewResourceConfigRaw(config), nil)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	d, err := schema.InternalMap(r.SchemaMap()).Data(state, diff)
	if err != nil {
		t.Fatalf("Data failed: %v", err)
	}
	return d
}

func environmentTLSMock(t *testing.T, uploads ...string) *MockServer {
	mock := NewMockServer(t)
	if uploads == nil {
		uploads = []string{"ca", "cert", "key"}
	}
	mock.On("PUT", "/endpoints/7", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 7}))
	mock.On("GET", "/endpoints/7", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 7, "Name": "prod", "Type": 1, "GroupId": 1}))
	for _, kind := range uploads {
		mock.On("POST", "/upload/tls/"+kind, RespondJSON(http.StatusOK, map[string]interface{}{}))
	}
	return mock
}

func decodePut(t *testing.T, mock *MockServer) map[string]interface{} {
	t.Helper()
	put := mock.FindRequest("PUT", "/endpoints/7")
	if put == nil {
		t.Fatal("expected PUT /endpoints/7")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode PUT body: %v", err)
	}
	return payload
}

func TestEnvironmentUpdate_DisablesTLS(t *testing.T) {
	mock := environmentTLSMock(t)
	d := environmentTLSUpdate(t, map[string]interface{}{"tls_enabled": false, "tls_skip_verify": true})

	if err := rcUpdate(resourceEnvironment(), d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	payload := decodePut(t, mock)
	for key, want := range map[string]bool{"tls": false, "tlsskipVerify": true, "tlsskipClientVerify": false} {
		got, present := payload[key]
		if !present || got != want {
			t.Errorf("payload.%s: expected %v to be sent, got %v (present=%v)", key, want, got, present)
		}
	}
	for _, kind := range []string{"ca", "cert", "key"} {
		if mock.FindRequest("POST", "/upload/tls/"+kind) != nil {
			t.Errorf("expected no %s upload with TLS disabled", kind)
		}
	}
}

func TestEnvironmentUpdate_UploadsRotatedCertificate(t *testing.T) {
	mock := environmentTLSMock(t)
	d := environmentTLSUpdate(t, map[string]interface{}{"tls_cert": "NEW-CERT", "tls_key": "NEW-KEY"})

	if err := rcUpdate(resourceEnvironment(), d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// The whole set is uploaded, the CA included: Portainer takes them as a
	// unit and the CA may have been removed while verification was skipped.
	for kind, want := range map[string]string{"ca": "OLD-CA", "cert": "NEW-CERT", "key": "NEW-KEY"} {
		req := mock.FindRequest("POST", "/upload/tls/"+kind)
		if req == nil {
			t.Errorf("expected POST /upload/tls/%s", kind)
			continue
		}
		folder, content := readTLSUpload(t, req)
		if folder != "7" || content != want {
			t.Errorf("upload %s: expected folder 7 and %q, got folder %q and %q", kind, want, folder, content)
		}
	}

	// The upload must precede the update, which reloads the proxy from disk.
	requests := mock.Requests()
	order := make([]string, 0, len(requests))
	for _, r := range requests {
		order = append(order, r.Method+" "+r.Path)
	}
	if got := strings.Join(order, ","); !strings.HasPrefix(got, "POST /upload/tls/ca,POST /upload/tls/cert,POST /upload/tls/key,PUT /endpoints/7") {
		t.Errorf("expected the uploads before PUT /endpoints/7, got %v", order)
	}
}

func TestEnvironmentUpdate_SkipsUploadWithoutTLSChange(t *testing.T) {
	mock := environmentTLSMock(t)
	d := environmentTLSUpdate(t, map[string]interface{}{"name": "prod-renamed"})

	if err := rcUpdate(resourceEnvironment(), d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	for _, kind := range []string{"ca", "cert", "key"} {
		if mock.FindRequest("POST", "/upload/tls/"+kind) != nil {
			t.Errorf("expected no %s upload when no TLS attribute changed", kind)
		}
	}
	payload := decodePut(t, mock)
	if tags, ok := payload["tagIDs"].([]interface{}); !ok || len(tags) != 1 {
		t.Errorf("payload.tagIDs: expected the unchanged [3], got %v", payload["tagIDs"])
	}
	if payload["userAccessPolicies"] != nil || payload["teamAccessPolicies"] != nil {
		t.Errorf("expected access policies left alone (null), got %v / %v", payload["userAccessPolicies"], payload["teamAccessPolicies"])
	}
}

func TestEnvironmentUpdate_ClearsTags(t *testing.T) {
	mock := environmentTLSMock(t)
	d := environmentTLSUpdate(t, map[string]interface{}{"tag_ids": []interface{}{}})

	if err := rcUpdate(resourceEnvironment(), d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	tags, ok := decodePut(t, mock)["tagIDs"].([]interface{})
	if !ok || len(tags) != 0 {
		t.Errorf("payload.tagIDs: expected an empty list to clear the tags, got %v", tags)
	}
}

func TestEnvironmentUpdate_UploadFailureStopsUpdate(t *testing.T) {
	mock := environmentTLSMock(t, "ca", "key")
	mock.On("POST", "/upload/tls/cert", RespondJSON(http.StatusBadRequest, map[string]string{"message": "Invalid certificate file"}))
	d := environmentTLSUpdate(t, map[string]interface{}{"tls_cert": "NEW-CERT"})

	err := rcUpdate(resourceEnvironment(), d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "Invalid certificate file") {
		t.Fatalf("expected the upload error, got %v", err)
	}
	if mock.FindRequest("PUT", "/endpoints/7") != nil {
		t.Error("expected no PUT after a failed upload")
	}
}

func readTLSUpload(t *testing.T, req *RecordedRequest) (folder, content string) {
	t.Helper()
	_, params, err := mime.ParseMediaType(req.Headers.Get("Content-Type"))
	if err != nil {
		t.Fatalf("parse content type: %v", err)
	}
	form, err := multipart.NewReader(strings.NewReader(string(req.Body)), params["boundary"]).ReadForm(1 << 20)
	if err != nil {
		t.Fatalf("read multipart form: %v", err)
	}
	if v := form.Value["folder"]; len(v) == 1 {
		folder = v[0]
	}
	if files := form.File["file"]; len(files) == 1 {
		f, err := files[0].Open()
		if err != nil {
			t.Fatalf("open uploaded file: %v", err)
		}
		defer f.Close()
		raw, _ := io.ReadAll(f)
		content = string(raw)
	}
	return folder, content
}

// TestEnvironmentRead_RefreshesTLS: TLS switched in the Portainer UI has to
// show up in the plan, which it never did while Read ignored TLSConfig.
func TestEnvironmentRead_RefreshesTLS(t *testing.T) {
	cases := []struct {
		name    string
		tls     map[string]interface{}
		envType int
		want    map[string]bool
	}{
		{
			"tls with client cert",
			map[string]interface{}{"TLS": true, "TLSSkipVerify": false, "TLSCert": "/data/tls/7/cert.pem"},
			1,
			map[string]bool{"tls_enabled": true, "tls_skip_verify": false, "tls_skip_client_verify": false},
		},
		{
			"tls off keeps skip flags",
			map[string]interface{}{"TLS": false},
			1,
			map[string]bool{"tls_enabled": false, "tls_skip_verify": true, "tls_skip_client_verify": true},
		},
		{
			"edge environment ignored",
			map[string]interface{}{"TLS": false},
			4,
			map[string]bool{"tls_enabled": true},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("GET", "/endpoints/7", RespondJSON(http.StatusOK, map[string]interface{}{
				"Id": 7, "Name": "prod", "Type": tc.envType, "GroupId": 1, "TLSConfig": tc.tls,
			}))
			r := resourceEnvironment()
			d := r.TestResourceData()
			d.SetId("7")
			_ = d.Set("tls_enabled", true)
			_ = d.Set("tls_skip_verify", true)
			_ = d.Set("tls_skip_client_verify", true)

			if err := rcRead(r, d, mock.Client()); err != nil {
				t.Fatalf("Read failed: %v", err)
			}
			for k, v := range tc.want {
				if got := d.Get(k).(bool); got != v {
					t.Errorf("%s: expected %v, got %v", k, v, got)
				}
			}
		})
	}
}

// TestEnvironmentUpdate_UploadsWriteOnlyKey: the client key given as
// tls_key_wo is never in state and has to come from the configuration when
// tls_key_wo_version changes.
func TestEnvironmentUpdate_UploadsWriteOnlyKey(t *testing.T) {
	mock := environmentTLSMock(t)
	r := resourceEnvironment()
	state := &terraform.InstanceState{
		ID: "7",
		Attributes: map[string]string{
			"id": "7", "name": "prod", "type": "1", "group_id": "1",
			"environment_address": "tcp://docker.example:2376",
			"tls_enabled":         "true", "tls_skip_verify": "true", "tls_skip_client_verify": "false",
			"tls_cert": "CERT", "tls_key_wo_version": "1",
		},
		RawConfig: cty.ObjectVal(map[string]cty.Value{"tls_key_wo": cty.StringVal("WO-KEY")}),
	}
	diff := &terraform.InstanceDiff{Attributes: map[string]*terraform.ResourceAttrDiff{
		"tls_key_wo_version": {Old: "1", New: "2"},
	}}
	d, err := schema.InternalMap(r.SchemaMap()).Data(state, diff)
	if err != nil {
		t.Fatalf("Data failed: %v", err)
	}

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	req := mock.FindRequest("POST", "/upload/tls/key")
	if req == nil {
		t.Fatal("expected the write-only key to be uploaded")
	}
	if _, content := readTLSUpload(t, req); content != "WO-KEY" {
		t.Errorf("uploaded key: expected the write-only value, got %q", content)
	}
	if d.Get("tls_key").(string) != "" {
		t.Error("tls_key must stay empty in state")
	}
}

// TestEnvironmentUpdate_PublicURLFollowsAddress covers public_ip left out of
// the configuration: being computed, it holds the old URL in state, which was
// sent again after the address changed, so published-port links kept pointing
// at the old host. An unchanged address leaves Portainer's public URL alone.
func TestEnvironmentUpdate_PublicURLFollowsAddress(t *testing.T) {
	cases := []struct {
		name    string
		changes map[string]interface{}
		want    interface{}
	}{
		{"address changed", map[string]interface{}{"environment_address": "tcp://other.example:2376"}, "tcp://other.example:2376"},
		{"address unchanged", map[string]interface{}{"name": "renamed"}, nil},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := environmentTLSMock(t)
			d := environmentTLSUpdate(t, tc.changes)
			if err := rcUpdate(resourceEnvironment(), d, mock.Client()); err != nil {
				t.Fatalf("Update failed: %v", err)
			}
			if got := decodePut(t, mock)["publicURL"]; got != tc.want {
				t.Errorf("publicURL: expected %v, got %v", tc.want, got)
			}
		})
	}
}
