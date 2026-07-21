package internal

import (
	"net/http"
	"sync/atomic"
	"testing"
)

// =========================================================================
// cov4 coverage for resource_endpoint_group_access.go: the nil-policy-map
// initialization branch in getEndpointGroupPolicies (response omits the policy
// maps entirely), the getEndpointGroupMap failure branch reached from Create
// (second GET fails), and the Read-chain failure in updateEndpointGroup (the
// post-PUT re-read fails).
//
// The last two use a single stateful handler on GET /endpoint_groups/{id} that
// serves success for the first N calls and an error afterwards, letting an
// earlier GET succeed while a later GET on the same path fails.
// =========================================================================

// TestEndpointGroupAccessCov4_Read_NilPolicyMaps covers the nil-map init in
// getEndpointGroupPolicies: when the group JSON omits UserAccessPolicies and
// TeamAccessPolicies, the decoder leaves them nil and the function allocates
// empty maps. With no matching policy the Read then clears the ID.
func TestEndpointGroupAccessCov4_Read_NilPolicyMaps(t *testing.T) {
	mock := NewMockServer(t)

	// No policy maps in the payload at all.
	mock.On("GET", "/endpoint_groups/4", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   4,
		"Name": "g4",
	}))

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	d.SetId("4/team/11")
	_ = d.Set("endpoint_group_id", 4)
	_ = d.Set("team_id", 11)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared when policy maps are absent, got %q", d.Id())
	}
}

// TestEndpointGroupAccessCov4_Create_MapFetchError covers the getEndpointGroupMap
// error branch reached from Create: getEndpointGroupPolicies (first GET) succeeds
// but getEndpointGroupMap (second GET on the same path) fails.
func TestEndpointGroupAccessCov4_Create_MapFetchError(t *testing.T) {
	mock := NewMockServer(t)

	var calls int32
	mock.On("GET", "/endpoint_groups/4", func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Id":4,"Name":"g4","UserAccessPolicies":{},"TeamAccessPolicies":{}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"map fetch boom"}`))
	})

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	_ = d.Set("endpoint_group_id", 4)
	_ = d.Set("team_id", 11)
	_ = d.Set("role_id", 3)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when the second (map) group fetch fails, got nil")
	}
	if mock.FindRequest("PUT", "/endpoint_groups/4") != nil {
		t.Error("did not expect a PUT after the map fetch failed")
	}
}

// TestEndpointGroupAccessCov4_Create_ReadChainError covers the Read-chain error
// branch in updateEndpointGroup: both group GETs succeed, the PUT succeeds, but
// the post-PUT re-read (a third GET) fails, so updateEndpointGroup surfaces the
// error.
func TestEndpointGroupAccessCov4_Create_ReadChainError(t *testing.T) {
	mock := NewMockServer(t)

	var calls int32
	mock.On("GET", "/endpoint_groups/4", func(w http.ResponseWriter, _ *http.Request) {
		if atomic.AddInt32(&calls, 1) <= 2 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"Id":4,"Name":"g4","UserAccessPolicies":{},"TeamAccessPolicies":{}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"read boom"}`))
	})
	mock.On("PUT", "/endpoint_groups/4", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 4}))

	r := resourceEndpointGroupAccess()
	d := r.TestResourceData()
	_ = d.Set("endpoint_group_id", 4)
	_ = d.Set("team_id", 11)
	_ = d.Set("role_id", 3)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when the post-PUT re-read fails, got nil")
	}
	if mock.FindRequest("PUT", "/endpoint_groups/4") == nil {
		t.Error("expected the PUT to have been sent before the failing re-read")
	}
}
