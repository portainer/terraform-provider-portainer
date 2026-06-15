package internal

import (
	"net/http"
	"reflect"
	"testing"
)

// TestEndpointGroupCreate_HappyPath exercises the create path which uses the
// generated SDK (client.Client.EndpointGroups.*). The Create flow first lists
// existing groups (to check for a name collision), then POSTs the new group,
// then re-reads it.
func TestEndpointGroupCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	// findExistingEndpointGroupByName: list returns no match.
	mock.On("GET", "/endpoint_groups", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "other"},
	}))
	mock.On("POST", "/endpoint_groups", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":          5,
		"Name":        "Production",
		"Description": "prod group",
		"TagIds":      []int{10, 20},
	}))
	mock.On("GET", "/endpoint_groups/5", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":          5,
		"Name":        "Production",
		"Description": "prod group",
		"TagIds":      []int{10, 20},
	}))

	r := resourceEndpointGroup()
	d := r.TestResourceData()
	_ = d.Set("name", "Production")
	_ = d.Set("description", "prod group")
	_ = d.Set("tag_ids", []interface{}{10, 20})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "5" {
		t.Errorf("expected ID %q, got %q", "5", d.Id())
	}
	if got := d.Get("name"); got != "Production" {
		t.Errorf("name: expected %q, got %v", "Production", got)
	}
	if got := d.Get("description"); got != "prod group" {
		t.Errorf("description: expected %q, got %v", "prod group", got)
	}
	got := d.Get("tag_ids").([]interface{})
	want := []interface{}{10, 20}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tag_ids: expected %v, got %v", want, got)
	}

	// Verify the POST payload contains the expected camelCase field names.
	post := mock.FindRequest("POST", "/endpoint_groups")
	if post == nil {
		t.Fatal("expected a POST to /endpoint_groups")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode POST body: %v", err)
	}
	if payload["name"] != "Production" {
		t.Errorf("payload.name: expected %q, got %v", "Production", payload["name"])
	}
	if payload["description"] != "prod group" {
		t.Errorf("payload.description: expected %q, got %v", "prod group", payload["description"])
	}
}

// TestEndpointGroupCreate_ExistingNameTriggersUpdate verifies that when a
// group with the same name already exists, Create falls back to Update with
// the existing ID instead of POSTing a duplicate.
func TestEndpointGroupCreate_ExistingNameTriggersUpdate(t *testing.T) {
	mock := NewMockServer(t)

	// Group already exists with id 7.
	mock.On("GET", "/endpoint_groups", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 7, "Name": "Production"},
	}))
	mock.On("PUT", "/endpoint_groups/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 7, "Name": "Production",
	}))
	mock.On("GET", "/endpoint_groups/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   7,
		"Name": "Production",
	}))

	r := resourceEndpointGroup()
	d := r.TestResourceData()
	_ = d.Set("name", "Production")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "7" {
		t.Errorf("expected ID %q (existing), got %q", "7", d.Id())
	}
	if mock.FindRequest("PUT", "/endpoint_groups/7") == nil {
		t.Error("expected a PUT to /endpoint_groups/7 (update path)")
	}
	// No POST should have been sent.
	if mock.FindRequest("POST", "/endpoint_groups") != nil {
		t.Error("did not expect a POST when group with same name already exists")
	}
}

// TestEndpointGroupRead_HappyPath verifies state is populated from GET.
func TestEndpointGroupRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":          42,
		"Name":        "staging",
		"Description": "staging desc",
		"TagIds":      []int{1, 2, 3},
	}))

	r := resourceEndpointGroup()
	d := r.TestResourceData()
	d.SetId("42")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("name"); got != "staging" {
		t.Errorf("name: expected %q, got %v", "staging", got)
	}
	if got := d.Get("description"); got != "staging desc" {
		t.Errorf("description: expected %q, got %v", "staging desc", got)
	}
	got := d.Get("tag_ids").([]interface{})
	want := []interface{}{1, 2, 3}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("tag_ids: expected %v, got %v", want, got)
	}
}

// TestEndpointGroupRead_404_ClearsID verifies that a 404 from the SDK clears
// the ID (drift detection).
func TestEndpointGroupRead_404_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoint_groups/99", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"not found"}`,
	))

	r := resourceEndpointGroup()
	d := r.TestResourceData()
	d.SetId("99")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404 and clear ID, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after 404, got %q", d.Id())
	}
}

// TestEndpointGroupUpdate_HappyPath verifies PUT is sent with the new payload.
func TestEndpointGroupUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/endpoint_groups/3", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":          3,
		"Name":        "renamed",
		"Description": "new desc",
	}))
	mock.On("GET", "/endpoint_groups/3", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":          3,
		"Name":        "renamed",
		"Description": "new desc",
	}))

	r := resourceEndpointGroup()
	d := r.TestResourceData()
	d.SetId("3")
	_ = d.Set("name", "renamed")
	_ = d.Set("description", "new desc")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/endpoint_groups/3")
	if put == nil {
		t.Fatal("expected PUT /endpoint_groups/3")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	if payload["name"] != "renamed" {
		t.Errorf("payload.name: expected %q, got %v", "renamed", payload["name"])
	}
	if payload["description"] != "new desc" {
		t.Errorf("payload.description: expected %q, got %v", "new desc", payload["description"])
	}
}

// TestEndpointGroupDelete_HappyPath verifies the SDK DELETE is sent.
func TestEndpointGroupDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoint_groups/8", RespondString(http.StatusNoContent, "", ""))

	r := resourceEndpointGroup()
	d := r.TestResourceData()
	d.SetId("8")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/endpoint_groups/8") == nil {
		t.Error("expected DELETE /endpoint_groups/8 to be sent")
	}
}

// TestEndpointGroupDelete_404Swallowed verifies that a 404 on Delete is not
// surfaced as an error (resource is already gone).
func TestEndpointGroupDelete_404Swallowed(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoint_groups/123", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"not found"}`,
	))

	r := resourceEndpointGroup()
	d := r.TestResourceData()
	d.SetId("123")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow 404, got error: %v", err)
	}
}
