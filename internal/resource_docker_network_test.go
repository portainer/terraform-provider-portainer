package internal

import (
	"net/http"
	"testing"
)

// TestDockerNetworkCreate_HappyPath_Bridge exercises the default bridge-network
// create path: it verifies the POST hits the docker networks/create endpoint,
// that the payload uses Docker's PascalCase field names, and that the returned
// network ID (a string hash, not an int) is stored as the resource ID.
func TestDockerNetworkCreate_HappyPath_Bridge(t *testing.T) {
	mock := NewMockServer(t)

	// Docker API returns 201 Created on network creation.
	mock.On("POST", "/endpoints/1/docker/networks/create", RespondJSON(http.StatusCreated, map[string]interface{}{
		"Id":      "abc123",
		"Warning": "",
	}))

	r := resourceDockerNetwork()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "mynet")
	_ = d.Set("driver", "bridge")
	_ = d.Set("scope", "local")
	_ = d.Set("ipam_driver", "default")

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	if d.Id() != "abc123" {
		t.Errorf("expected ID %q, got %q", "abc123", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/1/docker/networks/create")
	if post == nil {
		t.Fatal("expected POST to /endpoints/1/docker/networks/create")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode POST body: %v", err)
	}

	if got := payload["Name"]; got != "mynet" {
		t.Errorf("payload.Name: expected %q, got %v", "mynet", got)
	}
	if got := payload["Driver"]; got != "bridge" {
		t.Errorf("payload.Driver: expected %q, got %v", "bridge", got)
	}
	// ConfigOnly is always present in the payload (set unconditionally).
	if got := payload["ConfigOnly"]; got != false {
		t.Errorf("payload.ConfigOnly: expected false, got %v", got)
	}
	if got := payload["Scope"]; got != "local" {
		t.Errorf("payload.Scope: expected %q, got %v", "local", got)
	}
	// IPAM is always present with at least its Driver.
	ipam, ok := payload["IPAM"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload.IPAM: expected object, got %T (%v)", payload["IPAM"], payload["IPAM"])
	}
	if got := ipam["Driver"]; got != "default" {
		t.Errorf("payload.IPAM.Driver: expected %q, got %v", "default", got)
	}
}

// TestDockerNetworkCreate_WithIPAM verifies that an ipam_config block is
// serialized into the nested IPAM.Config array using PascalCase keys
// (Subnet, Gateway, IPRange).
func TestDockerNetworkCreate_WithIPAM(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/networks/create", RespondJSON(http.StatusCreated, map[string]interface{}{
		"Id": "ipamnet1",
	}))

	r := resourceDockerNetwork()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "ipamnet")
	_ = d.Set("driver", "bridge")
	_ = d.Set("ipam_driver", "default")
	_ = d.Set("ipam_config", []interface{}{
		map[string]interface{}{
			"subnet":   "172.20.0.0/16",
			"gateway":  "172.20.0.1",
			"ip_range": "172.20.10.0/24",
		},
	})

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "ipamnet1" {
		t.Errorf("expected ID %q, got %q", "ipamnet1", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/1/docker/networks/create")
	if post == nil {
		t.Fatal("expected POST to /endpoints/1/docker/networks/create")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode POST body: %v", err)
	}

	ipam, ok := payload["IPAM"].(map[string]interface{})
	if !ok {
		t.Fatalf("payload.IPAM: expected object, got %T", payload["IPAM"])
	}
	configs, ok := ipam["Config"].([]interface{})
	if !ok {
		t.Fatalf("payload.IPAM.Config: expected array, got %T (%v)", ipam["Config"], ipam["Config"])
	}
	if len(configs) != 1 {
		t.Fatalf("payload.IPAM.Config: expected 1 entry, got %d", len(configs))
	}
	cfg, ok := configs[0].(map[string]interface{})
	if !ok {
		t.Fatalf("payload.IPAM.Config[0]: expected object, got %T", configs[0])
	}
	if got := cfg["Subnet"]; got != "172.20.0.0/16" {
		t.Errorf("IPAM.Config[0].Subnet: expected %q, got %v", "172.20.0.0/16", got)
	}
	if got := cfg["Gateway"]; got != "172.20.0.1" {
		t.Errorf("IPAM.Config[0].Gateway: expected %q, got %v", "172.20.0.1", got)
	}
	if got := cfg["IPRange"]; got != "172.20.10.0/24" {
		t.Errorf("IPAM.Config[0].IPRange: expected %q, got %v", "172.20.10.0/24", got)
	}
}

// TestDockerNetworkCreate_Overlay verifies that an overlay network with the
// attachable flag set is serialized correctly (Driver=overlay, Attachable=true).
func TestDockerNetworkCreate_Overlay(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/networks/create", RespondJSON(http.StatusCreated, map[string]interface{}{
		"Id": "overlay-xyz",
	}))

	r := resourceDockerNetwork()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "myoverlay")
	_ = d.Set("driver", "overlay")
	_ = d.Set("scope", "swarm")
	_ = d.Set("attachable", true)

	if err := r.Create(d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "overlay-xyz" {
		t.Errorf("expected ID %q, got %q", "overlay-xyz", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/1/docker/networks/create")
	if post == nil {
		t.Fatal("expected POST to /endpoints/1/docker/networks/create")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("failed to decode POST body: %v", err)
	}

	if got := payload["Driver"]; got != "overlay" {
		t.Errorf("payload.Driver: expected %q, got %v", "overlay", got)
	}
	if got := payload["Attachable"]; got != true {
		t.Errorf("payload.Attachable: expected true, got %v", got)
	}
	if got := payload["Scope"]; got != "swarm" {
		t.Errorf("payload.Scope: expected %q, got %v", "swarm", got)
	}
}

// TestDockerNetworkCreate_HTTPError verifies that a non-2xx create response is
// surfaced as an error and the resource ID is left empty.
func TestDockerNetworkCreate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/networks/create", RespondString(
		http.StatusBadRequest, "application/json",
		`{"message":"network with name mynet already exists"}`,
	))

	r := resourceDockerNetwork()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "mynet")
	_ = d.Set("driver", "bridge")

	err := r.Create(d, mock.Client())
	if err == nil {
		t.Fatal("expected error on HTTP 400, got nil")
	}
	if d.Id() != "" {
		t.Errorf("expected empty ID after error, got %q", d.Id())
	}
}

