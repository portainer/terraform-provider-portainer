package internal

import (
	"net/http"
	"strings"
	"testing"
)

func gatewayListing() []map[string]interface{} {
	return []map[string]interface{}{
		{"Name": "other", "Namespace": "web", "ClassName": "eg"},
		{
			"Name": "edge", "Namespace": "web", "ClassName": "eg",
			"Listeners": []map[string]interface{}{
				{"Name": "http", "Port": 80, "Protocol": "HTTP", "Hostname": "*.example.com"},
			},
			"Addresses":       []map[string]interface{}{{"Type": "IPAddress", "Value": "10.0.0.5"}},
			"Labels":          map[string]string{"team": "web", "io.portainer.kubernetes.gateway.owner": "admin"},
			"ResourceVersion": "42", "UID": "uid-1", "CreationDate": "2026-11-01T00:00:00Z",
		},
	}
}

// TestKubernetesGatewayCreate_PostsAndReadsBack verifies the PascalCase
// payload, the "<env>:<ns>:<name>" ID and the read-back by listing.
func TestKubernetesGatewayCreate_PostsAndReadsBack(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/1/namespaces/web/gateways", RespondString(http.StatusNoContent, "", ""))
	mock.On("GET", "/kubernetes/1/namespaces/web/gateways", RespondJSON(http.StatusOK, gatewayListing()))

	r := resourceKubernetesGateway()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "web")
	_ = d.Set("name", "edge")
	_ = d.Set("class_name", "eg")
	_ = d.Set("listener", []interface{}{map[string]interface{}{
		"name": "http", "port": 80, "protocol": "HTTP", "hostname": "*.example.com",
	}})
	_ = d.Set("labels", map[string]interface{}{"team": "web"})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "1:web:edge" {
		t.Errorf("ID: got %q", d.Id())
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("POST", "/kubernetes/1/namespaces/web/gateways").DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["Name"] != "edge" || payload["Namespace"] != "web" || payload["ClassName"] != "eg" {
		t.Errorf("payload: %v", payload)
	}
	if _, ok := payload["ResourceVersion"]; ok {
		t.Error("a create must not send a resource version")
	}
	listeners := payload["Listeners"].([]interface{})
	if l := listeners[0].(map[string]interface{}); l["Port"] != float64(80) || l["Protocol"] != "HTTP" {
		t.Errorf("listener payload: %v", l)
	}

	if got := d.Get("resource_version"); got != "42" {
		t.Errorf("resource_version: got %v", got)
	}
	if got := d.Get("address.0.value"); got != "10.0.0.5" {
		t.Errorf("address.0.value: got %v", got)
	}
	if got := d.Get("uid"); got != "uid-1" {
		t.Errorf("uid: got %v", got)
	}
}

// TestKubernetesGatewayCreate_ExplainsOldServer verifies a 404 from a server
// without the Gateway API names the requirement.
func TestKubernetesGatewayCreate_ExplainsOldServer(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "2.45.3", "ServerEdition": "BE",
	}))

	r := resourceKubernetesGateway()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "web")
	_ = d.Set("name", "edge")
	_ = d.Set("class_name", "eg")

	err := rcCreate(r, d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "requires Portainer Business Edition 3.0 or later") {
		t.Fatalf("expected the requirement to be named, got %v", err)
	}
}

// TestKubernetesGatewayRead_DropsMissing verifies a gateway missing from the
// namespace listing is dropped from state.
func TestKubernetesGatewayRead_DropsMissing(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/1/namespaces/web/gateways", RespondJSON(http.StatusOK, gatewayListing()))

	r := resourceKubernetesGateway()
	d := r.TestResourceData()
	d.SetId("1:web:gone")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}
}

// TestKubernetesGatewayRead_Import verifies a read from the ID alone fills
// every argument, which is what an import relies on.
func TestKubernetesGatewayRead_Import(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/3/namespaces/web/gateways", RespondJSON(http.StatusOK, gatewayListing()))

	r := resourceKubernetesGateway()
	d := r.TestResourceData()
	d.SetId("3:web:edge")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Get("environment_id") != 3 || d.Get("namespace") != "web" || d.Get("name") != "edge" || d.Get("class_name") != "eg" {
		t.Errorf("identity not filled: %v %v %v %v", d.Get("environment_id"), d.Get("namespace"), d.Get("name"), d.Get("class_name"))
	}
	if got := d.Get("listener.0.hostname"); got != "*.example.com" {
		t.Errorf("listener.0.hostname: got %v", got)
	}
	// Portainer's own owner label is not configuration (seen live on 3.0.1,
	// where it showed as drift on every plan).
	if got := d.Get("labels").(map[string]interface{}); len(got) != 1 || got["team"] != "web" {
		t.Errorf("labels: expected only team=web, got %v", got)
	}
}

