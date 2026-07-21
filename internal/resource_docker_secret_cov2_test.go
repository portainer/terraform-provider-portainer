package internal

import (
	"net/http"
	"testing"
)

// =========================================================================
// cov2 coverage for resource_docker_secret.go: the Driver flatten branch in
// Read (the base suite only exercises the Templating flatten), plus a Create
// that returns HTTP 201 Created with labels and no resource control (the
// resource_control_id==0 skip branch).
// =========================================================================

// TestDockerSecretCov2_Read_DriverFlatten covers the Driver flatten branch in
// Read: Portainer returns the driver as {Name, Options:{...}} and Read must
// flatten Options back into the flat TypeMap schema (mirroring the Templating
// round-trip already covered elsewhere).
func TestDockerSecretCov2_Read_DriverFlatten(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/secrets/sec_d", RespondJSON(http.StatusOK, map[string]interface{}{
		"ID": "sec_d",
		"Spec": map[string]interface{}{
			"Name": "driven-secret",
			"Driver": map[string]interface{}{
				"Name": "vault-driver",
				"Options": map[string]interface{}{
					"name":     "vault-driver",
					"endpoint": "https://vault.example",
				},
			},
		},
		"Portainer": map[string]interface{}{
			"ResourceControl": map[string]interface{}{"Id": 21},
		},
	}))

	r := resourceDockerSecret()
	d := r.TestResourceData()
	d.SetId("sec_d")
	_ = d.Set("endpoint_id", 1)

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("name"); got != "driven-secret" {
		t.Errorf("name: expected %q, got %v", "driven-secret", got)
	}
	driver := d.Get("driver").(map[string]interface{})
	if driver["name"] != "vault-driver" {
		t.Errorf("driver.name: expected %q, got %v", "vault-driver", driver["name"])
	}
	if driver["endpoint"] != "https://vault.example" {
		t.Errorf("driver flatten failed, got %v", driver)
	}
	if got := d.Get("resource_control_id"); got != 21 {
		t.Errorf("resource_control_id: expected 21, got %v", got)
	}
}

// TestDockerSecretCov2_Create_Status201WithLabels covers Create accepting an
// HTTP 201 Created response (in addition to 200), carrying labels in the
// payload, and the resource_control_id==0 skip branch (no ResourceControl in
// the response).
func TestDockerSecretCov2_Create_Status201WithLabels(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/2/docker/secrets", RespondJSON(http.StatusOK, []map[string]interface{}{}))
	mock.On("POST", "/endpoints/2/docker/secrets/create", RespondJSON(http.StatusCreated, map[string]interface{}{
		"ID": "sec_created",
		// No Portainer.ResourceControl → resource_control_id stays unset.
	}))

	r := resourceDockerSecret()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 2)
	_ = d.Set("name", "labeled")
	_ = d.Set("data", "ZGF0YQ==")
	_ = d.Set("labels", map[string]interface{}{"env": "prod", "team": "core"})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "sec_created" {
		t.Errorf("expected ID %q, got %q", "sec_created", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/2/docker/secrets/create")
	if post == nil {
		t.Fatal("expected POST create")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
	labels, ok := payload["Labels"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected Labels map in payload, got %v", payload["Labels"])
	}
	if labels["env"] != "prod" || labels["team"] != "core" {
		t.Errorf("payload.Labels mismatch, got %v", labels)
	}
}
