package internal

import (
	"net/http"
	"testing"
)

// Read and Delete used to treat every lookup failure as "gone": a timeout or a
// 5xx dropped the resource control from state and the next apply tried to
// create one that still existed. Only a missing stack or a stack without a
// resource control may drop it now.

func TestResourceControlRead_ErrorKeepsState(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/stacks/5", RespondJSON(http.StatusInternalServerError, map[string]string{"message": "database is locked"}))

	r := resourceResourceControl()
	d := r.TestResourceData()
	d.SetId("12")
	_ = d.Set("type", 6)
	_ = d.Set("resource_id", "5")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected the lookup error to be reported")
	}
	if d.Id() != "12" {
		t.Errorf("expected the resource to stay in state, got ID %q", d.Id())
	}
}

func TestResourceControlRead_MissingStackDropsState(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/stacks/5", RespondJSON(http.StatusNotFound, map[string]string{"message": "Unable to find a stack with the specified identifier inside the database"}))

	r := resourceResourceControl()
	d := r.TestResourceData()
	d.SetId("12")
	_ = d.Set("type", 6)
	_ = d.Set("resource_id", "5")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected the resource dropped for a deleted stack, got ID %q", d.Id())
	}
}

func TestResourceControlRead_StackWithoutResourceControlDropsState(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/stacks/5", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 5}))

	r := resourceResourceControl()
	d := r.TestResourceData()
	d.SetId("12")
	_ = d.Set("type", 6)
	_ = d.Set("resource_id", "5")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected the resource dropped without a resource control, got ID %q", d.Id())
	}
}

func TestResourceControlDelete_ErrorIsReported(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/stacks/5", RespondJSON(http.StatusInternalServerError, map[string]string{"message": "database is locked"}))

	r := resourceResourceControl()
	d := r.TestResourceData()
	d.SetId("12")
	_ = d.Set("type", 6)
	_ = d.Set("resource_id", "5")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected the lookup error to be reported instead of forgetting the resource control")
	}
}