func TestKubernetesGatewayRead_InvalidID(t *testing.T) {
	r := resourceKubernetesGateway()
	d := r.TestResourceData()
	d.SetId("web:edge")
	if err := rcRead(r, d, NewMockServer(t).Client()); err == nil {
		t.Fatal("expected an error for a malformed ID")
	}
}

// TestKubernetesGatewayUpdate_SendsResourceVersion verifies PUT carries the
// resource version from state, so a concurrent change is rejected.
func TestKubernetesGatewayUpdate_SendsResourceVersion(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/kubernetes/1/namespaces/web/gateways", RespondString(http.StatusNoContent, "", ""))
	mock.On("GET", "/kubernetes/1/namespaces/web/gateways", RespondJSON(http.StatusOK, gatewayListing()))

	r := resourceKubernetesGateway()
	d := r.TestResourceData()
	d.SetId("1:web:edge")
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "web")
	_ = d.Set("name", "edge")
	_ = d.Set("class_name", "eg")
	_ = d.Set("resource_version", "41")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	var payload map[string]interface{}
	if err := mock.FindRequest("PUT", "/kubernetes/1/namespaces/web/gateways").DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["ResourceVersion"] != "41" {
		t.Errorf("payload.ResourceVersion: got %v", payload["ResourceVersion"])
	}
}

func TestKubernetesGatewayUpdate_HTTPError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/kubernetes/1/namespaces/web/gateways", RespondString(http.StatusConflict, "", "stale"))

	r := resourceKubernetesGateway()
	d := r.TestResourceData()
	d.SetId("1:web:edge")
	if err := rcUpdate(r, d, mock.Client()); err == nil {
		t.Fatal("expected an error on 409")
	}
}

// TestKubernetesGatewayDelete_BulkDeleteSingleItem verifies the bulk delete
// receives the one gateway, keyed by namespace, and a 404 is not an error.
func TestKubernetesGatewayDelete_BulkDeleteSingleItem(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/1/gateways/delete", RespondString(http.StatusNoContent, "", ""))

	r := resourceKubernetesGateway()
	d := r.TestResourceData()
	d.SetId("1:web:edge")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	var payload map[string][]string
	if err := mock.FindRequest("POST", "/kubernetes/1/gateways/delete").DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if names := payload["web"]; len(names) != 1 || names[0] != "edge" || len(payload) != 1 {
		t.Errorf("payload: expected {web: [edge]}, got %v", payload)
	}

	gone := NewMockServer(t)
	d.SetId("1:web:edge")
	gone.On("POST", "/kubernetes/1/gateways/delete", RespondString(http.StatusNotFound, "", "not found"))
	if err := rcDelete(r, d, gone.Client()); err != nil {
		t.Errorf("a 404 on delete should not fail, got %v", err)
	}
}

func TestKubernetesGatewayRead_ServerError(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/1/namespaces/web/gateways", RespondString(http.StatusInternalServerError, "", "boom"))

	r := resourceKubernetesGateway()
	d := r.TestResourceData()
	d.SetId("1:web:edge")
	if err := rcRead(r, d, mock.Client()); err == nil {
		t.Fatal("expected an error on 500")
	}

	notFound := NewMockServer(t)
	notFound.On("GET", "/kubernetes/1/namespaces/web/gateways", RespondString(http.StatusNotFound, "", "no env"))
	if err := rcRead(r, d, notFound.Client()); err != nil || d.Id() != "" {
		t.Errorf("a 404 listing should drop the gateway, got err=%v id=%q", err, d.Id())
	}
}

// TestDataSourceKubernetesGatewayClasses verifies the PascalCase payload.
func TestDataSourceKubernetesGatewayClasses(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/1/gatewayclasses", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"Name": "eg", "ControllerName": "gateway.envoyproxy.io/gatewayclass-controller", "UID": "u1",
			"Labels": map[string]string{"a": "b"},
		},
	}))

	ds := dataSourceKubernetesGatewayClasses()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 1)
	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("gateway_classes.0.controller_name"); got != "gateway.envoyproxy.io/gatewayclass-controller" {
		t.Errorf("controller_name: got %v", got)
	}
	if got := d.Get("gateway_classes.0.labels.a"); got != "b" {
		t.Errorf("labels: got %v", got)
	}
	if d.Id() != "1/gatewayclasses" {
		t.Errorf("id: got %q", d.Id())
	}
}

func TestDataSourceKubernetesGatewayClasses_ExplainsOldServer(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "2.45.3", "ServerEdition": "CE",
	}))
	ds := dataSourceKubernetesGatewayClasses()
	d := ds.TestResourceData()
	_ = d.Set("environment_id", 1)
	err := rcRead(ds, d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "portainer_kubernetes_gateway_classes requires") {
		t.Fatalf("expected the requirement to be named, got %v", err)
	}
}
