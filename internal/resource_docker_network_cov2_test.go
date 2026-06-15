package internal

import (
	"net/http"
	"testing"
)

// TestDockerNetworkCov2_Create_ConfigOnly covers the config_only branch: the
// per-network flags (internal/attachable/etc.) and scope are intentionally NOT
// added to the payload, while config_from is serialized into ConfigFrom.
func TestDockerNetworkCov2_Create_ConfigOnly(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/networks/create", RespondJSON(http.StatusCreated, map[string]interface{}{
		"Id": "cfgnet",
	}))

	r := resourceDockerNetwork()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "cfgonly")
	_ = d.Set("driver", "bridge")
	_ = d.Set("config_only", true)
	_ = d.Set("config_from", "basenet")
	// These should be ignored because config_only is true.
	_ = d.Set("internal", true)
	_ = d.Set("attachable", true)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "cfgnet" {
		t.Errorf("unexpected ID %q", d.Id())
	}

	post := mock.FindRequest("POST", "/endpoints/1/docker/networks/create")
	if post == nil {
		t.Fatal("expected POST recorded")
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if got := payload["ConfigOnly"]; got != true {
		t.Errorf("ConfigOnly: expected true, got %v", got)
	}
	if _, present := payload["Internal"]; present {
		t.Error("Internal must not be present when config_only is true")
	}
	if _, present := payload["Scope"]; present {
		t.Error("Scope must not be present when config_only is true")
	}
	cf, ok := payload["ConfigFrom"].(map[string]interface{})
	if !ok {
		t.Fatalf("ConfigFrom: expected object, got %T", payload["ConfigFrom"])
	}
	if cf["Network"] != "basenet" {
		t.Errorf("ConfigFrom.Network: expected basenet, got %v", cf["Network"])
	}
}

// TestDockerNetworkCov2_Create_OptionsLabelsAndSwarmHeader covers the
// options/labels/ipam_options payload branches and the X-PortainerAgent-Target
// header set from swarm_node_id.
func TestDockerNetworkCov2_Create_OptionsLabelsAndSwarmHeader(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("POST", "/endpoints/1/docker/networks/create", RespondJSON(http.StatusCreated, map[string]interface{}{
		"Id": "optnet",
		"Portainer": map[string]interface{}{
			"ResourceControl": map[string]interface{}{"Id": 99},
		},
	}))

	r := resourceDockerNetwork()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "optnet")
	_ = d.Set("driver", "macvlan")
	_ = d.Set("internal", true)
	_ = d.Set("enable_ipv6", true)
	_ = d.Set("options", map[string]interface{}{"parent": "eth0"})
	_ = d.Set("labels", map[string]interface{}{"env": "prod"})
	_ = d.Set("ipam_driver", "default")
	_ = d.Set("ipam_options", map[string]interface{}{"foo": "bar"})
	_ = d.Set("swarm_node_id", "node-xyz")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "optnet" {
		t.Errorf("unexpected ID %q", d.Id())
	}
	// Resource control id from response should be captured.
	if got := d.Get("resource_control_id"); got != 99 {
		t.Errorf("resource_control_id: expected 99, got %v", got)
	}

	post := mock.FindRequest("POST", "/endpoints/1/docker/networks/create")
	if post == nil {
		t.Fatal("expected POST recorded")
	}
	if got := post.Headers.Get("X-PortainerAgent-Target"); got != "node-xyz" {
		t.Errorf("X-PortainerAgent-Target: expected node-xyz, got %q", got)
	}
	var payload map[string]interface{}
	if err := post.DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if opts, ok := payload["Options"].(map[string]interface{}); !ok || opts["parent"] != "eth0" {
		t.Errorf("Options: expected parent=eth0, got %v", payload["Options"])
	}
	if labels, ok := payload["Labels"].(map[string]interface{}); !ok || labels["env"] != "prod" {
		t.Errorf("Labels: expected env=prod, got %v", payload["Labels"])
	}
	if payload["Internal"] != true {
		t.Errorf("Internal: expected true, got %v", payload["Internal"])
	}
	if payload["EnableIPv6"] != true {
		t.Errorf("EnableIPv6: expected true, got %v", payload["EnableIPv6"])
	}
	ipam, ok := payload["IPAM"].(map[string]interface{})
	if !ok {
		t.Fatalf("IPAM: expected object, got %T", payload["IPAM"])
	}
	if o, ok := ipam["Options"].(map[string]interface{}); !ok || o["foo"] != "bar" {
		t.Errorf("IPAM.Options: expected foo=bar, got %v", ipam["Options"])
	}
}

