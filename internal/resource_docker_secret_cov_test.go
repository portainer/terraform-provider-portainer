package internal

import (
	"net/http"
	"testing"
)

// TestDockerSecretCreate_ExistingTriggersUpdate verifies that when a secret with
// the same name already exists, Create adopts its ID and routes to Update.
func TestDockerSecretCreate_ExistingTriggersUpdate(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/secrets", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"ID":   "existing-sec",
			"Spec": map[string]interface{}{"Name": "db-password"},
		},
	}))
	mock.On("POST", "/endpoints/1/docker/secrets/existing-sec/update", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/endpoints/1/docker/secrets/existing-sec", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID":   "existing-sec",
		"Spec": map[string]interface{}{"Name": "db-password", "Labels": map[string]interface{}{}},
	}))

	r := resourceDockerSecret()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "db-password")
	_ = d.Set("data", "c2VjcmV0")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "existing-sec" {
		t.Errorf("expected adopted ID %q, got %q", "existing-sec", d.Id())
	}
	if mock.FindRequest("POST", "/endpoints/1/docker/secrets/existing-sec/update") == nil {
		t.Error("expected update POST for existing secret")
	}
}

// TestDockerSecretCreate_ListError verifies a non-200 from the dedup list
// surfaces an error.
func TestDockerSecretCreate_ListError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/secrets", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDockerSecret()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "x")
	_ = d.Set("data", "y")

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error when secret list fails, got nil")
	}
}

// TestDockerSecretCreate_WithDriverAndTemplating exercises the driver and
// templating branches of buildSecretPayload.
func TestDockerSecretCreate_WithDriverAndTemplating(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/secrets", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/endpoints/1/docker/secrets/create", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID": "sec1",
	}))

	r := resourceDockerSecret()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "s")
	_ = d.Set("data", "ZGF0YQ==")
	_ = d.Set("driver", map[string]interface{}{"name": "vault"})
	_ = d.Set("templating", map[string]interface{}{"name": "golang"})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	post := mock.FindRequest("POST", "/endpoints/1/docker/secrets/create")
	if post == nil {
		t.Fatal("expected create POST")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	if _, ok := payload["Driver"]; !ok {
		t.Error("expected Driver in payload")
	}
	if _, ok := payload["Templating"]; !ok {
		t.Error("expected Templating in payload")
	}
}

// TestDockerSecretUpdate_HTTPError verifies a non-2xx update surfaces an error.
func TestDockerSecretUpdate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/secrets/sec_1/update", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDockerSecret()
	d := r.TestResourceData()
	d.SetId("sec_1")
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "x")
	_ = d.Set("data", "y")

	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestDockerSecretDelete_HTTPError verifies a non-2xx/non-404 delete errors.
func TestDockerSecretDelete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/docker/secrets/sec_1", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDockerSecret()
	d := r.TestResourceData()
	d.SetId("sec_1")
	_ = d.Set("endpoint_id", 1)

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestDockerSecretRead_HTTPError verifies a non-200/404 read surfaces an error.
func TestDockerSecretRead_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/secrets/sec_1", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDockerSecret()
	d := r.TestResourceData()
	d.SetId("sec_1")
	_ = d.Set("endpoint_id", 1)

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestDockerSecretRead_TemplatingRoundTrip reproduces the shape Portainer
// returns for driver/templating ({Name, Options:{...}}). Read must flatten the
// nested Options back into the flat TypeMap[string]string schema instead of
// setting the raw nested object (which previously failed type conversion on
// refresh once d.Set errors were no longer ignored).
func TestDockerSecretRead_TemplatingRoundTrip(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/secrets/sec_1", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID": "sec_1",
		"Spec": map[string]interface{}{
			"Name": "app-key.crt",
			"Templating": map[string]interface{}{
				"Name": "some-driver",
				"Options": map[string]interface{}{
					"name":    "some-driver",
					"OptionA": "value for driver-specific option A",
				},
			},
		},
	}))

	r := resourceDockerSecret()
	d := r.TestResourceData()
	d.SetId("sec_1")
	_ = d.Set("endpoint_id", 1)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	templ := d.Get("templating").(map[string]interface{})
	if templ["name"] != "some-driver" || templ["OptionA"] != "value for driver-specific option A" {
		t.Errorf("templating not flattened correctly, got %v", templ)
	}
}
