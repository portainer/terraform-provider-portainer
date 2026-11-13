package internal

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// --- users: CreatedAuthenticationMethod / LastAuthenticationMethod (3.0) ---

func TestUserRead_AuthenticationMethods(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/users/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 42, "Username": "alice", "Role": 1,
		"CreatedAuthenticationMethod": 2, "LastAuthenticationMethod": 3,
	}))

	r := resourceUser()
	d := r.TestResourceData()
	d.SetId("42")
	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("created_authentication_method"); got != "ldap" {
		t.Errorf("created_authentication_method: expected ldap, got %v", got)
	}
	if got := d.Get("last_authentication_method"); got != "oauth" {
		t.Errorf("last_authentication_method: expected oauth, got %v", got)
	}
}

// Before 3.0 the fields are absent and the attributes stay empty.
func TestUserRead_AuthenticationMethodsPre30(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/users/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 42, "Username": "alice", "Role": 1,
	}))

	r := resourceUser()
	d := r.TestResourceData()
	d.SetId("42")
	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("created_authentication_method"); got != "" {
		t.Errorf("created_authentication_method: expected empty, got %v", got)
	}
	if got := d.Get("last_authentication_method"); got != "" {
		t.Errorf("last_authentication_method: expected empty, got %v", got)
	}
}

func TestDataSourceUserRead_AuthenticationMethods(t *testing.T) {
	mock := NewMockServer(t)
	// As live on 3.0.1: the list leaves the fields out, GET /users/{id} has
	// them.
	mock.On("GET", "/users", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Username": "admin", "Role": 1},
		{"Id": 5, "Username": "alice", "Role": 2},
	}))
	mock.On("GET", "/users/5", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 5, "Username": "alice", "Role": 2, "CreatedAuthenticationMethod": 3,
	}))

	ds := dataSourceUser()
	d := ds.TestResourceData()
	_ = d.Set("username", "alice")
	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("created_authentication_method"); got != "oauth" {
		t.Errorf("created_authentication_method: expected oauth, got %v", got)
	}
	if got := d.Get("last_authentication_method"); got != "" {
		t.Errorf("last_authentication_method: expected empty, got %v", got)
	}
}

func TestAuthenticationMethodName(t *testing.T) {
	for in, want := range map[int]string{0: "", 1: "internal", 2: "ldap", 3: "oauth", 9: "9"} {
		if got := authenticationMethodName(in); got != want {
			t.Errorf("authenticationMethodName(%d) = %q, want %q", in, got, want)
		}
	}
}

// --- environments: delete_cluster_on_destroy (DELETE ?deleteCluster=true) ---

func shortenEnvironmentRemovePoll(t *testing.T) {
	t.Helper()
	old := environmentRemovePollInterval
	environmentRemovePollInterval = time.Millisecond
	t.Cleanup(func() { environmentRemovePollInterval = old })
}

// Without the flag the request is unchanged and nothing is polled.
func TestEnvironmentDelete_DefaultSendsNoDeleteCluster(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/endpoints/7", RespondJSON(http.StatusNoContent, nil))

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("7")
	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	req := mock.FindRequest("DELETE", "/endpoints/7")
	if req == nil || req.Query != "" {
		t.Fatalf("expected DELETE /endpoints/7 without query, got %+v", req)
	}
	if mock.FindRequest("GET", "/endpoints/7") != nil {
		t.Error("expected no polling without delete_cluster_on_destroy")
	}
}

// 202: KubeSolo is uninstalled in the background; the provider waits for 404.
func TestEnvironmentDelete_DeleteClusterWaitsForRemoval(t *testing.T) {
	shortenEnvironmentRemovePoll(t)
	mock := NewMockServer(t)
	mock.On("DELETE", "/endpoints/7", RespondJSON(http.StatusAccepted, nil))
	var gets int32
	mock.On("GET", "/endpoints/7", func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&gets, 1) < 3 {
			RespondJSON(http.StatusOK, map[string]interface{}{"Id": 7, "Status": 3})(w, r)
			return
		}
		RespondString(http.StatusNotFound, "text/plain", "not found")(w, r)
	})

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("7")
	_ = d.Set("delete_cluster_on_destroy", true)
	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	req := mock.FindRequest("DELETE", "/endpoints/7")
	if req == nil || req.Query != "deleteCluster=true" {
		t.Fatalf("expected deleteCluster=true, got %+v", req)
	}
	if n := atomic.LoadInt32(&gets); n != 3 {
		t.Errorf("expected 3 GET polls, got %d", n)
	}
}

// 409: KubeSolo is still being installed.
func TestEnvironmentDelete_DeleteClusterConflict(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/endpoints/7", RespondString(http.StatusConflict, "application/json", `{"message":"installing"}`))

	r := resourceEnvironment()
	d := r.TestResourceData()
	d.SetId("7")
	_ = d.Set("delete_cluster_on_destroy", true)
	err := rcDelete(r, d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "still being installed") {
		t.Fatalf("expected install-in-progress error, got %v", err)
	}
}

func TestWaitEnvironmentRemoved_Timeout(t *testing.T) {
	shortenEnvironmentRemovePoll(t)
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/7", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 7}))

	err := waitEnvironmentRemoved(context.Background(), mock.Client(), 7, 20*time.Millisecond)
	if err == nil || !strings.Contains(err.Error(), "still exists") {
		t.Fatalf("expected timeout error, got %v", err)
	}
}

