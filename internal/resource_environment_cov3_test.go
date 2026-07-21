package internal

import (
	"net/http"
	"strings"
	"testing"
)

// =========================================================================
// cov3 coverage for resource_environment.go: the tag_ids branch in Create for
// a non-edge environment (SetTagIds on the multipart form), the edge-agent
// tag-application follow-up Update, the team_access_policies Create->Update
// follow-up, and the findExistingEnvironmentByName list-error path.
// =========================================================================

// TestEnvironmentCov3_Create_NonEdgeWithTags covers the SetTagIds branch: a
// Docker (type=1) environment with tag_ids attaches the tag IDs to the
// multipart create form (Edge agents skip this and apply tags via Update).
func TestEnvironmentCov3_Create_NonEdgeWithTags(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/endpoints", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 61, "Name": "tagged", "Type": 1,
	}))
	mock.On("GET", "/endpoints/61", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 61, "Name": "tagged", "Type": 1, "GroupId": 1,
		"URL": "tcp://docker.example:2375", "TagIds": []int{8, 9},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	_ = d.Set("name", "tagged")
	_ = d.Set("environment_address", "tcp://docker.example:2375")
	_ = d.Set("type", 1)
	_ = d.Set("tag_ids", []interface{}{8, 9})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "61" {
		t.Errorf("expected ID 61, got %q", d.Id())
	}
	post := mock.FindRequest("POST", "/endpoints")
	if post == nil {
		t.Fatal("expected POST /endpoints")
	}
	// The multipart form should carry a TagIds field for a non-edge environment.
	if !strings.Contains(string(post.Body), "TagIds") {
		t.Errorf("expected multipart body to include TagIds form field, body=%q", string(post.Body))
	}
	// No follow-up Update should be issued for a non-edge tags-only create.
	if mock.FindRequest("PUT", "/endpoints/61") != nil {
		t.Error("did not expect a PUT follow-up for a non-edge create with tags")
	}
}

// TestEnvironmentCov3_Create_EdgeAgentWithTags covers the edge-agent tag
// follow-up: for type=4, tag_ids are NOT sent in the multipart create; instead
// Create delegates to Update (PUT) to attach them after creation.
func TestEnvironmentCov3_Create_EdgeAgentWithTags(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/endpoints", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 62, "Name": "edge-tagged", "Type": 4,
		"EdgeID": "eid", "EdgeKey": "ekey",
	}))
	mock.On("PUT", "/endpoints/62", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 62, "Name": "edge-tagged", "Type": 4,
	}))
	mock.On("GET", "/endpoints/62", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 62, "Name": "edge-tagged", "Type": 4, "GroupId": 1,
		"URL": "", "TagIds": []int{15},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	_ = d.Set("name", "edge-tagged")
	_ = d.Set("environment_address", "")
	_ = d.Set("type", 4)
	_ = d.Set("tag_ids", []interface{}{15})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "62" {
		t.Errorf("expected ID 62, got %q", d.Id())
	}
	// The tags-follow-up Update must have been issued.
	if mock.FindRequest("PUT", "/endpoints/62") == nil {
		t.Error("expected PUT /endpoints/62 (edge-agent tag follow-up Update)")
	}
}

// TestEnvironmentCov3_Create_WithTeamAccessPolicies covers the
// team_access_policies Create->Update follow-up branch (the base cov2 suite
// only exercises the user_access_policies follow-up).
func TestEnvironmentCov3_Create_WithTeamAccessPolicies(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/endpoints", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 63, "Name": "team-pol", "Type": 1,
	}))
	mock.On("PUT", "/endpoints/63", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 63, "Name": "team-pol", "Type": 1,
	}))
	mock.On("GET", "/endpoints/63", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 63, "Name": "team-pol", "Type": 1, "GroupId": 1,
		"URL": "tcp://docker.example:2375", "TagIds": []int{},
	}))

	r := resourceEnvironment()
	d := r.TestResourceData()
	_ = d.Set("name", "team-pol")
	_ = d.Set("environment_address", "tcp://docker.example:2375")
	_ = d.Set("type", 1)
	_ = d.Set("team_access_policies", map[string]interface{}{"4": 2})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "63" {
		t.Errorf("expected ID 63, got %q", d.Id())
	}
	put := mock.FindRequest("PUT", "/endpoints/63")
	if put == nil {
		t.Fatal("expected Create to delegate to Update (PUT /endpoints/63) for team access policies")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode PUT body: %v", err)
	}
	if _, ok := payload["teamAccessPolicies"]; !ok {
		t.Errorf("expected teamAccessPolicies in PUT body, got keys: %v", payload)
	}
}

// TestEnvironmentCov3_Create_ListError covers the findExistingEnvironmentByName
// failure branch: the initial GET /endpoints list returns 500, so Create errors
// before any POST.
func TestEnvironmentCov3_Create_ListError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"list boom"}`,
	))

	r := resourceEnvironment()
	d := r.TestResourceData()
	_ = d.Set("name", "x")
	_ = d.Set("environment_address", "tcp://docker.example:2375")
	_ = d.Set("type", 1)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when environment list returns 500, got nil")
	}
	if mock.FindRequest("POST", "/endpoints") != nil {
		t.Error("did not expect POST /endpoints after list failure")
	}
}
