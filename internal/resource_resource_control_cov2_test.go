package internal

import (
	"net/http"
	"testing"
)

// =========================================================================
// Additional coverage (cov2) for resource_resource_control.go: the
// lookup-by-stack Read (populating administrators_only/public/teams/users),
// the Read lookup-failure drift path, the Delete-by-lookup happy and
// not-found paths, the 403 clears-ID branch, the unsupported-type lookup
// error, and the Update/Delete lookup-error branches.
// =========================================================================

// TestResourceControlCov2_Read_LookupByStack populates state from a stack's
// ResourceControl payload (Public + team/user accesses).
func TestResourceControlCov2_Read_LookupByStack(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 42,
		"ResourceControl": map[string]interface{}{
			"Id":                 200,
			"AdministratorsOnly": false,
			"Public":             true,
			"TeamAccesses":       []map[string]interface{}{{"TeamId": 3}},
			"UserAccesses":       []map[string]interface{}{{"UserId": 5}},
		},
	}))

	r := resourceResourceControl()
	d := r.TestResourceData()
	_ = d.Set("resource_id", "42")
	_ = d.Set("type", 6)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "200" {
		t.Errorf("ID: expected 200, got %q", d.Id())
	}
	if got := d.Get("public"); got != true {
		t.Errorf("public: expected true, got %v", got)
	}
	teams := d.Get("teams").([]interface{})
	if len(teams) != 1 || teams[0].(int) != 3 {
		t.Errorf("teams: expected [3], got %v", teams)
	}
	users := d.Get("users").([]interface{})
	if len(users) != 1 || users[0].(int) != 5 {
		t.Errorf("users: expected [5], got %v", users)
	}
}

// TestResourceControlCov2_Read_LookupNotFoundClearsID covers the drift path:
// the stack lookup fails, so Read clears the ID and returns nil.
func TestResourceControlCov2_Read_LookupNotFoundClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks/42", RespondString(
		http.StatusNotFound, "application/json", `{"message":"gone"}`,
	))

	r := resourceResourceControl()
	d := r.TestResourceData()
	d.SetId("200")
	_ = d.Set("resource_id", "42")
	_ = d.Set("type", 6)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow lookup failure, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared on lookup failure, got %q", d.Id())
	}
}

// TestResourceControlCov2_Delete_LookupByStack covers Delete resolving the
// resource control via a stack lookup before issuing the DELETE.
func TestResourceControlCov2_Delete_LookupByStack(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":              42,
		"ResourceControl": map[string]interface{}{"Id": 201},
	}))
	mock.On("DELETE", "/resource_controls/201", RespondString(http.StatusOK, "application/json", `{}`))

	r := resourceResourceControl()
	d := r.TestResourceData()
	d.SetId("201")
	_ = d.Set("resource_id", "42")
	_ = d.Set("type", 6)

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/resource_controls/201") == nil {
		t.Error("expected DELETE /resource_controls/201 after stack lookup")
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestResourceControlCov2_Delete_LookupFailsClearsID covers the lookup-failure
// branch in Delete: the stack lookup fails, so Delete clears the ID and
// returns success.
func TestResourceControlCov2_Delete_LookupFailsClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks/42", RespondString(
		http.StatusNotFound, "application/json", `{"message":"gone"}`,
	))

	r := resourceResourceControl()
	d := r.TestResourceData()
	d.SetId("201")
	_ = d.Set("resource_id", "42")
	_ = d.Set("type", 6)

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow lookup failure, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestResourceControlCov2_Delete_403ClearsID covers the http.StatusForbidden
// branch of Delete (treated the same as 404).
func TestResourceControlCov2_Delete_403ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/resource_controls/300", RespondString(http.StatusForbidden, "application/json", `{}`))

	r := resourceResourceControl()
	d := r.TestResourceData()
	_ = d.Set("resource_control_id", 300)
	d.SetId("300")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should treat 403 as success, got: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after 403, got %q", d.Id())
	}
}

// TestResourceControlCov2_Delete_HTTPError covers the >= 400 (non-404/403)
// error branch of Delete.
func TestResourceControlCov2_Delete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/resource_controls/301", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceResourceControl()
	d := r.TestResourceData()
	_ = d.Set("resource_control_id", 301)
	d.SetId("301")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500 delete, got nil")
	}
}

// TestResourceControlCov2_Update_LookupError covers the lookup-failure branch
// of Update (reached via Create -> Update): the stack lookup fails and the
// error is surfaced.
func TestResourceControlCov2_Update_LookupError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks/77", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceResourceControl()
	d := r.TestResourceData()
	_ = d.Set("resource_id", "77")
	_ = d.Set("type", 6)
	_ = d.Set("public", true)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when stack lookup fails during Create/Update, got nil")
	}
}

// TestResourceControlCov2_LookupUnsupportedType covers the default branch of
// lookupResourceControlID via Update with an unsupported resource type.
func TestResourceControlCov2_LookupUnsupportedType(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceResourceControl()
	d := r.TestResourceData()
	_ = d.Set("resource_id", "1")
	_ = d.Set("type", 1) // container - unsupported by lookupResourceControlID
	_ = d.Set("public", true)

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error for unsupported resource type, got nil")
	}
}

// TestResourceControlCov2_Lookup_NoResourceControl covers the branch where the
// stack response has no ResourceControl (or a nil Id), so the lookup errors.
func TestResourceControlCov2_Lookup_NoResourceControl(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/stacks/88", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 88,
		// No ResourceControl key.
	}))

	r := resourceResourceControl()
	d := r.TestResourceData()
	_ = d.Set("resource_id", "88")
	_ = d.Set("type", 6)
	_ = d.Set("public", true)

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when stack has no resource control, got nil")
	}
}
