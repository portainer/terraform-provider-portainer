package internal

import (
	"net/http"
	"testing"
)

// TestTagCreate_HappyPath exercises resource_tag, which uses the generated
// SDK (client.Client.Tags.*), to verify the mock harness handles SDK-routed
// requests correctly.
func TestTagCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	// SDK calls hit /api/tags after the dispatcher strips the prefix tests
	// register handlers on the bare path.
	mock.On("POST", "/tags", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":   17,
		"Name": "production",
	}))
	mock.On("GET", "/tags", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 17, "Name": "production"},
		{"Id": 18, "Name": "staging"},
	}))

	r := resourceTag()
	d := r.TestResourceData()
	_ = d.Set("name", "production")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "17" {
		t.Errorf("expected ID %q, got %q", "17", d.Id())
	}
	if got := d.Get("name"); got != "production" {
		t.Errorf("name: expected %q, got %v", "production", got)
	}
}

// TestTagRead_NotInList verifies that when the tag ID is no longer present in
// the list response, the resource clears its ID (drift detection).
func TestTagRead_NotInList(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/tags", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "other"},
	}))

	r := resourceTag()
	d := r.TestResourceData()
	d.SetId("99")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after tag not found, got %q", d.Id())
	}
}

// TestTagDelete_HappyPath verifies the SDK Delete call is sent.
func TestTagDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/tags/5", RespondString(http.StatusNoContent, "", ""))

	r := resourceTag()
	d := r.TestResourceData()
	d.SetId("5")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if mock.FindRequest("DELETE", "/tags/5") == nil {
		t.Error("expected DELETE /tags/5 to be sent")
	}
}
