package internal

import (
	"net/http"
	"testing"
)

// TestEndpointGroupMembershipCreate_HappyPath verifies the PUT that places an
// environment in a group, and that the follow-up read confirms it.
func TestEndpointGroupMembershipCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/endpoint_groups/5/endpoints/9", RespondString(http.StatusNoContent, "", ""))
	mock.On("GET", "/endpoints/9", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 9, "Name": "prod", "GroupId": 5,
	}))

	r := resourceEndpointGroupMembership()
	d := r.TestResourceData()
	_ = d.Set("endpoint_group_id", 5)
	_ = d.Set("endpoint_id", 9)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "5/9" {
		t.Errorf("id: expected %q, got %q", "5/9", d.Id())
	}
	if mock.FindRequest("PUT", "/endpoint_groups/5/endpoints/9") == nil {
		t.Error("expected PUT /endpoint_groups/5/endpoints/9")
	}
}

// TestEndpointGroupMembershipRead_MovedAwayClearsID verifies that an
// environment moved to another group outside Terraform drops the membership
// from state. Group membership lives on the environment, so another group
// claiming it silently ends this one.
func TestEndpointGroupMembershipRead_MovedAwayClearsID(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/9", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 9, "Name": "prod", "GroupId": 7,
	}))

	r := resourceEndpointGroupMembership()
	d := r.TestResourceData()
	d.SetId("5/9")
	_ = d.Set("endpoint_group_id", 5)
	_ = d.Set("endpoint_id", 9)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected the ID to be cleared when the environment moved to another group, got %q", d.Id())
	}
}

// TestEndpointGroupMembershipRead_EnvironmentGoneClearsID covers the
// environment itself being deleted.
func TestEndpointGroupMembershipRead_EnvironmentGoneClearsID(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/9",
		RespondString(http.StatusNotFound, "application/json", `{"message":"environment not found"}`))

	r := resourceEndpointGroupMembership()
	d := r.TestResourceData()
	d.SetId("5/9")
	_ = d.Set("endpoint_group_id", 5)
	_ = d.Set("endpoint_id", 9)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should not fail on 404: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected the ID to be cleared, got %q", d.Id())
	}
}

// TestEndpointGroupMembershipDelete_HappyPath verifies the removal call.
func TestEndpointGroupMembershipDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/endpoint_groups/5/endpoints/9", RespondString(http.StatusNoContent, "", ""))

	r := resourceEndpointGroupMembership()
	d := r.TestResourceData()
	d.SetId("5/9")
	_ = d.Set("endpoint_group_id", 5)
	_ = d.Set("endpoint_id", 9)

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoint_groups/5/endpoints/9") == nil {
		t.Error("expected DELETE /endpoint_groups/5/endpoints/9")
	}
}