// TestDockerNetworkCov2_Read_DecodeError verifies that a 200 body that is not
// valid JSON surfaces a decode error.
func TestDockerNetworkCov2_Read_DecodeError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/networks/abc123", RespondString(
		http.StatusOK, "application/json", `{ this is not json`,
	))

	r := resourceDockerNetwork()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	d.SetId("abc123")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected decode error on malformed JSON, got nil")
	}
}

// TestDockerNetworkCov2_Read_Non200Errors verifies that a non-404 error status
// on inspect surfaces an error (rather than drift-clearing the ID).
func TestDockerNetworkCov2_Read_Non200Errors(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/networks/abc123", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDockerNetwork()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	d.SetId("abc123")

	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestDockerNetworkCov2_Read_ConfigOnly verifies the config_only read branch:
// driver/scope are preserved from config (not the API response) and the
// per-network flags are echoed from prior state.
func TestDockerNetworkCov2_Read_ConfigOnly(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/networks/cfgnet", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id":         "cfgnet",
		"Name":       "cfgonly",
		"ConfigOnly": true,
		"Driver":     "",
		"Scope":      "",
		"IPAM":       map[string]interface{}{"Driver": "default"},
	}))

	r := resourceDockerNetwork()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("driver", "bridge")
	_ = d.Set("scope", "local")
	d.SetId("cfgnet")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("config_only"); got != true {
		t.Errorf("config_only: expected true, got %v", got)
	}
	// driver/scope come from config, not the (empty) API values.
	if got := d.Get("driver"); got != "bridge" {
		t.Errorf("driver: expected bridge (from config), got %v", got)
	}
	if got := d.Get("scope"); got != "local" {
		t.Errorf("scope: expected local (from config), got %v", got)
	}
}

// TestDockerNetworkCov2_FindFallback_MultiMatchReturnsNil verifies the fallback
// helper returns (nil, nil) when more than one network matches the filter (the
// resource then clears its ID upstream).
func TestDockerNetworkCov2_FindFallback_MultiMatchReturnsNil(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/networks", RespondJSON(http.StatusOK, []map[string]interface{}{
		{"Id": "a", "Name": "mynet"},
		{"Id": "b", "Name": "mynet"},
	}))

	r := resourceDockerNetwork()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "mynet")
	_ = d.Set("driver", "bridge")
	_ = d.Set("scope", "local")

	net, err := findDockerNetworkFallback(d, mock.Client(), 1, map[string]string{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if net != nil {
		t.Errorf("expected nil network for multi-match, got %+v", net)
	}
}

// TestDockerNetworkCov2_FindFallback_ListErrors verifies the fallback helper
// surfaces an error when the list endpoint returns a non-200.
func TestDockerNetworkCov2_FindFallback_ListErrors(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("GET", "/endpoints/1/docker/networks", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDockerNetwork()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	_ = d.Set("name", "mynet")
	_ = d.Set("driver", "bridge")
	_ = d.Set("scope", "local")

	if _, err := findDockerNetworkFallback(d, mock.Client(), 1, map[string]string{}); err == nil {
		t.Fatal("expected error from failing list endpoint, got nil")
	}
}

// TestDockerNetworkCov2_Delete_HTTPError verifies a non-2xx/non-404 delete
// surfaces an error.
func TestDockerNetworkCov2_Delete_HTTPError(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/docker/networks/abc123", RespondString(
		http.StatusInternalServerError, "application/json", `{"message":"boom"}`,
	))

	r := resourceDockerNetwork()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	d.SetId("abc123")

	if err := rcDelete(r, d, mock.Client()); err == nil {
		t.Fatal("expected error on HTTP 500, got nil")
	}
}

// TestDockerNetworkCov2_Delete_404IsSuccess verifies a 404 on delete is treated
// as success.
func TestDockerNetworkCov2_Delete_404IsSuccess(t *testing.T) {
	mock := NewMockServer(t)

	mock.On("DELETE", "/endpoints/1/docker/networks/abc123", RespondString(
		http.StatusNotFound, "application/json", `{"message":"gone"}`,
	))

	r := resourceDockerNetwork()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 1)
	d.SetId("abc123")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete should swallow 404, got error: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}
