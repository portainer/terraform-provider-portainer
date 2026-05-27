package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceTagRead_HappyPath verifies that the SDK-based list call is
// made and the matched tag's ID is populated.
//
// Note: the Portainer Tag model serializes ID as the lowercase JSON key "id"
// while most others use "Id". Match the swagger model exactly.
func TestDataSourceTagRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/tags", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": 1, "Name": "production"},
		{"id": 2, "Name": "staging"},
	}))

	ds := dataSourceTag()
	d := ds.TestResourceData()
	_ = d.Set("name", "staging")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "2" {
		t.Errorf("expected ID %q, got %q", "2", d.Id())
	}
	if mock.FindRequest("GET", "/tags") == nil {
		t.Error("expected GET /tags to be sent")
	}
}

// TestDataSourceTagRead_NotFound verifies the error path when the name is
// not in the returned list.
func TestDataSourceTagRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/tags", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"id": 1, "Name": "production"},
	}))

	ds := dataSourceTag()
	d := ds.TestResourceData()
	_ = d.Set("name", "ghost")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error when tag not found, got nil")
	}
}

// TestDataSourceTagRead_HTTPError verifies HTTP errors propagate.
func TestDataSourceTagRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/tags", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	ds := dataSourceTag()
	d := ds.TestResourceData()
	_ = d.Set("name", "production")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
