package internal

import (
	"net/http"
	"testing"
)

// TestEdgeGroupCreate_HappyPath exercises the standard create flow when no
// existing group with the same name is present. Create POSTs to /edge_groups,
// receives an ID, then chains into Read.
func TestEdgeGroupCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	// findExistingEdgeGroupByName lists all groups first.
	mock.On("GET", "/edge_groups", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "other"},
	}))

	mock.On("POST", "/edge_groups", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 5,
	}))

	mock.On("GET", "/edge_groups/5", RespondJSON(http.StatusOK, map[string]interface{}{
		"Name":         "production",
		"Dynamic":      true,
		"PartialMatch": false,
		"TagIds":       []int{},
		"Endpoints":    []int{},
	}))

	r := resourceEdgeGroup()
	d := r.TestResourceData()
	_ = d.Set("name", "production")
	_ = d.Set("dynamic", true)
	_ = d.Set("partial_match", false)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "5" {
		t.Errorf("expected ID %q, got %q", "5", d.Id())
	}

	post := mock.FindRequest("POST", "/edge_groups")
	if post == nil {
		t.Fatal("expected POST to /edge_groups")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode POST body: %v", err)
	}
	if got := payload["name"]; got != "production" {
		t.Errorf("payload.name: expected %q, got %v", "production", got)
	}
	if got := payload["dynamic"]; got != true {
		t.Errorf("payload.dynamic: expected true, got %v", got)
	}
	if got := payload["partialMatch"]; got != false {
		t.Errorf("payload.partialMatch: expected false, got %v", got)
	}

	// Confirm Read was chained.
	if mock.FindRequest("GET", "/edge_groups/5") == nil {
		t.Error("expected Create to chain into Read at /edge_groups/5")
	}
}

// TestEdgeGroupCreate_ExistingNameTriggersUpdate verifies that when a group
// with the same name already exists, the resource adopts its ID and switches
// to Update (PUT) instead of POSTing a duplicate.
func TestEdgeGroupCreate_ExistingNameTriggersUpdate(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_groups", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 42, "Name": "production"},
	}))

	mock.On("PUT", "/edge_groups/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 42,
	}))

	mock.On("GET", "/edge_groups/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"Name":         "production",
		"Dynamic":      true,
		"PartialMatch": false,
	}))

	r := resourceEdgeGroup()
	d := r.TestResourceData()
	_ = d.Set("name", "production")
	_ = d.Set("dynamic", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "42" {
		t.Errorf("expected adopted ID %q, got %q", "42", d.Id())
	}

	if mock.FindRequest("PUT", "/edge_groups/42") == nil {
		t.Error("expected Update PUT /edge_groups/42 to be sent")
	}
	if mock.FindRequest("POST", "/edge_groups") != nil {
		t.Error("did not expect POST /edge_groups when group with same name exists")
	}
}

// TestEdgeGroupRead_HappyPath verifies state is populated from GET response.
func TestEdgeGroupRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_groups/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"Name":         "staging",
		"Dynamic":      false,
		"PartialMatch": true,
		"TagIds":       []int{1, 2},
		"Endpoints":    []int{10, 11},
	}))

	r := resourceEdgeGroup()
	d := r.TestResourceData()
	d.SetId("7")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("name"); got != "staging" {
		t.Errorf("name: expected %q, got %v", "staging", got)
	}
	if got := d.Get("dynamic"); got != false {
		t.Errorf("dynamic: expected false, got %v", got)
	}
	if got := d.Get("partial_match"); got != true {
		t.Errorf("partial_match: expected true, got %v", got)
	}
}

// TestEdgeGroupRead_404_ClearsID verifies drift detection clears the ID.
func TestEdgeGroupRead_404_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_groups/99", RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`))

	r := resourceEdgeGroup()
	d := r.TestResourceData()
	d.SetId("99")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after 404, got %q", d.Id())
	}
}

// TestEdgeGroupDelete_HappyPath verifies DELETE is sent.
func TestEdgeGroupDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/edge_groups/5", RespondString(http.StatusNoContent, "", ""))

	r := resourceEdgeGroup()
	d := r.TestResourceData()
	d.SetId("5")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if mock.FindRequest("DELETE", "/edge_groups/5") == nil {
		t.Error("expected DELETE /edge_groups/5 to be sent")
	}
}

// TestEdgeGroupCreate_HTTPError verifies POST 4xx propagates as an error.
func TestEdgeGroupCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	// No existing group with same name.
	mock.On("GET", "/edge_groups", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	mock.On("POST", "/edge_groups", RespondString(http.StatusBadRequest, "application/json", `{"message":"invalid"}`))

	r := resourceEdgeGroup()
	d := r.TestResourceData()
	_ = d.Set("name", "bad")
	_ = d.Set("dynamic", true)

	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
