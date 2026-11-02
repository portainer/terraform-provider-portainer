package internal

import (
	"net/http"
	"reflect"
	"strings"
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

// TestKubernetesIngressControllersDelete_RestoresViaPUT verifies that Delete
// re-PUTs the controllers with Availability=true (no DELETE endpoint exists):
// Portainer's default, where Availability=false blocked the classes for the
// whole cluster after a destroy.
func TestKubernetesIngressControllersDelete_RestoresViaPUT(t *testing.T) {
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
	if payload[0]["Availability"] != true {
		t.Errorf("expected Availability=true on delete, got %v", payload[0]["Availability"])
	}
}

// TestRefreshIngressControllers is the regression test for controllers read
// back verbatim: the cluster's other controllers and the server-side name,
// type, used and new fields planned a change (a replacement per namespace) on
// every run. Only the configured classes are kept, with their availability.
func TestRefreshIngressControllers(t *testing.T) {
	actual := []IngressController{
		{ClassName: "traefik", Name: "traefik.io/ingress-controller", Type: "traefik", Availability: false, Used: true},
		{ClassName: "nginx", Name: "k8s.io/ingress-nginx", Type: "nginx", Availability: true},
	}
	configured := []interface{}{
		map[string]interface{}{"class_name": "traefik", "name": "traefik", "type": "custom", "availability": true, "used": false, "new": false},
		map[string]interface{}{"class_name": "gone", "name": "gone", "type": "custom", "availability": true, "used": false, "new": false},
	}

	got := refreshIngressControllers(actual, configured)
	want := []map[string]interface{}{
		{"class_name": "traefik", "name": "traefik", "type": "custom", "availability": false, "used": false, "new": false},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("configured: expected %v, got %v", want, got)
	}

	if imported := refreshIngressControllers(actual, nil); len(imported) != 2 || imported[1]["class_name"] != "nginx" {
		t.Errorf("import: expected every controller, got %v", imported)
	}
}

// TestKubernetesIngressControllersCreate_UnknownClass verifies a class the
// cluster does not have fails the apply: Portainer ignores it, and every
// later plan would add it again.
func TestKubernetesIngressControllersCreate_UnknownClass(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/kubernetes/1/ingresscontrollers", RespondString(http.StatusNoContent, "", ""))
	mock.On("GET", "/kubernetes/1/ingresscontrollers", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"ClassName": "traefik", "Name": "traefik.io/ingress-controller", "Type": "traefik", "Availability": true},
	}))

	r := resourceKubernetesIngressControllers()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("controllers", []interface{}{
		map[string]interface{}{"class_name": "nginx", "name": "nginx", "type": "nginx", "availability": true, "used": false, "new": false},
	})

	err := rcCreate(r, d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), `ingress class "nginx" is not in the cluster; available classes: traefik`) {
		t.Fatalf("expected the unknown class error, got %v", err)
	}
	if mock.FindRequest("PUT", "/kubernetes/1/ingresscontrollers") != nil {
		t.Error("a rejected list must not be sent")
	}
}