// TestDockerNetworkRead_HappyPath verifies that an inspect (GET by ID) maps the
// scalar Docker network fields (name, driver, scope, ipam_driver) into resource
// state. The IPAM Config payload is intentionally not asserted here: the
// resource sets ipam_config directly from the API's PascalCase keys, which the
// SDK schema (snake_case) does not adopt — asserting it would test SDK quirks,
// not the resource contract.
func TestDockerNetworkRead_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/networks/abc123", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":     "abc123",
		"Name":   "mynet",
		"Driver": "bridge",
		"Scope":  "local",
		"IPAM": map[string]interface{}{
			"Driver": "default",
			"Config": []interface{}{
				map[string]interface{}{"Subnet": "172.20.0.0/16", "Gateway": "172.20.0.1"},
			},
		},
	}))

	r := resourceDockerNetwork()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	d.SetId("abc123")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("name"); got != "mynet" {
		t.Errorf("name: expected %q, got %v", "mynet", got)
	}
	if got := d.Get("driver"); got != "bridge" {
		t.Errorf("driver: expected %q, got %v", "bridge", got)
	}
	if got := d.Get("scope"); got != "local" {
		t.Errorf("scope: expected %q, got %v", "local", got)
	}
	if got := d.Get("ipam_driver"); got != "default" {
		t.Errorf("ipam_driver: expected %q, got %v", "default", got)
	}
}

// TestDockerNetworkRead_404_ClearsID verifies that when the network is gone
// (404 on inspect) and the name/driver/scope fallback list returns nothing,
// the resource clears its ID (standard drift detection).
//
// The fallback issues a GET to the bare networks list path with a ?filters=
// query. The mock matches on path only, so an empty list there yields no
// match and the resource removes itself from state.
func TestDockerNetworkRead_404_ClearsID(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/networks/gone123", RespondString(
		http.StatusNotFound, "application/json",
		`{"message":"network gone123 not found"}`,
	))
	// Fallback list lookup returns no networks -> resource clears its ID.
	mock.On("GET", "/endpoints/1/docker/networks", RespondJSON(http.StatusOK, []map[string]interface{}{}))

	r := resourceDockerNetwork()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "gone")
	_ = d.Set("driver", "bridge")
	_ = d.Set("scope", "local")
	d.SetId("gone123")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read should swallow 404 (with empty fallback) and clear ID, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after 404, got %q", d.Id())
	}
}

