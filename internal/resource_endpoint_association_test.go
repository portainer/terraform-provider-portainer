package internal

import (
	"net/http"
	"testing"
)

// TestEndpointAssociationCreate_HappyPath verifies that Create sends a
// PUT /endpoints/{id}/association and sets the ID to the endpoint ID.
func TestEndpointAssociationCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/endpoints/12/association", RespondString(http.StatusNoContent, "", ""))

	r := resourceEndpointAssociation()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 12)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "12" {
		t.Errorf("expected ID %q, got %q", "12", d.Id())
	}
	if mock.FindRequest("PUT", "/endpoints/12/association") == nil {
		t.Error("expected PUT /endpoints/12/association to be sent")
	}
}

// TestEndpointAssociationCreate_HTTPError verifies that a non-2xx surfaces an
// error.
func TestEndpointAssociationCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/endpoints/4/association", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	r := resourceEndpointAssociation()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 4)

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestEndpointAssociationRead_HappyPath verifies that Read confirms the
// endpoint exists via GET /endpoints/{id} and keeps it in state.
func TestEndpointAssociationRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/15", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   15,
		"Name": "edge-1",
	}))

	r := resourceEndpointAssociation()
	d := r.TestResourceData()
	d.SetId("15")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("endpoint_id"); got != 15 {
		t.Errorf("endpoint_id: expected 15, got %v", got)
	}
	if d.Id() != "15" {
		t.Errorf("expected ID to remain %q, got %q", "15", d.Id())
	}
}

// TestEndpointAssociationRead_404_ClearsID verifies a 404 removes from state.
func TestEndpointAssociationRead_404_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/77", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"not found"}`,
	))

	r := resourceEndpointAssociation()
	d := r.TestResourceData()
	d.SetId("77")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after 404, got %q", d.Id())
	}
}

// TestEndpointAssociationDelete_ClearsID verifies Delete is state-only.
func TestEndpointAssociationDelete_ClearsID(t *testing.T) {
	r := resourceEndpointAssociation()
	d := r.TestResourceData()
	d.SetId("33")

	if err := r.Delete(d, nil); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}
