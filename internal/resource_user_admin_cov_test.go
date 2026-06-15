package internal

import "testing"

// =========================================================================
// Additional coverage for resource_user_admin.go: Update and Delete are both
// documented no-ops that clear the ID (bootstrap-only resource).
// =========================================================================

// TestUserAdminUpdate_ClearsID verifies the no-op Update clears the ID.
func TestUserAdminUpdate_ClearsID(t *testing.T) {
	r := resourceUserAdmin()
	d := r.TestResourceData()
	d.SetId("1")

	if err := rcUpdate(r, d, nil); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared by Update, got %q", d.Id())
	}
}

// TestUserAdminDelete_ClearsID verifies the no-op Delete clears the ID.
func TestUserAdminDelete_ClearsID(t *testing.T) {
	r := resourceUserAdmin()
	d := r.TestResourceData()
	d.SetId("1")

	if err := rcDelete(r, d, nil); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared by Delete, got %q", d.Id())
	}
}
