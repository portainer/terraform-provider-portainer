package internal

import (
	"net/http"
	"testing"
)

// TestResourceControl_ExternalStackLifecycle covers a stack deployed outside
// Portainer: its resource ID is "<endpointId>_<compose project>", which
// GET /stacks/{id} cannot resolve. Create must POST the control, Read must find
// it on the project's containers, and Delete must remove it by the stored ID.
func TestResourceControl_ExternalStackLifecycle(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/resource_controls", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 17}))
	mock.On("DELETE", "/resource_controls/17", RespondString(http.StatusNoContent, "application/json", ``))
	// The project's container carries the control once it has been created.
	mock.On("GET", "/endpoints/4/docker/containers/json", func(w http.ResponseWriter, r *http.Request) {
		container := map[string]interface{}{"Id": "c1"}
		if mock.FindRequest("POST", "/resource_controls") != nil {
			container["Portainer"] = map[string]interface{}{"ResourceControl": map[string]interface{}{
				"Id": 17, "Type": 6, "ResourceId": "4_server", "TeamAccesses": []map[string]interface{}{{"TeamId": 3}},
			}}
		}
		RespondJSON(http.StatusOK, []interface{}{container})(w, r)
	})

	r := resourceResourceControl()
	d := r.TestResourceData()
	_ = d.Set("resource_id", "4_server")
	_ = d.Set("type", 6)
	_ = d.Set("teams", []interface{}{3})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "17" {
		t.Fatalf("expected ID %q after create, got %q", "17", d.Id())
	}

	post := mock.FindRequest("POST", "/resource_controls")
	if post == nil {
		t.Fatal("expected POST /resource_controls")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode POST body: %v", err)
	}
	if payload["ResourceID"] != "4_server" || payload["Type"] != float64(6) {
		t.Errorf("unexpected POST payload: %v", payload)
	}

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "17" {
		t.Errorf("Read dropped the external stack control from state (ID %q)", d.Id())
	}
	if mock.FindRequest("GET", "/stacks/4_server") != nil {
		t.Error("Read must not look up an external stack through GET /stacks/{id}")
	}

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if mock.FindRequest("DELETE", "/resource_controls/17") == nil {
		t.Error("expected DELETE /resource_controls/17")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after delete, got %q", d.Id())
	}
}

func TestResourceControl_ExternalStackWithoutContainersIsKept(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/4/docker/containers/json", RespondJSON(http.StatusOK, []interface{}{}))

	r := resourceResourceControl()
	d := r.TestResourceData()
	d.SetId("17")
	_ = d.Set("resource_id", "4_server")
	_ = d.Set("type", 6)
	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "17" {
		t.Errorf("a project without containers cannot be read and must be kept, got ID %q", d.Id())
	}
}

func TestResourceControl_DockerTypesReadFromProxy(t *testing.T) {
	cases := []struct {
		name    string
		rc      map[string]interface{}
		wantID  string
		private bool
	}{
		{"own control", map[string]interface{}{"Id": 21, "Type": 1, "ResourceId": "abc", "AdministratorsOnly": true}, "21", true},
		{"only an inherited stack control", map[string]interface{}{"Id": 5, "Type": 6, "ResourceId": "3_app"}, "", false},
		{"no control", nil, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockServer(t)
			body := map[string]interface{}{"Id": "abc"}
			if tc.rc != nil {
				body["Portainer"] = map[string]interface{}{"ResourceControl": tc.rc}
			}
			mock.On("GET", "/endpoints/3/docker/containers/abc/json", RespondJSON(http.StatusOK, body))

			r := resourceResourceControl()
			d := r.TestResourceData()
			d.SetId("21")
			_ = d.Set("resource_id", "abc")
			_ = d.Set("type", 1)
			_ = d.Set("endpoint_id", 3)
			if err := rcRead(r, d, mock.Client()); err != nil {
				t.Fatalf("Read failed: %v", err)
			}
			if d.Id() != tc.wantID {
				t.Errorf("ID: expected %q, got %q", tc.wantID, d.Id())
			}
			if d.Get("administrators_only").(bool) != tc.private {
				t.Errorf("administrators_only: expected %v", tc.private)
			}
		})
	}
}

