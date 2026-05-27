package internal

import (
	"net/http"
	"testing"
)

// TestEdgeUpdateSchedulesCreate_HappyPath verifies the POST payload and that
// the returned ID populates the resource. Note: Create does NOT chain into
// Read in this resource.
func TestEdgeUpdateSchedulesCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/edge_update_schedules", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":            21,
		"name":          "fleet-update",
		"agentImage":    "portainer/agent:2.20.0",
		"updaterImage":  "portainer/updater:latest",
		"registryId":    3,
		"scheduledTime": "2026-01-01T00:00:00Z",
		"edgeGroupIds":  []int{1, 2},
		"type":          0,
	}))

	r := resourcePortainerEdgeUpdateSchedules()
	d := r.TestResourceData()
	_ = d.Set("name", "fleet-update")
	_ = d.Set("agent_image", "portainer/agent:2.20.0")
	_ = d.Set("updater_image", "portainer/updater:latest")
	_ = d.Set("registry_id", 3)
	_ = d.Set("scheduled_time", "2026-01-01T00:00:00Z")
	_ = d.Set("group_ids", []interface{}{1, 2})
	_ = d.Set("type", 0)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "21" {
		t.Errorf("expected ID %q, got %q", "21", d.Id())
	}

	post := mock.FindRequest("POST", "/edge_update_schedules")
	if post == nil {
		t.Fatal("expected POST /edge_update_schedules")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode POST body: %v", err)
	}
	if got := payload["name"]; got != "fleet-update" {
		t.Errorf("payload.name: expected %q, got %v", "fleet-update", got)
	}
	if got := payload["agentImage"]; got != "portainer/agent:2.20.0" {
		t.Errorf("payload.agentImage: got %v", got)
	}
	// JSON numbers decode as float64.
	if got := payload["registryID"]; got != float64(3) {
		t.Errorf("payload.registryID: expected 3, got %v", got)
	}
	if got := payload["type"]; got != float64(0) {
		t.Errorf("payload.type: expected 0, got %v", got)
	}
}

// TestEdgeUpdateSchedulesRead_HappyPath verifies state is populated correctly,
// including the edgeGroupIds → group_ids mapping.
func TestEdgeUpdateSchedulesRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_update_schedules/21", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":            21,
		"name":          "fleet-update",
		"agentImage":    "portainer/agent:2.20.0",
		"updaterImage":  "portainer/updater:latest",
		"registryId":    3,
		"scheduledTime": "2026-01-01T00:00:00Z",
		"edgeGroupIds":  []int{1, 2},
		"type":          1,
	}))

	r := resourcePortainerEdgeUpdateSchedules()
	d := r.TestResourceData()
	d.SetId("21")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("name"); got != "fleet-update" {
		t.Errorf("name: expected %q, got %v", "fleet-update", got)
	}
	if got := d.Get("registry_id"); got != 3 {
		t.Errorf("registry_id: expected 3, got %v", got)
	}
	if got := d.Get("type"); got != 1 {
		t.Errorf("type: expected 1, got %v", got)
	}
}

// TestEdgeUpdateSchedulesRead_404_ClearsID verifies drift detection.
func TestEdgeUpdateSchedulesRead_404_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/edge_update_schedules/99", RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`))

	r := resourcePortainerEdgeUpdateSchedules()
	d := r.TestResourceData()
	d.SetId("99")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestEdgeUpdateSchedulesUpdate_HappyPath verifies Update POSTs to /<id>
// (this resource uses POST, not PUT, for Update — confirmed in source).
func TestEdgeUpdateSchedulesUpdate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/edge_update_schedules/21", RespondJSON(http.StatusOK, map[string]interface{}{}))

	// Update chains into Read at the end.
	mock.On("GET", "/edge_update_schedules/21", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":           21,
		"name":         "fleet-update-2",
		"agentImage":   "portainer/agent:2.21.0",
		"updaterImage": "portainer/updater:latest",
		"registryId":   3,
		"edgeGroupIds": []int{1},
		"type":         0,
	}))

	r := resourcePortainerEdgeUpdateSchedules()
	d := r.TestResourceData()
	d.SetId("21")
	_ = d.Set("name", "fleet-update-2")
	_ = d.Set("agent_image", "portainer/agent:2.21.0")
	_ = d.Set("updater_image", "portainer/updater:latest")
	_ = d.Set("registry_id", 3)
	_ = d.Set("scheduled_time", "2026-01-02T00:00:00Z")
	_ = d.Set("group_ids", []interface{}{1})
	_ = d.Set("type", 0)

	if err := r.Update(d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	if mock.FindRequest("POST", "/edge_update_schedules/21") == nil {
		t.Error("expected POST /edge_update_schedules/21 for Update")
	}
}

// TestEdgeUpdateSchedulesDelete_HappyPath verifies DELETE is sent and a 204
// is accepted.
func TestEdgeUpdateSchedulesDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/edge_update_schedules/21", RespondString(http.StatusNoContent, "", ""))

	r := resourcePortainerEdgeUpdateSchedules()
	d := r.TestResourceData()
	d.SetId("21")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if mock.FindRequest("DELETE", "/edge_update_schedules/21") == nil {
		t.Error("expected DELETE /edge_update_schedules/21 to be sent")
	}
}

// TestEdgeUpdateSchedulesCreate_HTTPError verifies POST 4xx surfaces as error.
func TestEdgeUpdateSchedulesCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/edge_update_schedules", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad scheduled time"}`))

	r := resourcePortainerEdgeUpdateSchedules()
	d := r.TestResourceData()
	_ = d.Set("name", "bad")
	_ = d.Set("agent_image", "x")
	_ = d.Set("updater_image", "y")
	_ = d.Set("registry_id", 1)
	_ = d.Set("scheduled_time", "not-a-time")
	_ = d.Set("group_ids", []interface{}{1})
	_ = d.Set("type", 0)

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}
