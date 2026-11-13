package internal

import (
	"context"
	"net/http"
	"strings"
	"testing"
)

func scopedKubeconfigStatusJSON(active bool) map[string]interface{} {
	return map[string]interface{}{
		"id": 5, "active": active, "createdAt": 1000, "expiresAt": 4600,
		"description": "ci", "includeSecrets": false, "namespaces": []string{"team-a"},
		"serviceAccount": "portainer-scoped-5",
	}
}

// TestKubernetesScopedKubeconfigCreate verifies the payload, and that the
// credential, returned only by the issuing call, is kept in state.
func TestKubernetesScopedKubeconfigCreate(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/2/scoped_kubeconfig", RespondJSON(http.StatusCreated, map[string]interface{}{
		"id": 5, "kubeconfig": "apiVersion: v1\nkind: Config\n", "serviceAccount": "portainer-scoped-5",
		"namespaces": []string{"team-a"}, "expiresAt": 4600,
	}))
	mock.On("GET", "/kubernetes/2/scoped_kubeconfig/5", RespondJSON(http.StatusOK, scopedKubeconfigStatusJSON(true)))

	r := resourceKubernetesScopedKubeconfig()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 2)
	_ = d.Set("expiry_seconds", 3600)
	_ = d.Set("namespaces", []interface{}{"team-a"})
	_ = d.Set("description", "ci")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "2:5" {
		t.Errorf("ID: got %q", d.Id())
	}
	var payload map[string]interface{}
	if err := mock.FindRequest("POST", "/kubernetes/2/scoped_kubeconfig").DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["expirySeconds"] != float64(3600) || payload["description"] != "ci" || payload["includeSecrets"] != false {
		t.Errorf("payload: %v", payload)
	}
	if ns := payload["namespaces"].([]interface{}); len(ns) != 1 || ns[0] != "team-a" {
		t.Errorf("payload.namespaces: %v", ns)
	}
	if got := d.Get("kubeconfig"); !strings.Contains(got.(string), "kind: Config") {
		t.Errorf("kubeconfig: got %q", got)
	}
	if got := d.Get("created_at"); got != 1000 {
		t.Errorf("created_at: got %v", got)
	}
	if !r.Schema["kubeconfig"].Sensitive {
		t.Error("kubeconfig must be sensitive")
	}
}

// TestKubernetesScopedKubeconfigCreate_NamespacesOmitted verifies an unset
// namespace list is left out, which asks for every namespace the caller sees.
func TestKubernetesScopedKubeconfigCreate_NamespacesOmitted(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/2/scoped_kubeconfig", RespondJSON(http.StatusCreated, map[string]interface{}{"id": 5}))
	mock.On("GET", "/kubernetes/2/scoped_kubeconfig/5", RespondJSON(http.StatusOK, scopedKubeconfigStatusJSON(true)))

	r := resourceKubernetesScopedKubeconfig()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 2)
	_ = d.Set("expiry_seconds", 60)
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	var payload map[string]interface{}
	_ = mock.FindRequest("POST", "/kubernetes/2/scoped_kubeconfig").DecodeJSON(&payload)
	if _, ok := payload["namespaces"]; ok {
		t.Errorf("namespaces must be left out, got %v", payload["namespaces"])
	}
}

func TestKubernetesScopedKubeconfigCreate_Errors(t *testing.T) {
	r := resourceKubernetesScopedKubeconfig()

	old := NewMockServer(t)
	old.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "2.45.1", "ServerEdition": "BE",
	}))
	d := r.TestResourceData()
	_ = d.Set("environment_id", 2)
	_ = d.Set("expiry_seconds", 60)
	err := rcCreate(r, d, old.Client())
	if err == nil || !strings.Contains(err.Error(), "requires Portainer Business Edition 2.45.2 or later") {
		t.Errorf("expected the requirement to be named, got %v", err)
	}

	noID := NewMockServer(t)
	noID.On("POST", "/kubernetes/2/scoped_kubeconfig", RespondJSON(http.StatusCreated, map[string]interface{}{}))
	d = r.TestResourceData()
	_ = d.Set("environment_id", 2)
	if err := rcCreate(r, d, noID.Client()); err == nil {
		t.Error("expected an error when no identifier is returned")
	}
}

