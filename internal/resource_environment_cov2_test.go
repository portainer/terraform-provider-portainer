package internal

import (
	"net/http"
	"strings"
	"testing"
)

// These tests cover environment branches the base suite skips: the TLS
// client-cert upload path in Create (tls_enabled && !tls_skip_verify) with a
// public_ip, the access-policy branches in Update, and the Create → Update
// follow-up triggered when access policies are configured.

// TestEnvironmentCov2_Create_TLSCertUpload exercises the TLS file-upload branch
// in Create: with TLS enabled and verification NOT skipped, the CA/cert/key
// fields are attached to the multipart form. Also covers the public_ip branch.
func TestEnvironmentCov2_Create_TLSCertUpload(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/endpoints", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   17,
		"Name": "tls-env",
		"Type": 1,
	}))
	mock.On("GET", "/endpoints/17", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":        17,
		"Name":      "tls-env",
		"Type":      1,
		"GroupId":   1,
		"URL":       "tcp://docker.example:2376",
		"PublicURL": "docker.example",
		"TagIds":    []int{},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	_ = d.Set("name", "tls-env")
	_ = d.Set("environment_address", "tcp://docker.example:2376")
	_ = d.Set("type", 1)
	_ = d.Set("public_ip", "docker.example")
	_ = d.Set("tls_enabled", true)
	_ = d.Set("tls_skip_verify", false)
	_ = d.Set("tls_ca_cert", "CA-CERT-PEM")
	_ = d.Set("tls_cert", "CLIENT-CERT-PEM")
	_ = d.Set("tls_key", "CLIENT-KEY-PEM")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "17" {
		t.Errorf("expected ID %q, got %q", "17", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints")
	if post == nil {
		t.Fatal("expected POST /endpoints to be sent")
	}
	body := string(post.Body)
	// The cert/key file contents should be embedded in the multipart body.
	if !strings.Contains(body, "CA-CERT-PEM") {
		t.Errorf("expected multipart body to carry the CA cert, body=%q", body)
	}
	if !strings.Contains(body, "CLIENT-CERT-PEM") {
		t.Errorf("expected multipart body to carry the client cert")
	}
	if !strings.Contains(body, "CLIENT-KEY-PEM") {
		t.Errorf("expected multipart body to carry the client key")
	}
	if got := d.Get("public_ip"); got != "docker.example" {
		t.Errorf("public_ip: expected %q, got %v", "docker.example", got)
	}
}

// TestEnvironmentCov2_Update_AccessPolicies covers the user_access_policies and
// team_access_policies branches of Update for a non-edge environment.
func TestEnvironmentCov2_Update_AccessPolicies(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/endpoints/31", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   31,
		"Name": "prod",
		"Type": 1,
	}))
	mock.On("GET", "/endpoints/31", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":      31,
		"Name":    "prod",
		"Type":    1,
		"GroupId": 1,
		"URL":     "tcp://docker.example:2375",
		"TagIds":  []int{},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("31")
	_ = d.Set("name", "prod")
	_ = d.Set("environment_address", "tcp://docker.example:2375")
	_ = d.Set("type", 1)
	_ = d.Set("group_id", 1)
	_ = d.Set("user_access_policies", map[string]interface{}{"3": 1})
	_ = d.Set("team_access_policies", map[string]interface{}{"5": 2})

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/endpoints/31")
	if put == nil {
		t.Fatal("expected PUT /endpoints/31 to be sent")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	if _, ok := payload["userAccessPolicies"]; !ok {
		t.Errorf("expected userAccessPolicies in PUT body, got keys: %v", payload)
	}
	if _, ok := payload["teamAccessPolicies"]; !ok {
		t.Errorf("expected teamAccessPolicies in PUT body, got keys: %v", payload)
	}
}

// TestEnvironmentCov2_Create_WithAccessPolicies covers the Create → Update
// follow-up: when user/team access policies are set, Create issues the POST,
// then delegates to Update (PUT) to apply the policies.
func TestEnvironmentCov2_Create_WithAccessPolicies(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/endpoints", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   44,
		"Name": "with-policies",
		"Type": 1,
	}))
	mock.On("PUT", "/endpoints/44", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   44,
		"Name": "with-policies",
		"Type": 1,
	}))
	mock.On("GET", "/endpoints/44", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":      44,
		"Name":    "with-policies",
		"Type":    1,
		"GroupId": 1,
		"URL":     "tcp://docker.example:2375",
		"TagIds":  []int{},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	_ = d.Set("name", "with-policies")
	_ = d.Set("environment_address", "tcp://docker.example:2375")
	_ = d.Set("type", 1)
	_ = d.Set("user_access_policies", map[string]interface{}{"7": 3})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "44" {
		t.Errorf("expected ID %q, got %q", "44", d.Id())
	}
	if mock.FindRequest("POST", "/endpoints") == nil {
		t.Error("expected POST /endpoints to be sent")
	}
	if mock.FindRequest("PUT", "/endpoints/44") == nil {
		t.Error("expected Create to delegate to Update (PUT /endpoints/44) when access policies are set")
	}
}
