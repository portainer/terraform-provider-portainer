package internal

import (
	"net/http"
	"testing"
)

// TestRegistryCov3_Read_QuayGitlabEcr covers the Quay, Gitlab and Ecr
// conditional set branches of resourceRegistryRead, which the existing
// happy-path test (GitHub only) does not reach.
func TestRegistryCov3_Read_QuayGitlabEcr(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":             7,
		"Name":           "multi",
		"URL":            "registry.example.com",
		"Type":           7,
		"Authentication": true,
		"Username":       "svc",
		"Quay": map[string]interface{}{
			"UseOrganisation":  true,
			"OrganisationName": "quayorg",
		},
		"Gitlab": map[string]interface{}{
			"InstanceURL": "https://gitlab.example.com",
		},
		"Ecr": map[string]interface{}{
			"Region": "eu-central-1",
		},
	}))

	r := resourceRegistry()
	d := r.TestResourceData()
	d.SetId("7")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("quay_use_organisation"); got != true {
		t.Errorf("quay_use_organisation: expected true, got %v", got)
	}
	if got := d.Get("quay_organisation_name"); got != "quayorg" {
		t.Errorf("quay_organisation_name: got %v", got)
	}
	if got := d.Get("instance_url"); got != "https://gitlab.example.com" {
		t.Errorf("instance_url: got %v", got)
	}
	if got := d.Get("aws_region"); got != "eu-central-1" {
		t.Errorf("aws_region: got %v", got)
	}
}

// TestRegistryCov3_Read_InspectError covers the non-404 error branch of
// resourceRegistryRead: a 500 from RegistryInspect must surface as an error
// (not silently clear the ID like a 404 does).
func TestRegistryCov3_Read_InspectError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/registries/8", RespondString(
		http.StatusInternalServerError, "application/json",
		`{"message":"boom"}`,
	))

	r := resourceRegistry()
	d := r.TestResourceData()
	d.SetId("8")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500 inspect, got nil")
	}
}
