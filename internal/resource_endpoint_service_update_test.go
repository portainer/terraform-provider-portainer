package internal

import (
	"net/http"
	"testing"
)

// TestEndpointServiceUpdateCreate_HappyPath verifies the full flow:
// 1) GET /endpoints/{id}/docker/services to resolve service name -> ID
// 2) PUT /endpoints/{id}/forceupdateservice with the resolved ID + pullImage
// 3) composite ID "<endpoint>-<service>"
func TestEndpointServiceUpdateCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/2/docker/services", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"ID": "svc-abc", "Spec": map[string]interface{}{"Name": "web"}},
		{"ID": "svc-def", "Spec": map[string]interface{}{"Name": "db"}},
	}))
	mock.On("PUT", "/endpoints/2/forceupdateservice", RespondJSON(http.StatusOK, map[string]interface{}{
		"Warnings": []string{},
	}))

	r := resourceEndpointServiceUpdate()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("service_name", "web")
	_ = d.Set("pull_image", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "2-svc-abc" {
		t.Errorf("expected ID %q, got %q", "2-svc-abc", d.Id())
	}

	put := mock.FindRequest("PUT", "/endpoints/2/forceupdateservice")
	if put == nil {
		t.Fatal("expected PUT /endpoints/2/forceupdateservice")
	}
	var payload map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode PUT body: %v", err)
	}
	if payload["pullImage"] != true {
		t.Errorf("pullImage: expected true, got %v", payload["pullImage"])
	}
	if payload["serviceID"] != "svc-abc" {
		t.Errorf("serviceID: expected svc-abc, got %v", payload["serviceID"])
	}
}

// TestEndpointServiceUpdateCreate_ServiceNotFound verifies that a missing
// service name yields an error and no PUT is sent.
func TestEndpointServiceUpdateCreate_ServiceNotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/2/docker/services", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"ID": "svc-1", "Spec": map[string]interface{}{"Name": "other"}},
	}))

	r := resourceEndpointServiceUpdate()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("service_name", "missing")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when service name is not in the list, got nil")
	}
	if mock.FindRequest("PUT", "/endpoints/2/forceupdateservice") != nil {
		t.Error("did not expect PUT when service was not found")
	}
}

// TestEndpointServiceUpdateCreate_ListHTTPError verifies that a non-200 on
// the service list propagates an error.
func TestEndpointServiceUpdateCreate_ListHTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/2/docker/services", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	r := resourceEndpointServiceUpdate()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("service_name", "web")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on list 500, got nil")
	}
}

// TestEndpointServiceUpdateCreate_PutHTTPError verifies that a non-200 on
// the forceupdateservice call propagates an error.
func TestEndpointServiceUpdateCreate_PutHTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/2/docker/services", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"ID": "svc-1", "Spec": map[string]interface{}{"Name": "web"}},
	}))
	mock.On("PUT", "/endpoints/2/forceupdateservice", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"nope"}`,
	))

	r := resourceEndpointServiceUpdate()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("service_name", "web")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on PUT 400, got nil")
	}
}

// TestEndpointServiceUpdate_ReadAndDelete_AreNoop verifies that Read and
// Delete (schema.Noop) do not error and do not call the API.
func TestEndpointServiceUpdate_ReadAndDelete_AreNoop(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceEndpointServiceUpdate()
	d := r.TestResourceData()
	d.SetId("2-svc-abc")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read (noop) failed: %v", err)
	}
	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete (noop) failed: %v", err)
	}
	if len(mock.Requests()) != 0 {
		t.Errorf("expected zero requests for Noop Read/Delete, got %d", len(mock.Requests()))
	}
}
