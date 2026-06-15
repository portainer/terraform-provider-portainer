package internal

import (
	"net/http"
	"testing"
)

// TestDockerConfigCreate_ExistingTriggersUpdate verifies that when a config with
// the same name already exists, Create adopts its ID and routes to Update
// (POST .../update) rather than creating a new one.
func TestDockerConfigCreate_ExistingTriggersUpdate(t *testing.T) {
	mock := NewMockServer(t)

	// Duplicate-detection GET returns a config matching the requested name.
	mock.On("GET", "/endpoints/1/docker/configs", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"ID":   "existing-id",
			"Spec": map[string]interface{}{"Name": "my-config"},
		},
	}))
	mock.On("POST", "/endpoints/1/docker/configs/existing-id/update", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/endpoints/1/docker/configs/existing-id", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID":   "existing-id",
		"Spec": map[string]interface{}{"Name": "my-config", "Labels": map[string]interface{}{}},
	}))

	r := resourceDockerConfig()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "my-config")
	_ = d.Set("data", "c2VjcmV0")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "existing-id" {
		t.Errorf("expected adopted ID %q, got %q", "existing-id", d.Id())
	}
	if mock.FindRequest("POST", "/endpoints/1/docker/configs/existing-id/update") == nil {
		t.Error("expected update POST for existing config")
	}
}

// TestDockerConfigCreate_ListError verifies that a non-200 from the dedup list
// surfaces an error from findExistingDockerConfigByName.
func TestDockerConfigCreate_ListError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/configs", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDockerConfig()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "x")
	_ = d.Set("data", "y")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when config list fails, got nil")
	}
}

// TestDockerConfigCreate_WithTemplating verifies the templating block is encoded
// in the create payload.
func TestDockerConfigCreate_WithTemplating(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/configs", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/endpoints/1/docker/configs/create", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID": "cfg1",
	}))

	r := resourceDockerConfig()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "cfg")
	_ = d.Set("data", "ZGF0YQ==")
	_ = d.Set("templating", map[string]interface{}{"name": "golang"})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	post := mock.FindRequest("POST", "/endpoints/1/docker/configs/create")
	if post == nil {
		t.Fatal("expected create POST")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if _, ok := payload["Templating"]; !ok {
		t.Error("expected Templating in payload")
	}
}

// TestDockerConfigUpdate_HTTPError verifies a non-2xx update surfaces an error.
func TestDockerConfigUpdate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/configs/abc/update", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDockerConfig()
	d := r.TestResourceData()
	d.SetId("abc")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "x")
	_ = d.Set("data", "y")

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestDockerConfigDelete_HTTPError verifies a non-2xx/non-404 delete errors.
func TestDockerConfigDelete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/docker/configs/abc", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDockerConfig()
	d := r.TestResourceData()
	d.SetId("abc")
	_ = d.Set("endpoint_id", 1)

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestDockerConfigRead_WithTemplating verifies Read reconstructs the templating
// map from the inspect payload.
func TestDockerConfigRead_WithTemplating(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/configs/abc", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID": "abc",
		"Spec": map[string]interface{}{
			"Name": "cfg",
			"Templating": map[string]interface{}{
				"Name":    "golang",
				"Options": map[string]interface{}{"opt1": "v1"},
			},
		},
	}))

	r := resourceDockerConfig()
	d := r.TestResourceData()
	d.SetId("abc")
	_ = d.Set("endpoint_id", 1)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	templ := d.Get("templating").(map[string]interface{})
	if templ["name"] != "golang" {
		t.Errorf("templating.name: expected %q, got %v", "golang", templ["name"])
	}
	if templ["opt1"] != "v1" {
		t.Errorf("templating.opt1: expected %q, got %v", "v1", templ["opt1"])
	}
}
