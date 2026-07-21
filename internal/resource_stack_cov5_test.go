package internal

import (
	"net/http"
	"testing"
)

// TestStackCov5_UpdateAccessControl covers updateStackAccessControl for each
// ownership arm (public / administrators / restricted). The helper looks up the
// stack's resource-control ID (GET /stacks/{id}) and PUTs the new policy to
// /resource_controls/{rcID}.
func TestStackCov5_UpdateAccessControl(t *testing.T) {
	cases := []struct {
		name      string
		ownership string
		withLists bool
	}{
		{"public", "public", false},
		{"administrators", "administrators", false},
		{"restricted", "restricted", true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("GET", "/stacks/42", RespondJSON(http.StatusOK, map[string]interface{}{
				"Id":              42,
				"ResourceControl": map[string]interface{}{"Id": 55},
			}))
			mock.On("PUT", "/resource_controls/55", RespondJSON(http.StatusOK, map[string]interface{}{}))

			r := resourcePortainerStack()
			d := r.TestResourceData()
			d.SetId("42")
			_ = d.Set("ownership", tc.ownership)
			if tc.withLists {
				_ = d.Set("authorized_users", []interface{}{5, 7})
				_ = d.Set("authorized_teams", []interface{}{2})
			}

			if err := updateStackAccessControl(d, mock.Client(), "42"); err != nil {
				t.Fatalf("updateStackAccessControl(%s) failed: %v", tc.ownership, err)
			}
			if mock.FindRequest("PUT", "/resource_controls/55") == nil {
				t.Errorf("%s: expected PUT /resource_controls/55", tc.ownership)
			}
		})
	}
}

// TestStackCov5_UpdateAccessControl_NoOwnership verifies the early return when
// ownership is unset (no lookup, no PUT).
func TestStackCov5_UpdateAccessControl_NoOwnership(t *testing.T) {
	mock := NewMockServer(t)

	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("42")

	if err := updateStackAccessControl(d, mock.Client(), "42"); err != nil {
		t.Fatalf("expected nil for unset ownership, got: %v", err)
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected no HTTP calls when ownership is unset, got %d", len(mock.Requests()))
	}
}

// TestStackCov5_UpdateAccessControl_LookupError surfaces a lookup failure.
func TestStackCov5_UpdateAccessControl_LookupError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/stacks/42", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourcePortainerStack()
	d := r.TestResourceData()
	d.SetId("42")
	_ = d.Set("ownership", "public")

	if err := updateStackAccessControl(d, mock.Client(), "42"); err == nil {
		t.Fatal("expected error when resource-control lookup fails, got nil")
	}
}
