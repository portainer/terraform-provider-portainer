package internal

import (
	"net/http"
	"testing"
)

// TestTeamCreate_HappyPath covers the create path where the team does not yet
// exist on the server. The resource lists teams first, then POSTs a new one,
// then re-reads it via TeamInspect.
func TestTeamCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	// Initial list — team not present yet.
	mock.On("GET", "/teams", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	// Create returns PortainerTeam with capitalized JSON fields.
	mock.On("POST", "/teams", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   42,
		"Name": "devops",
	}))

	// Re-read via inspect.
	mock.On("GET", "/teams/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   42,
		"Name": "devops",
	}))

	r := resourceTeam()
	d := r.TestResourceData()
	_ = d.Set("name", "devops")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "42" {
		t.Errorf("expected ID %q, got %q", "42", d.Id())
	}
	if got := d.Get("name"); got != "devops" {
		t.Errorf("name: expected %q, got %v", "devops", got)
	}

	// Verify Create payload uses correct field name.
	post := mock.FindRequest("POST", "/teams")
	if post == nil {
		t.Fatal("expected a POST to /teams")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode POST body: %v", err)
	}
	if got := payload["name"]; got != "devops" {
		t.Errorf("payload.name: expected %q, got %v", "devops", got)
	}
}

// TestTeamCreate_AlreadyExists verifies that when a team with the same name
// is found in the list, the resource skips POST and falls through to Update.
func TestTeamCreate_AlreadyExists(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/teams", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 7, "Name": "devops"},
	}))

	// Update goes to /teams/7 (PUT) and then Read /teams/7.
	mock.On("PUT", "/teams/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   7,
		"Name": "devops",
	}))
	mock.On("GET", "/teams/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   7,
		"Name": "devops",
	}))

	r := resourceTeam()
	d := r.TestResourceData()
	_ = d.Set("name", "devops")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "7" {
		t.Errorf("expected ID %q from existing team, got %q", "7", d.Id())
	}
	// No POST should have been sent.
	if mock.FindRequest("POST", "/teams") != nil {
		t.Error("expected NO POST when team already exists")
	}
}

// TestTeamRead_HappyPath verifies state population.
func TestTeamRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/teams/9", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   9,
		"Name": "platform",
	}))

	r := resourceTeam()
	d := r.TestResourceData()
	d.SetId("9")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("name"); got != "platform" {
		t.Errorf("name: expected %q, got %v", "platform", got)
	}
}

// TestTeamUpdate_HappyPath verifies a PUT is sent with the new name.
func TestTeamUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/teams/3", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   3,
		"Name": "renamed",
	}))
	mock.On("GET", "/teams/3", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   3,
		"Name": "renamed",
	}))

	r := resourceTeam()
	d := r.TestResourceData()
	d.SetId("3")
	_ = d.Set("name", "renamed")

	if err := r.Update(d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	put := mock.FindRequest("PUT", "/teams/3")
	if put == nil {
		t.Fatal("expected PUT /teams/3")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode PUT body: %v", err)
	}
	if got := payload["name"]; got != "renamed" {
		t.Errorf("payload.name: expected %q, got %v", "renamed", got)
	}
}

// TestTeamDelete_HappyPath verifies the DELETE call is sent.
func TestTeamDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/teams/5", RespondString(http.StatusNoContent, "", ""))

	r := resourceTeam()
	d := r.TestResourceData()
	d.SetId("5")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/teams/5") == nil {
		t.Error("expected DELETE /teams/5 to be sent")
	}
}

// TestTeamRead_404_ClearsID verifies that TeamInspect 404 clears the ID
// (Terraform drift detection).
func TestTeamRead_404_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/teams/404", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"team not found"}`,
	))

	r := resourceTeam()
	d := r.TestResourceData()
	d.SetId("404")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404 and clear ID, got: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after 404, got %q", d.Id())
	}
}