func TestResourceControl_DockerTypeWithoutEndpointKeepsState(t *testing.T) {
	r := resourceResourceControl()
	d := r.TestResourceData()
	d.SetId("21")
	_ = d.Set("resource_id", "abc")
	_ = d.Set("type", 1)
	if err := rcRead(r, d, NewMockServer(t).Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "21" {
		t.Errorf("without endpoint_id the control is tracked as configured, got ID %q", d.Id())
	}
}

// TestResourceControl_ExternalStackShadowedOrGone: a container with a control
// of its own hides the stack's, which must not be taken for a deleted stack
// control; a container showing no control at all proves it is gone.
func TestResourceControl_ExternalStackShadowedOrGone(t *testing.T) {
	for name, tc := range map[string]struct {
		container map[string]interface{}
		wantID    string
	}{
		"shadowed by the container's own control": {map[string]interface{}{"Id": "c1", "Portainer": map[string]interface{}{"ResourceControl": map[string]interface{}{"Id": 20, "Type": 1, "ResourceId": "c1"}}}, "17"},
		"container shows no control":              {map[string]interface{}{"Id": "c1"}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("GET", "/endpoints/4/docker/containers/json", RespondJSON(http.StatusOK, []interface{}{tc.container}))
			r := resourceResourceControl()
			d := r.TestResourceData()
			d.SetId("17")
			_ = d.Set("resource_id", "4_server")
			_ = d.Set("type", 6)
			if err := rcRead(r, d, mock.Client()); err != nil {
				t.Fatalf("Read failed: %v", err)
			}
			if d.Id() != tc.wantID {
				t.Errorf("ID: expected %q, got %q", tc.wantID, d.Id())
			}
		})
	}
}

// TestResourceControl_DockerCanonicalIDs is the regression test for controls
// keyed differently from resource_id: a volume's control is keyed
// "<name>_<docker ID>" and a container's by its full ID, so a resource_id
// with the volume name, Portainer's volume key or a container name never
// matched and the resource dropped out of state on every refresh.
func TestResourceControl_DockerCanonicalIDs(t *testing.T) {
	cases := []struct {
		name, resourceID, path string
		resourceType           int
		object                 map[string]interface{}
		rcResource             string
	}{
		{
			"volume by name", "data", "/endpoints/3/docker/volumes/data", 3,
			map[string]interface{}{"Name": "data", "ResourceID": "data_dockerid"},
			"data_dockerid",
		},
		{
			"volume by Portainer's key", "data_dockerid", "/endpoints/3/docker/volumes/data", 3,
			map[string]interface{}{"Name": "data", "ResourceID": "data_dockerid"},
			"data_dockerid",
		},
		{
			"container by name", "web", "/endpoints/3/docker/containers/web/json", 1,
			map[string]interface{}{"Id": "abcdef0123", "Name": "/web"},
			"abcdef0123",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockServer(t)
			tc.object["Portainer"] = map[string]interface{}{"ResourceControl": map[string]interface{}{
				"Id": 21, "Type": tc.resourceType, "ResourceId": tc.rcResource,
			}}
			mock.On("GET", tc.path, RespondJSON(http.StatusOK, tc.object))

			r := resourceResourceControl()
			d := r.TestResourceData()
			d.SetId("21")
			_ = d.Set("resource_id", tc.resourceID)
			_ = d.Set("type", tc.resourceType)
			_ = d.Set("endpoint_id", 3)
			if err := rcRead(r, d, mock.Client()); err != nil {
				t.Fatalf("Read failed: %v", err)
			}
			if d.Id() != "21" {
				t.Errorf("ID: expected the control to be found, got %q", d.Id())
			}
		})
	}
}
