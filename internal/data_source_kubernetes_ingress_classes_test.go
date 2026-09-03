package internal

import (
	"net/http"
	"testing"
)

// TestDataSourceKubernetesIngressClasses_HappyPath verifies the PascalCase
// payload of K8sIngressClass is decoded and the cluster default is surfaced.
func TestDataSourceKubernetesIngressClasses_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/1/ingressclasses", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Name": "traefik", "Controller": "traefik.io/ingress-controller", "IsDefault": false},
		{
			"Name": "nginx", "Controller": "k8s.io/ingress-nginx", "IsDefault": true,
			"Annotations": map[string]string{"ingressclass.kubernetes.io/is-default-class": "true"},
		},
	}))

	ds := dataSourceKubernetesIngressClasses()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 1)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	classes := d.Get("ingress_classes").([]interface{})
	if len(classes) != 2 {
		t.Fatalf("ingress_classes: expected 2 entries, got %d", len(classes))
	}
	first := classes[0].(map[string]interface{})
	if first["name"] != "traefik" || first["controller"] != "traefik.io/ingress-controller" {
		t.Errorf("ingress_classes[0] mismatch: %v", first)
	}
	if first["annotations"] == nil {
		t.Error("annotations: expected an empty map rather than nil")
	}

	second := classes[1].(map[string]interface{})
	if second["is_default"] != true {
		t.Errorf("ingress_classes[1].is_default: expected true, got %v", second["is_default"])
	}
	if got := d.Get("default_ingress_class"); got != "nginx" {
		t.Errorf("default_ingress_class: expected %q, got %v", "nginx", got)
	}
	if got := d.Id(); got != "1/ingressclasses" {
		t.Errorf("id: got %q", got)
	}
}

// TestDataSourceKubernetesIngressClasses_NoDefault verifies a cluster without a
// default class reports an empty string rather than picking one arbitrarily.
func TestDataSourceKubernetesIngressClasses_NoDefault(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/1/ingressclasses", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Name": "traefik", "Controller": "traefik.io/ingress-controller", "IsDefault": false},
	}))

	ds := dataSourceKubernetesIngressClasses()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 1)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("default_ingress_class"); got != "" {
		t.Errorf("default_ingress_class: expected empty, got %v", got)
	}
}
