package internal

import (
	"encoding/base64"
	"net/http"
	"strings"
	"testing"
)

// TestDataSourceRegistryConnection_Reachable verifies the ping payload uses the
// API's PascalCase keys and that the outcome is reported, not raised.
func TestDataSourceRegistryConnection_Reachable(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/registries/ping", RespondJSON(http.StatusOK, map[string]interface{}{
		"success": true, "message": "registry is reachable",
	}))

	ds := dataSourceRegistryConnection()
	d := ds.TestResourceData()
	_ = d.Set("url", "registry.example.com")
	_ = d.Set("type", 3)
	_ = d.Set("username", "robot")
	_ = d.Set("password", "secret")
	_ = d.Set("tls", true)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("success"); got != true {
		t.Errorf("success: got %v", got)
	}

	post := mock.FindRequest("POST", "/registries/ping")
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	for k, want := range map[string]interface{}{
		"URL": "registry.example.com", "Type": float64(3),
		"Username": "robot", "Password": "secret", "TLS": true,
	} {
		if payload[k] != want {
			t.Errorf("payload.%s: expected %v, got %v", k, want, payload[k])
		}
	}
}

// TestDataSourceRegistryConnection_FailOnError verifies the opt-in failure mode
// and that the default only reports.
func TestDataSourceRegistryConnection_FailOnError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/registries/ping", RespondJSON(http.StatusOK, map[string]interface{}{
		"success": false, "message": "unauthorized",
	}))

	ds := dataSourceRegistryConnection()
	d := ds.TestResourceData()
	_ = d.Set("url", "registry.example.com")
	_ = d.Set("type", 3)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("an unreachable registry must be reported, not raised: %v", err)
	}
	if got := d.Get("message"); got != "unauthorized" {
		t.Errorf("message: got %v", got)
	}

	strict := dataSourceRegistryConnection()
	ds2 := strict.TestResourceData()
	_ = ds2.Set("url", "registry.example.com")
	_ = ds2.Set("type", 3)
	_ = ds2.Set("fail_on_error", true)
	err := rcRead(strict, ds2, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "unauthorized") {
		t.Errorf("expected the failure to carry the server's message, got: %v", err)
	}
}

// TestRegistryConfigure_SendsPayload verifies the configure payload, including
// the TLS material which Portainer models as []byte and therefore expects
// base64-encoded on the wire.
func TestRegistryConfigure_SendsPayload(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/registries/4/configure", RespondString(http.StatusNoContent, "", ""))

	r := resourceRegistryConfigure()
	d := r.TestResourceData()
	_ = d.Set("registry_id", 4)
	_ = d.Set("authentication", true)
	_ = d.Set("username", "robot")
	_ = d.Set("password", "secret")
	_ = d.Set("tls", true)
	_ = d.Set("tls_skip_verify", false)
	_ = d.Set("tls_ca_cert", "-----BEGIN CERTIFICATE-----\nca\n-----END CERTIFICATE-----")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "4/configure" {
		t.Errorf("id: got %q", d.Id())
	}

	post := mock.FindRequest("POST", "/registries/4/configure")
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload["Authentication"] != true || payload["Username"] != "robot" {
		t.Errorf("payload mismatch: %v", payload)
	}
	raw, ok := payload["TLSCACertFile"].(string)
	if !ok {
		t.Fatalf("TLSCACertFile should be a base64 string, got %T", payload["TLSCACertFile"])
	}
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		t.Fatalf("TLSCACertFile is not valid base64: %v", err)
	}
	if !strings.Contains(string(decoded), "BEGIN CERTIFICATE") {
		t.Errorf("decoded certificate mismatch: %q", decoded)
	}
	// Fields left unset must not be sent at all.
	if _, present := payload["Region"]; present {
		t.Error("Region must be omitted when it is not configured")
	}
}

// TestDataSourceLDAPCheck_Success verifies the 204 answer is read as success
// and that the settings are nested under LDAPSettings.
func TestDataSourceLDAPCheck_Success(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/ldap/check", RespondString(http.StatusNoContent, "", ""))

	ds := dataSourceLDAPCheck()
	d := ds.TestResourceData()
	_ = d.Set("url", "ldap.example.com:389")
	_ = d.Set("reader_dn", "cn=reader,dc=example,dc=com")
	_ = d.Set("password", "secret")
	_ = d.Set("start_tls", true)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("success"); got != true {
		t.Errorf("success: got %v", got)
	}

	post := mock.FindRequest("POST", "/ldap/check")
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	settings, ok := payload["LDAPSettings"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload should nest the settings under LDAPSettings, got %v", payload)
	}
	if settings["URL"] != "ldap.example.com:389" || settings["StartTLS"] != true {
		t.Errorf("settings mismatch: %v", settings)
	}
	if _, ok := settings["TLSConfig"].(map[string]interface{}); !ok {
		t.Errorf("TLSConfig should be nested, got %v", settings["TLSConfig"])
	}
}

