package internal

import (
	"testing"
)

// --- users: CreatedAuthenticationMethod / LastAuthenticationMethod (3.0) ---

// The resource tests of these attributes are in fw_resource_user_test.go.

func TestAuthenticationMethodName(t *testing.T) {
	t.Parallel()
	for in, want := range map[int]string{0: "", 1: "internal", 2: "ldap", 3: "oauth", 9: "9"} {
		if got := authenticationMethodName(in); got != want {
			t.Errorf("authenticationMethodName(%d) = %q, want %q", in, got, want)
		}
	}
}
