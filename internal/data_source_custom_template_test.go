package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceCustomTemplateRead_HappyPath verifies the SDK-routed list call
// to /custom_templates is filtered by title and populates computed fields.
// (SDK transport prepends /api which the mock dispatcher strips.)
func TestDataSourceCustomTemplateRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/custom_templates", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Title": "other", "Description": "x", "Type": 1},
		{"Id": 42, "Title": "wanted", "Description": "my-desc", "Type": 2},
	}))

	ds := dataSourceCustomTemplate()
	d := ds.TestResourceData()
	_ = d.Set("title", "wanted")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "42" {
		t.Errorf("expected ID %q, got %q", "42", d.Id())
	}
	if got := d.Get("description"); got != "my-desc" {
		t.Errorf("description: expected %q, got %v", "my-desc", got)
	}
	if got := d.Get("type"); got != 2 {
		t.Errorf("type: expected 2, got %v", got)
	}
}

// TestDataSourceCustomTemplateRead_NotFound verifies that a missing title
// returns an error.
func TestDataSourceCustomTemplateRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/custom_templates", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Title": "other"},
	}))

	ds := dataSourceCustomTemplate()
	d := ds.TestResourceData()
	_ = d.Set("title", "missing")

	err := rcRead(ds, d, mock.Client())
	if err == nil {
		t.Fatal("expected error when title not found, got nil")
	}
}

// TestDataSourceCustomTemplateRead_HTTPError verifies that an HTTP 500 (one of
// the SDK's documented error codes) is surfaced as an error.
func TestDataSourceCustomTemplateRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/custom_templates", RespondString(http.StatusInternalServerError,
		"application/json", `{"message":"boom"}`))

	ds := dataSourceCustomTemplate()
	d := ds.TestResourceData()
	_ = d.Set("title", "wanted")

	err := rcRead(ds, d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