// TestDataSourceLDAPCheck_FailureRaisesByDefault verifies that a directory
// which refuses the bind fails the read — the point of a pre-flight check — and
// that fail_on_error=false turns it back into a report.
func TestDataSourceLDAPCheck_FailureRaisesByDefault(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/ldap/check",
		RespondString(http.StatusBadRequest, "application/json", `{"message":"invalid credentials"}`))

	// The schema default of true is applied by Terraform during a plan, not by
	// TestResourceData, so it is set explicitly here; the default itself is
	// asserted separately below.
	if got := dataSourceLDAPCheck().Schema["fail_on_error"].Default; got != true {
		t.Fatalf("fail_on_error should default to true, got %v", got)
	}

	ds := dataSourceLDAPCheck()
	d := ds.TestResourceData()
	_ = d.Set("url", "ldap.example.com:389")
	_ = d.Set("fail_on_error", true)

	err := rcRead(ds, d, mock.Client())
	if err == nil {
		t.Fatal("expected the check to fail by default")
	}
	if !strings.Contains(err.Error(), "invalid credentials") {
		t.Errorf("error should carry the server's message, got: %v", err)
	}

	lenient := dataSourceLDAPCheck()
	dl := lenient.TestResourceData()
	_ = dl.Set("url", "ldap.example.com:389")
	_ = dl.Set("fail_on_error", false)
	if err := rcRead(lenient, dl, mock.Client()); err != nil {
		t.Fatalf("with fail_on_error=false the outcome must be reported: %v", err)
	}
	if got := dl.Get("success"); got != false {
		t.Errorf("success: expected false, got %v", got)
	}
}

// TestKubernetesPersistentVolume_SetsReclaimPolicy verifies the PUT and the
// follow-up read that adopts the volume's observed state.
func TestKubernetesPersistentVolume_SetsReclaimPolicy(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/kubernetes/2/persistent_volumes/reclaim_policy", RespondString(http.StatusNoContent, "", ""))
	mock.On("GET", "/kubernetes/2/persistent_volumes/pv-data", RespondJSON(http.StatusOK, map[string]interface{}{
		"spec": map[string]interface{}{
			"capacity":                      map[string]string{"storage": "10Gi"},
			"accessModes":                   []string{"ReadWriteOnce"},
			"persistentVolumeReclaimPolicy": "Retain",
			"storageClassName":              "fast",
			"claimRef":                      map[string]interface{}{"namespace": "prod", "name": "data"},
		},
		"status": map[string]interface{}{"phase": "Bound"},
	}))

	r := resourceKubernetesPersistentVolume()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 2)
	_ = d.Set("name", "pv-data")
	_ = d.Set("reclaim_policy", "Retain")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "2/pv-data" {
		t.Errorf("id: got %q", d.Id())
	}
	if got := d.Get("phase"); got != "Bound" {
		t.Errorf("phase: got %v", got)
	}
	if got := d.Get("capacity"); got != "10Gi" {
		t.Errorf("capacity: got %v", got)
	}
	if got := d.Get("claim_ref"); got != "prod/data" {
		t.Errorf("claim_ref: expected namespace/name, got %v", got)
	}

	put := mock.FindRequest("PUT", "/kubernetes/2/persistent_volumes/reclaim_policy")
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload["name"] != "pv-data" || payload["reclaimPolicy"] != "Retain" {
		t.Errorf("payload mismatch: %v", payload)
	}
}

// TestKubernetesPersistentVolume_DeleteSendsList verifies the delete endpoint
// is given a one-element list, which is the shape it takes.
func TestKubernetesPersistentVolume_DeleteSendsList(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/2/persistent_volumes/delete", RespondString(http.StatusNoContent, "", ""))

	r := resourceKubernetesPersistentVolume()
	d := r.TestResourceData()
	d.SetId("2/pv-data")
	_ = d.Set("environment_id", 2)
	_ = d.Set("name", "pv-data")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	post := mock.FindRequest("POST", "/kubernetes/2/persistent_volumes/delete")
	var payload []string
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if len(payload) != 1 || payload[0] != "pv-data" {
		t.Errorf("payload should be a one-element list, got %v", payload)
	}
}

// TestKubernetesPersistentVolume_Read404ClearsID covers a volume deleted
// outside Terraform.
func TestKubernetesPersistentVolume_Read404ClearsID(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/2/persistent_volumes/gone",
		RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`))

	r := resourceKubernetesPersistentVolume()
	d := r.TestResourceData()
	d.SetId("2/gone")
	_ = d.Set("environment_id", 2)
	_ = d.Set("name", "gone")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should not fail on 404: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected the ID to be cleared, got %q", d.Id())
	}
}
