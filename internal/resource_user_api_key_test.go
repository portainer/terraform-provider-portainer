package internal

import (
	"net/http"
	"testing"
)

// TestUserAPIKeyCreate_HappyPath verifies the key is created against the user's
// token endpoint and that the raw key — returned exactly once — is stored.
func TestUserAPIKeyCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/users/3/tokens", RespondJSON(http.StatusOK, map[string]interface{}{
		"rawAPIKey": "ptr_secretvalue",
		"apiKey": map[string]interface{}{
			"id": 11, "userId": 3, "description": "terraform",
			"prefix": "ptr_abc", "dateCreated": 1756900000, "lastUsed": 0,
		},
	}))

	r := resourceUserAPIKey()
	d := r.TestResourceData()
	_ = d.Set("user_id", 3)
	_ = d.Set("description", "terraform")
	_ = d.Set("password", "hunter2")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "11" {
		t.Errorf("id: expected %q, got %q", "11", d.Id())
	}
	if got := d.Get("raw_api_key"); got != "ptr_secretvalue" {
		t.Errorf("raw_api_key: got %v", got)
	}
	if got := d.Get("prefix"); got != "ptr_abc" {
		t.Errorf("prefix: got %v", got)
	}

	post := mock.FindRequest("POST", "/users/3/tokens")
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode failed: %v", err)
	}
	if payload["description"] != "terraform" || payload["password"] != "hunter2" {
		t.Errorf("payload mismatch: %v", payload)
	}
}

// TestUserAPIKeyCreate_NoIDReturned verifies a response without a key ID fails
// instead of leaving a resource that can never be revoked.
func TestUserAPIKeyCreate_NoIDReturned(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/users/3/tokens", RespondJSON(http.StatusOK, map[string]interface{}{
		"rawAPIKey": "ptr_secretvalue",
	}))

	r := resourceUserAPIKey()
	d := r.TestResourceData()
	_ = d.Set("user_id", 3)
	_ = d.Set("description", "terraform")
	_ = d.Set("password", "hunter2")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected Create to fail when Portainer returns no key ID")
	}
}

// TestUserAPIKeyRead_MatchesFromList verifies the read finds the key in the
// user's key list, since Portainer has no single-key endpoint.
func TestUserAPIKeyRead_MatchesFromList(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/users/3/tokens", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": 10, "description": "other", "prefix": "ptr_zzz"},
		{"id": 11, "description": "terraform", "prefix": "ptr_abc", "lastUsed": 1756999999},
	}))

	r := resourceUserAPIKey()
	d := r.TestResourceData()
	d.SetId("11")
	_ = d.Set("user_id", 3)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("description"); got != "terraform" {
		t.Errorf("description: got %v", got)
	}
	if got := d.Get("last_used"); got != 1756999999 {
		t.Errorf("last_used: got %v", got)
	}
}

// TestUserAPIKeyRead_RevokedElsewhereClearsID verifies a key revoked outside
// Terraform drops out of state rather than failing every plan.
func TestUserAPIKeyRead_RevokedElsewhereClearsID(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/users/3/tokens", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": 10, "description": "other"},
	}))

	r := resourceUserAPIKey()
	d := r.TestResourceData()
	d.SetId("11")
	_ = d.Set("user_id", 3)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected the ID to be cleared when the key is gone, got %q", d.Id())
	}
}

// TestUserAPIKeyDelete_Revokes verifies the revoke call, which is the gap this
// resource exists to close: portainer_user could create a key but never remove
// one.
func TestUserAPIKeyDelete_Revokes(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/users/3/tokens/11", RespondString(http.StatusNoContent, "", ""))

	r := resourceUserAPIKey()
	d := r.TestResourceData()
	d.SetId("11")
	_ = d.Set("user_id", 3)

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/users/3/tokens/11") == nil {
		t.Error("expected DELETE /users/3/tokens/11")
	}
	if d.Id() != "" {
		t.Errorf("expected the ID to be cleared, got %q", d.Id())
	}
}

func TestUserAPIKeyDelete_404IsSuccess(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/users/3/tokens/11",
		RespondString(http.StatusNotFound, "application/json", `{"message":"key not found"}`))

	r := resourceUserAPIKey()
	d := r.TestResourceData()
	d.SetId("11")
	_ = d.Set("user_id", 3)

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should treat a missing key as success: %v", err)
	}
}
