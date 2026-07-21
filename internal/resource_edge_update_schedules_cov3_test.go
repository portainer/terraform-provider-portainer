package internal

import (
	"net/http"
	"testing"
)

// TestEdgeUpdSchedCov3_Create_ResponseDecodeError covers the branch where the
// POST /edge_update_schedules response is a 2xx but carries an undecodable body,
// so the json.Decode of the response struct fails and Create returns an error.
func TestEdgeUpdSchedCov3_Create_ResponseDecodeError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/edge_update_schedules", RespondString(http.StatusOK, "application/json", `not-json`))

	r := resourcePortainerEdgeUpdateSchedules()
	d := r.TestResourceData()
	_ = d.Set("name", "x")
	_ = d.Set("agent_image", "a")
	_ = d.Set("updater_image", "u")
	_ = d.Set("registry_id", 1)
	_ = d.Set("scheduled_time", "2026-01-01T00:00:00Z")
	_ = d.Set("group_ids", []interface{}{1})
	_ = d.Set("type", 0)

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected decode error on malformed create response, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestEdgeUpdSchedCov3_Read_DecodeError covers the json.Decode failure branch of
// Read: a 200 response with a body that is not a valid schedule object.
func TestEdgeUpdSchedCov3_Read_DecodeError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_update_schedules/21", RespondString(http.StatusOK, "application/json", `not-json`))

	r := resourcePortainerEdgeUpdateSchedules()
	d := r.TestResourceData()
	d.SetId("21")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected decode error on malformed read response, got nil")
	}
}
