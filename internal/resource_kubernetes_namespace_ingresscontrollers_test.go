package internal

import (
	"net/http"
	"testing"
)

// TestKubernetesNamespaceIngressControllersCreate_HappyPath verifies that
// Create sends PUT /kubernetes/{id}/namespaces/{ns}/ingresscontrollers and
// builds the composite ID "<envID>:<ns>". Create calls Read at the end, so the
// GET handler is also registered.
func TestKubernetesNamespaceIngressControllersCreate_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/kubernetes/2/namespaces/prod/ingresscontrollers",
		RespondString(http.StatusOK, "application/json", `[]`))
	mock.On("GET", "/kubernetes/2/namespaces/prod/ingresscontrollers",
		RespondJSON(http.StatusOK, []map[string]interface{}{
			{
				"Name":         "nginx-controller",
				"ClassName":    "nginx",
				"Type":         "nginx",
				"Availability": true,
				"Used":         true,
				"New":          false,
			},
		}))

	r := resourceKubernetesNamespaceIngressControllers()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 2)
	_ = d.Set("namespace", "prod")
	_ = d.Set("controllers", []interface{}{
		map[string]interface{}{
			"name":         "nginx-controller",
			"class_name":   "nginx",
			"type":         "nginx",
			"availability": true,
			"used":         true,
			"new":          false,
		},
	})

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "2:prod" {
		t.Errorf("expected ID %q, got %q", "2:prod", d.Id())
	}

	put := mock.FindRequest("PUT", "/kubernetes/2/namespaces/prod/ingresscontrollers")
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
	if payload[0]["Availability"] != true {
		t.Errorf("expected Availability=true, got %v", payload[0]["Availability"])
	}
}

// TestKubernetesNamespaceIngressControllersCreate_HTTPError verifies error surfaces.
func TestKubernetesNamespaceIngressControllersCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/kubernetes/1/namespaces/default/ingresscontrollers",
		RespondString(http.StatusBadRequest, "application/json", `{"message":"bad"}`))

	r := resourceKubernetesNamespaceIngressControllers()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "default")
	_ = d.Set("controllers", []interface{}{
		map[string]interface{}{
			"name":         "nginx-controller",
			"class_name":   "nginx",
			"type":         "nginx",
			"availability": true,
			"used":         true,
			"new":          false,
		},
	})

	if err := r.Create(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
}

// TestKubernetesNamespaceIngressControllersRead_HappyPath verifies Read parses
// the response and populates state.
func TestKubernetesNamespaceIngressControllersRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/1/namespaces/default/ingresscontrollers",
		RespondJSON(http.StatusOK, []map[string]interface{}{
			{
				"Name":         "traefik-controller",
				"ClassName":    "traefik",
				"Type":         "traefik",
				"Availability": true,
				"Used":         true,
				"New":          false,
			},
		}))

	r := resourceKubernetesNamespaceIngressControllers()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "default")

	if err := r.Read(d, mock.Client()); err != nil {
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

// TestKubernetesNamespaceIngressControllersDelete_DisablesViaPUT verifies
// Delete re-PUTs the controllers with Availability=false.
func TestKubernetesNamespaceIngressControllersDelete_DisablesViaPUT(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("PUT", "/kubernetes/2/namespaces/prod/ingresscontrollers",
		RespondString(http.StatusOK, "application/json", `[]`))

	r := resourceKubernetesNamespaceIngressControllers()
	d := r.TestResourceData()
	d.SetId("2:prod")
	_ = d.Set("environment_id", 2)
	_ = d.Set("namespace", "prod")
	_ = d.Set("controllers", []interface{}{
		map[string]interface{}{
			"name":         "nginx-controller",
			"class_name":   "nginx",
			"type":         "nginx",
			"availability": true,
			"used":         true,
			"new":          false,
		},
	})

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	put := mock.FindRequest("PUT", "/kubernetes/2/namespaces/prod/ingresscontrollers")
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