// TestDockerNetworkRead_404_FallbackResolves verifies the fallback path: when
// inspect returns 404 but the filtered list returns exactly one network, the
// resource adopts that network's ID and re-reads it successfully.
func TestDockerNetworkRead_404_FallbackResolves(t *testing.T) {
	mock := NewMockServer(t)

	// First inspect (by the stale ID) 404s.
	mock.On("GET", "/endpoints/1/docker/networks/stale", RespondString(
		http.StatusNotFound, "application/json", `{"message":"not found"}`,
	))
	// Fallback list returns exactly one match -> ID is adopted.
	mock.On("GET", "/endpoints/1/docker/networks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": "real-id", "Name": "mynet", "Driver": "bridge", "Scope": "local"},
	}))
	// Re-read with the resolved ID succeeds.
	mock.On("GET", "/endpoints/1/docker/networks/real-id", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":     "real-id",
		"Name":   "mynet",
		"Driver": "bridge",
		"Scope":  "local",
		"IPAM":   map[string]interface{}{"Driver": "default"},
	}))

	r := resourceDockerNetwork()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "mynet")
	_ = d.Set("driver", "bridge")
	_ = d.Set("scope", "local")
	d.SetId("stale")

	if err := r.Read(d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "real-id" {
		t.Errorf("expected fallback to resolve ID to %q, got %q", "real-id", d.Id())
	}
	if got := d.Get("name"); got != "mynet" {
		t.Errorf("name: expected %q, got %v", "mynet", got)
	}
}

// TestDockerNetworkDelete_HappyPath verifies the DELETE is sent to the
// per-network endpoint and the ID is cleared. Docker returns 204 No Content.
func TestDockerNetworkDelete_HappyPath(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/docker/networks/abc123", RespondString(http.StatusNoContent, "", ""))

	r := resourceDockerNetwork()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	d.SetId("abc123")

	if err := r.Delete(d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}

	if mock.FindRequest("DELETE", "/endpoints/1/docker/networks/abc123") == nil {
		t.Error("expected DELETE /endpoints/1/docker/networks/abc123 to be sent")
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared after delete, got %q", d.Id())
	}
}

// TestDockerNetworkImport_ParsesCompositeID verifies the importer splits the
// "<endpoint_id>:<network_id>" composite ID into the endpoint_id attribute and
// the bare (string hash) network ID. This guards against treating the docker
// network ID as an int.
func TestDockerNetworkImport_ParsesCompositeID(t *testing.T) {
	r := resourceDockerNetwork()
	d := r.TestResourceData()
	d.SetId("1:abc123")

	results, err := r.Importer.State(d, nil)
	if err != nil {
		t.Fatalf("Import failed: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 imported state, got %d", len(results))
	}
	imported := results[0]
	if imported.Id() != "abc123" {
		t.Errorf("expected network ID %q, got %q", "abc123", imported.Id())
	}
	if got := imported.Get("endpoint_id"); got != 1 {
		t.Errorf("endpoint_id: expected 1, got %v", got)
	}
}

// TestDockerNetworkImport_BadID rejects an ID that is not in composite form.
func TestDockerNetworkImport_BadID(t *testing.T) {
	r := resourceDockerNetwork()
	d := r.TestResourceData()
	d.SetId("not-composite")

	if _, err := r.Importer.State(d, nil); err == nil {
		t.Fatal("expected error for non-composite import ID, got nil")
	}
}

// TestDockerNetworkResource_NoUpdate documents and enforces the design decision
// that a docker network is immutable: every attribute is ForceNew and there is
// no Update function.
func TestDockerNetworkResource_NoUpdate(t *testing.T) {
	r := resourceDockerNetwork()
	if r.Update != nil {
		t.Error("expected no Update function (network is ForceNew/immutable)")
	}
}
