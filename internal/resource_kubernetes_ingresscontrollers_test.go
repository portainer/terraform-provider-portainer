package internal

import (
	"net/http"
	"testing"
)

// TestKubernetesIngressControllersCreate_HappyPath verifies that Create sends
// PUT /kubernetes/{id}/ingresscontrollers with the marshaled controllers list.
// Note: Create calls Read at the end, so we register the GET mock too.
func TestKubernetesIngressControllersCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/kubernetes/1/ingresscontrollers",
		RespondString(http.StatusOK, "application/json", `[]`))
	mock.On("GET", "/kubernetes/1/ingresscontrollers",
		RespondJSON(http.StatusOK, []map[string]interface{}{
			{
				"Availability": true,
				"ClassName":    "nginx",
				"Name":         "nginx-controller",
				"New":          false,
				"Type":         "nginx",
				"Used":         true,
			},
		}))

	r := resourceKubernetesIngressControllers()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("controllers", []interface{}{
		map[string]interface{}{
			"availability": true,
			"class_name":   "nginx",
			"name":         "nginx-controller",
			"new":          false,
			"type":         "nginx",
			"used":         true,
		},
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "1" {
		t.Errorf("expected ID %q, got %q", "1", d.Id())
	}

	put := mock.FindRequest("PUT", "/kubernetes/1/ingresscontrollers")
	if put == nil {
		t.Fatal("expected PUT request to be recorded")
	}
	var payload []map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(payload) != 1 || payload[0]["Name"] != "nginx-controller" {
		t.Errorf("unexpected payload: %+v", payload)
	}
}

// TestKubernetesIngressControllersCreate_HTTPError verifies HTTP error surfaces.
func TestKubernetesIngressControllersCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/kubernetes/1/ingresscontrollers",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	r := resourceKubernetesIngressControllers()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("controllers", []interface{}{
		map[string]interface{}{
			"availability": true,
			"class_name":   "nginx",
			"name":         "nginx-controller",
			"new":          false,
			"type":         "nginx",
			"used":         true,
		},
	})

	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestKubernetesIngressControllersRead_HappyPath verifies that Read parses the
// server response and populates state.
func TestKubernetesIngressControllersRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/1/ingresscontrollers",
		RespondJSON(http.StatusOK, []map[string]interface{}{
			{
				"Availability": true,
				"ClassName":    "traefik",
				"Name":         "traefik-controller",
				"New":          false,
				"Type":         "traefik",
				"Used":         true,
			},
		}))

	r := resourceKubernetesIngressControllers()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	got := d.Get("controllers").([]interface{})
	if len(got) != 1 {
		t.Fatalf("expected 1 controller, got %d", len(got))
	}
	first := got[0].(map[string]interface{})
	if first["name"] != "traefik-controller" {
		t.Errorf("expected name %q, got %v", "traefik-controller", first["name"])
	}
}

// TestKubernetesIngressControllersRead_NotFound verifies 404 clears the ID.
func TestKubernetesIngressControllersRead_NotFound(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/1/ingresscontrollers",
		RespondString(http.StatusNotFound, "", ""))

	r := resourceKubernetesIngressControllers()
	d := r.TestResourceData()
	d.SetId("1")
	_ = d.Set("environment_id", 1)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared on 404, got %q", d.Id())
	}
}

// TestKubernetesIngressControllersDelete_DisablesViaPUT verifies that Delete
// re-PUTs the controllers with Availability=false (no DELETE endpoint exists).
func TestKubernetesIngressControllersDelete_DisablesViaPUT(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/kubernetes/1/ingresscontrollers",
		RespondString(http.StatusOK, "application/json", `[]`))

	r := resourceKubernetesIngressControllers()
	d := r.TestResourceData()
	d.SetId("1")
	_ = d.Set("environment_id", 1)
	_ = d.Set("controllers", []interface{}{
		map[string]interface{}{
			"availability": true,
			"class_name":   "nginx",
			"name":         "nginx-controller",
			"new":          false,
			"type":         "nginx",
			"used":         true,
		},
	})

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/kubernetes/1/ingresscontrollers")
	if put == nil {
		t.Fatal("expected PUT request to be recorded for disable-on-delete")
	}
	var payload []map[string]interface{}
	if err := put.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if len(payload) != 1 {
		t.Fatalf("expected 1 controller in payload, got %d", len(payload))
	}
	if payload[0]["Availability"] != false {
		t.Errorf("expected Availability=false on delete, got %v", payload[0]["Availability"])
	}
}
