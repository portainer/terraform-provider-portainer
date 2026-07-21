package internal

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// userDataWithPasswordChange builds a ResourceData with a real InstanceState +
// InstanceDiff (via the SDK's InternalMap) so d.HasChange("password") is true
// and both old/new values are non-empty — the only way to enter the password
// update branch of resourceUserUpdate, which TestResourceData alone cannot
// reach (it produces no diff).
func userDataWithPasswordChange(t *testing.T, id, oldPw, newPw string) *schema.ResourceData {
	t.Helper()
	r := resourceUser()
	state := &terraform.InstanceState{
		ID: id,
		Attributes: map[string]string{
			"id":       id,
			"username": "alice",
			"role":     "2",
			"password": oldPw,
		},
	}
	diff := &terraform.InstanceDiff{
		Attributes: map[string]*terraform.ResourceAttrDiff{
			"password": {Old: oldPw, New: newPw},
		},
	}
	d, err := schema.InternalMap(r.Schema).Data(state, diff)
	if err != nil {
		t.Fatalf("failed to build diffed ResourceData: %v", err)
	}
	return d
}

// TestUserCov4_Update_PasswordChange exercises the password-change branch of
// resourceUserUpdate: PUT /users/{id}/passwd, then the regular PUT /users/{id}
// update, then the chained Read.
func TestUserCov4_Update_PasswordChange(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/users/6/passwd", RespondString(http.StatusNoContent, "", ""))
	mock.On("PUT", "/users/6", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 6, "Username": "alice", "Role": 2,
	}))
	mock.On("GET", "/users/6", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 6, "Username": "alice", "Role": 2,
	}))

	r := resourceUser()
	d := userDataWithPasswordChange(t, "6", "oldpass", "newpass")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	if mock.FindRequest("PUT", "/users/6/passwd") == nil {
		t.Error("expected PUT /users/6/passwd for the password change")
	}
	if mock.FindRequest("PUT", "/users/6") == nil {
		t.Error("expected PUT /users/6 for the user update")
	}
}

// TestUserCov4_Update_PasswordChangeError verifies a failing password update is
// surfaced as an error and the regular user PUT is not attempted.
func TestUserCov4_Update_PasswordChangeError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/users/6/passwd", RespondString(
		http.StatusForbidden, "application/json", `{"message":"invalid current password"}`,
	))

	r := resourceUser()
	d := userDataWithPasswordChange(t, "6", "oldpass", "newpass")

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when password update fails, got nil")
	}
	if mock.FindRequest("PUT", "/users/6") != nil {
		t.Error("user update PUT should not run after a failed password change")
	}
}
