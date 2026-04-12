package internal

import (
	"testing"
)

// --------------- parseUserGitCredentialID ---------------

func TestParseUserGitCredentialID(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		wantUserID   int
		wantCredID   int
		wantErr      bool
		errSubstring string
	}{
		{
			name:       "valid format",
			input:      "1:2",
			wantUserID: 1,
			wantCredID: 2,
			wantErr:    false,
		},
		{
			name:       "large IDs",
			input:      "12345:67890",
			wantUserID: 12345,
			wantCredID: 67890,
			wantErr:    false,
		},
		{
			name:       "zero IDs",
			input:      "0:0",
			wantUserID: 0,
			wantCredID: 0,
			wantErr:    false,
		},
		{
			name:         "missing colon",
			input:        "12345",
			wantErr:      true,
			errSubstring: "unexpected format",
		},
		{
			name:         "empty string",
			input:        "",
			wantErr:      true,
			errSubstring: "unexpected format",
		},
		{
			name:         "non-numeric user ID",
			input:        "abc:123",
			wantErr:      true,
			errSubstring: "invalid user ID",
		},
		{
			name:         "non-numeric credential ID",
			input:        "123:abc",
			wantErr:      true,
			errSubstring: "invalid credential ID",
		},
		{
			name:         "both non-numeric",
			input:        "abc:def",
			wantErr:      true,
			errSubstring: "invalid user ID",
		},
		{
			name:         "extra colons",
			input:        "1:2:3",
			wantUserID:   1,
			wantCredID:   0,
			wantErr:      true,
			errSubstring: "invalid credential ID",
		},
		{
			name:         "only colon",
			input:        ":",
			wantErr:      true,
			errSubstring: "invalid user ID",
		},
		{
			name:       "negative user ID",
			input:      "-1:2",
			wantUserID: -1,
			wantCredID: 2,
			wantErr:    false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			userID, credID, err := parseUserGitCredentialID(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				if tt.errSubstring != "" {
					if !containsStr(err.Error(), tt.errSubstring) {
						t.Errorf("error %q does not contain %q", err.Error(), tt.errSubstring)
					}
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if userID != tt.wantUserID {
				t.Errorf("userID: expected %d, got %d", tt.wantUserID, userID)
			}
			if credID != tt.wantCredID {
				t.Errorf("credID: expected %d, got %d", tt.wantCredID, credID)
			}
		})
	}
}

// containsStr is a test-only helper to check substring presence.
func containsStr(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && containsSubstring(s, sub))
}

func containsSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}
