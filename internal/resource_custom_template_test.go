package internal

import (
	"net/http"
	"testing"
)

// TestCustomTemplateCreate_FromString_HappyPath verifies the string-content
// create path: GET list to confirm no existing template, POST to
// /custom_templates/create/string, and ID is set from the response.
func TestCustomTemplateCreate_FromString_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	// findExistingCustomTemplateByTitle lists all templates first.
	mock.On("GET", "/custom_templates", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Title": "other"},
	}))

	mock.On("POST", "/custom_templates/create/string", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":          77,
		"Title":       "my-template",
		"Description": "desc",
		"Note":        "note",
		"Platform":    1,
		"Type":        1,
	}))

	r := resourceCustomTemplate()
	d := r.TestResourceData()
	_ = d.Set("title", "my-template")
	_ = d.Set("description", "desc")
	_ = d.Set("note", "note")
	_ = d.Set("platform", 1)
	_ = d.Set("type", 1)
	_ = d.Set("file_content", "version: '3'\nservices: {}\n")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "77" {
		t.Errorf("ID: got %q, want %q", d.Id(), "77")
	}
	if mock.FindRequest("POST", "/custom_templates/create/string") == nil {
		t.Error("expected POST /custom_templates/create/string")
	}
}

// TestCustomTemplateRead_HappyPath verifies that Read fetches the template
// and populates schema fields.
func TestCustomTemplateRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/custom_templates/42", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":              42,
		"Title":           "loaded",
		"Description":     "from-server",
		"Note":            "n",
		"Platform":        2,
		"Type":            3,
		"Logo":            "logo.png",
		"edgeTemplate":    true,
		"isComposeFormat": false,
	}))

	r := resourceCustomTemplate()
	d := r.TestResourceData()
	d.SetId("42")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("title"); got != "loaded" {
		t.Errorf("title: got %v", got)
	}
	if got := d.Get("description"); got != "from-server" {
		t.Errorf("description: got %v", got)
	}
	if got := d.Get("platform"); got != 2 {
		t.Errorf("platform: got %v", got)
	}
	if got := d.Get("edge_template"); got != true {
		t.Errorf("edge_template: got %v", got)
	}
}

// TestCustomTemplateDelete_HappyPath verifies DELETE is sent.
func TestCustomTemplateDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/custom_templates/15", RespondString(http.StatusNoContent, "", ""))

	r := resourceCustomTemplate()
	d := r.TestResourceData()
	d.SetId("15")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/custom_templates/15") == nil {
		t.Error("expected DELETE /custom_templates/15")
	}
}

// TestCustomTemplateCreate_ExistingTitleTriggersUpdate verifies that if a
// template with the same title already exists, Create reuses its ID and
// dispatches to Update (PUT /custom_templates/{id}).
func TestCustomTemplateCreate_ExistingTitleTriggersUpdate(t *testing.T) {
	mock := NewMockServer(t)

	// Existing template with the same title
	mock.On("GET", "/custom_templates", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": 99, "Title": "dup"},
	}))
	mock.On("PUT", "/custom_templates/99", RespondString(http.StatusOK, "application/json", `{}`))
	mock.On("GET", "/custom_templates/99", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":       99,
		"Title":    "dup",
		"Platform": 1,
		"Type":     1,
	}))

	r := resourceCustomTemplate()
	d := r.TestResourceData()
	_ = d.Set("title", "dup")
	_ = d.Set("description", "desc")
	_ = d.Set("note", "n")
	_ = d.Set("platform", 1)
	_ = d.Set("type", 1)
	_ = d.Set("file_content", "x")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "99" {
		t.Errorf("ID: got %q want 99", d.Id())
	}
	if mock.FindRequest("PUT", "/custom_templates/99") == nil {
		t.Error("expected PUT /custom_templates/99 for existing template path")
	}
}