// TestKubernetesScopedKubeconfigRead_DropsInactive verifies an expired (or
// gone) kubeconfig leaves state, so the next apply issues a new one, and that
// a read keeps the credential, which the status never carries.
func TestKubernetesScopedKubeconfigRead_DropsInactive(t *testing.T) {
	r := resourceKubernetesScopedKubeconfig()

	active := NewMockServer(t)
	active.On("GET", "/kubernetes/2/scoped_kubeconfig/5", RespondJSON(http.StatusOK, scopedKubeconfigStatusJSON(true)))
	d := r.TestResourceData()
	d.SetId("2:5")
	_ = d.Set("kubeconfig", "secret")
	if err := rcRead(r, d, active.Client()); err != nil || d.Id() != "2:5" {
		t.Fatalf("active read: err=%v id=%q", err, d.Id())
	}
	if d.Get("kubeconfig") != "secret" {
		t.Error("a read must keep the credential")
	}

	expired := NewMockServer(t)
	expired.On("GET", "/kubernetes/2/scoped_kubeconfig/5", RespondJSON(http.StatusOK, scopedKubeconfigStatusJSON(false)))
	if err := rcRead(r, d, expired.Client()); err != nil || d.Id() != "" {
		t.Errorf("expired read: err=%v id=%q", err, d.Id())
	}

	gone := NewMockServer(t)
	d.SetId("2:5")
	if err := rcRead(r, d, gone.Client()); err != nil || d.Id() != "" {
		t.Errorf("404 read: err=%v id=%q", err, d.Id())
	}

	failing := NewMockServer(t)
	failing.On("GET", "/kubernetes/2/scoped_kubeconfig/5", RespondString(http.StatusInternalServerError, "", "boom"))
	d.SetId("2:5")
	if err := rcRead(r, d, failing.Client()); err == nil {
		t.Error("expected an error on 500")
	}

	d.SetId("5")
	if err := rcRead(r, d, failing.Client()); err == nil {
		t.Error("expected an error for a malformed ID")
	}
}

// TestKubernetesScopedKubeconfigDelete covers a revoke, an already gone one,
// and the 500 of a revoke whose cluster objects stayed, which only warns.
func TestKubernetesScopedKubeconfigDelete(t *testing.T) {
	r := resourceKubernetesScopedKubeconfig()
	for status, wantErr := range map[int]bool{
		http.StatusNoContent:           false,
		http.StatusNotFound:            false,
		http.StatusInternalServerError: false,
		http.StatusForbidden:           true,
	} {
		mock := NewMockServer(t)
		mock.On("DELETE", "/kubernetes/2/scoped_kubeconfig/5", RespondString(status, "", ""))
		d := r.TestResourceData()
		d.SetId("2:5")
		diags := r.DeleteContext(context.Background(), d, mock.Client())
		if diags.HasError() != wantErr {
			t.Errorf("status %d: HasError = %v, want %v (%v)", status, diags.HasError(), wantErr, diags)
		}
		if status == http.StatusInternalServerError && len(diags) != 1 {
			t.Errorf("status 500: expected one warning, got %v", diags)
		}
		if !wantErr && d.Id() != "" {
			t.Errorf("status %d: expected ID cleared", status)
		}
	}
}

func TestImportKubernetesScopedKubeconfig(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/2/scoped_kubeconfig/5", RespondJSON(http.StatusOK, scopedKubeconfigStatusJSON(true)))

	r := resourceKubernetesScopedKubeconfig()
	d := r.TestResourceData()
	d.SetId("2:5")
	if _, err := importKubernetesScopedKubeconfig(context.Background(), d, mock.Client()); err != nil {
		t.Fatalf("import failed: %v", err)
	}
	if d.Get("environment_id") != 2 || d.Get("expiry_seconds") != 3600 || d.Get("namespaces.0") != "team-a" {
		t.Errorf("import did not fill the arguments: %v %v %v", d.Get("environment_id"), d.Get("expiry_seconds"), d.Get("namespaces"))
	}

	d.SetId("2:6")
	if _, err := importKubernetesScopedKubeconfig(context.Background(), d, mock.Client()); err == nil {
		t.Error("expected an error for an unknown kubeconfig")
	}
	d.SetId("2-5")
	if _, err := importKubernetesScopedKubeconfig(context.Background(), d, mock.Client()); err == nil {
		t.Error("expected an error for a malformed ID")
	}
}

func TestDataSourceKubernetesScopedKubeconfigs(t *testing.T) {
	item := scopedKubeconfigStatusJSON(true)
	item["endpointId"], item["userId"], item["username"] = 2, 1, "admin"

	for name, tc := range map[string]struct {
		envID    int
		path, id string
	}{
		"all environments": {0, "/kubernetes/scoped_kubeconfigs", "portainer-scoped-kubeconfigs"},
		"one environment":  {2, "/kubernetes/2/scoped_kubeconfig", "2/scoped_kubeconfigs"},
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("GET", tc.path, RespondJSON(http.StatusOK, []map[string]interface{}{item}))
			ds := dataSourceKubernetesScopedKubeconfigs()
			d := ds.TestResourceData()
			if tc.envID != 0 {
				_ = d.Set("environment_id", tc.envID)
			}
			if err := rcRead(ds, d, mock.Client()); err != nil {
				t.Fatalf("Read failed: %v", err)
			}
			if d.Get("scoped_kubeconfigs.0.username") != "admin" || d.Get("scoped_kubeconfigs.0.environment_id") != 2 {
				t.Errorf("listing not mapped: %v", d.Get("scoped_kubeconfigs"))
			}
			if d.Id() != tc.id {
				t.Errorf("id: got %q", d.Id())
			}
		})
	}

	old := NewMockServer(t)
	old.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "2.45.3", "ServerEdition": "BE",
	}))
	ds := dataSourceKubernetesScopedKubeconfigs()
	d := ds.TestResourceData()
	if err := rcRead(ds, d, old.Client()); err == nil || !strings.Contains(err.Error(), "3.0 or later") {
		t.Errorf("expected the requirement to be named, got %v", err)
	}
}
