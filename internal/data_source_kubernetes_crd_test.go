package internal

import (
	"net/http"
	"strings"
	"testing"
)

// TestDataSourceKubernetesCRDRead_ByName fetches a single CRD by name and
// populates the crds list with one entry plus a deterministic ID.
func TestDataSourceKubernetesCRDRead_ByName(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/5/customresourcedefinitions/widgets.example.com",
		RespondJSON(http.StatusOK, map[string]interface{}{
			"name":             "widgets.example.com",
			"group":            "example.com",
			"scope":            "Namespaced",
			"creationDate":     "2024-01-02T03:04:05Z",
			"releaseName":      "widget-op",
			"releaseNamespace": "kube-system",
			"releaseVersion":   "1.2.3",
		}))

	ds := dataSourceKubernetesCRD()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 5)
	_ = d.Set("name", "widgets.example.com")

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if d.Id() != "5/widgets.example.com" {
		t.Errorf("expected ID %q, got %q", "5/widgets.example.com", d.Id())
	}

	crds := d.Get("crds").([]interface{})
	if len(crds) != 1 {
		t.Fatalf("expected 1 crd entry, got %d", len(crds))
	}
	c := crds[0].(map[string]interface{})
	if c["name"] != "widgets.example.com" {
		t.Errorf("name: expected widgets.example.com, got %v", c["name"])
	}
	if c["group"] != "example.com" {
		t.Errorf("group: expected example.com, got %v", c["group"])
	}
	if c["scope"] != "Namespaced" {
		t.Errorf("scope: expected Namespaced, got %v", c["scope"])
	}
	if c["release_name"] != "widget-op" {
		t.Errorf("release_name: expected widget-op, got %v", c["release_name"])
	}
}

// TestDataSourceKubernetesCRDRead_List fetches all CRDs for an environment and
// returns them as a list. The ID is a synthetic timestamp/envID string.
func TestDataSourceKubernetesCRDRead_List(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/5/customresourcedefinitions",
		RespondJSON(http.StatusOK, []map[string]interface{}{
			{
				"name":         "widgets.example.com",
				"group":        "example.com",
				"scope":        "Namespaced",
				"creationDate": "2024-01-02T03:04:05Z",
			},
			{
				"name":         "gadgets.example.com",
				"group":        "example.com",
				"scope":        "Cluster",
				"creationDate": "2024-02-03T04:05:06Z",
			},
		}))

	ds := dataSourceKubernetesCRD()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 5)

	if err := ds.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if !strings.HasSuffix(d.Id(), "/5") {
		t.Errorf("expected list ID to end with %q, got %q", "/5", d.Id())
	}

	crds := d.Get("crds").([]interface{})
	if len(crds) != 2 {
		t.Fatalf("expected 2 crd entries, got %d", len(crds))
	}
	first := crds[0].(map[string]interface{})
	if first["name"] != "widgets.example.com" {
		t.Errorf("first name: expected widgets.example.com, got %v", first["name"])
	}
}

// TestDataSourceKubernetesCRDRead_ByNameHTTPError surfaces HTTP 4xx/5xx.
func TestDataSourceKubernetesCRDRead_ByNameHTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/5/customresourcedefinitions/missing",
		RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`))

	ds := dataSourceKubernetesCRD()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 5)
	_ = d.Set("name", "missing")

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 404, got nil")
	}
}

// TestDataSourceKubernetesCRDRead_ListHTTPError surfaces HTTP 4xx/5xx on list.
func TestDataSourceKubernetesCRDRead_ListHTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/kubernetes/5/customresourcedefinitions",
		RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))

	ds := dataSourceKubernetesCRD()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 5)

	if err := ds.Read(d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}
