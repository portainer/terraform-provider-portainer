package internal

import (
	"net/http"
	"testing"
)

// TestEdgeUpdSchedCov2_Update_HTTPError covers the >=400 branch of Update.
func TestEdgeUpdSchedCov2_Update_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/edge_update_schedules/21", RespondString(http.StatusBadRequest, "application/json", `{"message":"bad"}`))

	r := resourcePortainerEdgeUpdateSchedules()
	d := r.TestResourceData()
	d.SetId("21")
	_ = d.Set("name", "x")
	_ = d.Set("agent_image", "a")
	_ = d.Set("updater_image", "u")
	_ = d.Set("registry_id", 1)
	_ = d.Set("scheduled_time", "2026-01-01T00:00:00Z")
	_ = d.Set("group_ids", []interface{}{1})
	_ = d.Set("type", 0)

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
}

// TestEdgeUpdSchedCov2_Read_HTTPError covers the >=400 (non-404) branch of Read.
func TestEdgeUpdSchedCov2_Read_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_update_schedules/21", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourcePortainerEdgeUpdateSchedules()
	d := r.TestResourceData()
	d.SetId("21")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestEdgeUpdSchedCov2_Delete_404 covers the 404 branch of Delete (treated as
// success).
func TestEdgeUpdSchedCov2_Delete_404(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/edge_update_schedules/21", RespondString(http.StatusNotFound, "application/json", `{"message":"gone"}`))

	r := resourcePortainerEdgeUpdateSchedules()
	d := r.TestResourceData()
	d.SetId("21")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow 404, got: %v", err)
	}
}

// TestEdgeUpdSchedCov2_Delete_HTTPError covers the >=400 (non-404) branch of
// Delete.
func TestEdgeUpdSchedCov2_Delete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/edge_update_schedules/21", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourcePortainerEdgeUpdateSchedules()
	d := r.TestResourceData()
	d.SetId("21")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500 delete, got nil")
	}
}