// Changing only delete_cluster_on_destroy must not PUT the environment.
func TestEnvironmentUpdate_DeleteClusterOnlyIsStateOnly(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 7, "Name": "prod", "Type": 1, "URL": "tcp://docker.example:2376", "GroupId": 1,
	}))

	d := environmentTLSUpdate(t, map[string]interface{}{"delete_cluster_on_destroy": true})
	if err := rcUpdate(resourceEnvironment(), d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("PUT", "/endpoints/7") != nil {
		t.Error("expected no PUT when only delete_cluster_on_destroy changes")
	}
	if got := d.Get("delete_cluster_on_destroy"); got != true {
		t.Errorf("delete_cluster_on_destroy: expected true, got %v", got)
	}
}

// Together with a real change the environment is still updated.
func TestEnvironmentUpdate_DeleteClusterWithOtherChangeUpdates(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/endpoints/7", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 7}))
	mock.On("GET", "/endpoints/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 7, "Name": "renamed", "Type": 1, "URL": "tcp://docker.example:2376", "GroupId": 1,
	}))

	d := environmentTLSUpdate(t, map[string]interface{}{"delete_cluster_on_destroy": true, "name": "renamed"})
	if err := rcUpdate(resourceEnvironment(), d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("PUT", "/endpoints/7") == nil {
		t.Error("expected PUT /endpoints/7 for the name change")
	}
}

// --- kubernetes namespaces: ResourceQuotas / EffectiveCeiling (3.0) ---

func TestKubernetesNamespaceRead_QuotasAndCeiling(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/3/namespaces/team-a", RespondJSON(http.StatusOK, map[string]interface{}{
		"Name": "team-a",
		"ResourceQuotas": []map[string]interface{}{
			{
				"metadata": map[string]interface{}{"name": "portainer-rq-team-a"},
				"spec":     map[string]interface{}{"hard": map[string]string{"limits.cpu": "2", "limits.memory": "1Gi"}},
				"status": map[string]interface{}{
					"hard": map[string]string{"limits.cpu": "2", "limits.memory": "1Gi"},
					"used": map[string]string{"limits.cpu": "500m", "limits.memory": "256Mi"},
				},
			},
			{
				"metadata": map[string]interface{}{"name": "extra"},
				"status":   map[string]interface{}{"hard": map[string]string{"limits.cpu": "1"}},
			},
		},
		"EffectiveCeiling": map[string]interface{}{
			"limits.memory": map[string]string{"BoundBy": "portainer-rq-team-a", "Hard": "1Gi", "Used": "256Mi"},
			"limits.cpu":    map[string]string{"BoundBy": "extra", "Hard": "1", "Used": "500m"},
		},
	}))

	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	d.SetId("3:team-a")
	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if req := mock.FindRequest("GET", "/kubernetes/3/namespaces/team-a"); req == nil || req.Query != "withResourceQuota=true" {
		t.Fatalf("expected withResourceQuota=true, got %+v", req)
	}

	if n := d.Get("resource_quotas.#"); n != 2 {
		t.Fatalf("resource_quotas: expected 2, got %v", n)
	}
	checks := map[string]string{
		"resource_quotas.0.name":       "portainer-rq-team-a",
		"resource_quotas.1.name":       "extra",
		"effective_ceiling.0.resource": "limits.cpu",
		"effective_ceiling.0.hard":     "1",
		"effective_ceiling.0.used":     "500m",
		"effective_ceiling.0.bound_by": "extra",
		"effective_ceiling.1.resource": "limits.memory",
		"effective_ceiling.1.bound_by": "portainer-rq-team-a",
	}
	for k, want := range checks {
		if got := d.Get(k); got != want {
			t.Errorf("%s: expected %q, got %v", k, want, got)
		}
	}
	// Map keys contain dots, so read the maps whole.
	maps := []struct {
		path, key, want string
	}{
		{"resource_quotas.0.hard", "limits.memory", "1Gi"},
		{"resource_quotas.0.used", "limits.cpu", "500m"},
		{"resource_quotas.1.hard", "limits.cpu", "1"}, // status.hard fallback
	}
	for _, m := range maps {
		if got := d.Get(m.path).(map[string]interface{})[m.key]; got != m.want {
			t.Errorf("%s[%s]: expected %q, got %v", m.path, m.key, m.want, got)
		}
	}
}

// Before 3.0 the fields are absent and both attributes are empty.
func TestKubernetesNamespaceRead_QuotasPre30(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/3/namespaces/team-a", RespondJSON(http.StatusOK, map[string]interface{}{
		"Name": "team-a",
		"ResourceQuota": map[string]interface{}{
			"spec": map[string]interface{}{"hard": map[string]string{"limits.cpu": "2"}},
		},
	}))

	r := resourceKubernetesNamespace()
	d := r.TestResourceData()
	d.SetId("3:team-a")
	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if n := d.Get("resource_quotas.#"); n != 0 {
		t.Errorf("resource_quotas: expected empty, got %v", n)
	}
	if n := d.Get("effective_ceiling.#"); n != 0 {
		t.Errorf("effective_ceiling: expected empty, got %v", n)
	}
}

// TestKubernetesNamespace_QuotaQuantitiesSuppressed: Kubernetes returns
// quantities in canonical form ("1000m" as "1"), which is not a change
// (seen live on 3.0.1, where it planned an update on every run).
func TestKubernetesNamespace_QuotaQuantitiesSuppressed(t *testing.T) {
	suppress := resourceKubernetesNamespace().Schema["resource_quota"].DiffSuppressFunc
	for _, tc := range []struct {
		old, new string
		want     bool
	}{
		{"1", "1000m", true},
		{"1Gi", "1024Mi", true},
		{"500m", "1", false},
		{"", "1", false},
	} {
		if got := suppress("resource_quota.cpu_limit", tc.old, tc.new, nil); got != tc.want {
			t.Errorf("%q -> %q: suppressed = %v, want %v", tc.old, tc.new, got, tc.want)
		}
	}
}
