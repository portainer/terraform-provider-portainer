package internal

import "testing"

// =========================================================================
// Additional coverage for resource_user_admin.go: Update and Delete are both
// documented no-ops (bootstrap-only resource); Update keeps the ID so the plan
// settles, Delete only forgets the resource.
// =========================================================================

// TestUserAdminUpdate_KeepsID verifies the no-op Update keeps the resource.
func TestUserAdminUpdate_KeepsID(t *testing.T) {
	r := resourceUserAdmin()
	d := r.TestResourceData()
	d.SetId("1")

	if err := rcUpdate(r, d, nil); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if d.Id() != "1" {
		t.Errorf("expected ID kept by Update, got %q", d.Id())
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
