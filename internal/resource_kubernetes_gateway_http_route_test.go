package internal

import (
	"net/http"
	"strings"
	"testing"
)

func httpRouteListing() []map[string]interface{} {
	return []map[string]interface{}{{
		"Name": "app", "Namespace": "web", "Hostnames": []string{"app.example.com"},
		"ParentRefs": []map[string]interface{}{
			{"Name": "edge", "SectionName": "http", "Group": "gateway.networking.k8s.io"},
		},
		"Rules": []map[string]interface{}{{
			"Matches": []map[string]interface{}{{
				"Path":    map[string]interface{}{"Type": "PathPrefix", "Value": "/api"},
				"Headers": []map[string]interface{}{{"Name": "x-env", "Type": "Exact", "Value": "prod"}},
			}},
			"Filters": []map[string]interface{}{{
				"Type": "RequestHeaderModifier",
				"RequestHeaderModifier": map[string]interface{}{
					"Set": []map[string]interface{}{{"Name": "x-from", "Value": "gw"}}, "Remove": []string{"x-debug"},
				},
			}},
			"BackendRefs": []map[string]interface{}{{
				"BackendRef": map[string]interface{}{
					"BackendObjectReference": map[string]interface{}{"Name": "api", "Port": 8080, "Kind": "Service"},
					"Weight":                 1,
				},
			}},
		}},
		"ResourceVersion": "7",
	}}
}

// TestKubernetesGatewayHTTPRouteCreate verifies the nested payload, the ID and
// the read-back of the server-side defaults.
func TestKubernetesGatewayHTTPRouteCreate(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/1/namespaces/web/gatewayhttproutes", RespondString(http.StatusNoContent, "", ""))
	mock.On("GET", "/kubernetes/1/namespaces/web/gatewayhttproutes", RespondJSON(http.StatusOK, httpRouteListing()))

	r := resourceKubernetesGatewayHTTPRoute()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "web")
	_ = d.Set("name", "app")
	_ = d.Set("hostnames", []interface{}{"app.example.com"})
	_ = d.Set("parent_ref", []interface{}{map[string]interface{}{"name": "edge", "section_name": "http"}})
	_ = d.Set("rule", []interface{}{map[string]interface{}{
		"match": []interface{}{map[string]interface{}{
			"path_type": "PathPrefix", "path_value": "/api",
			"header": []interface{}{map[string]interface{}{"name": "x-env", "value": "prod"}},
		}},
		"filter": []interface{}{map[string]interface{}{
			"type": "RequestHeaderModifier",
			"request_header_modifier": []interface{}{map[string]interface{}{
				"set":    []interface{}{map[string]interface{}{"name": "x-from", "value": "gw"}},
				"remove": []interface{}{"x-debug"},
			}},
		}},
		"backend_ref": []interface{}{map[string]interface{}{"name": "api", "port": 8080, "weight": 1}},
	}})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if d.Id() != "1:web:app" {
		t.Errorf("ID: got %q", d.Id())
	}

	var payload map[string]interface{}
	if err := mock.FindRequest("POST", "/kubernetes/1/namespaces/web/gatewayhttproutes").DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	parent := payload["ParentRefs"].([]interface{})[0].(map[string]interface{})
	if parent["Name"] != "edge" || parent["SectionName"] != "http" {
		t.Errorf("parent ref payload: %v", parent)
	}
	rule := payload["Rules"].([]interface{})[0].(map[string]interface{})
	match := rule["Matches"].([]interface{})[0].(map[string]interface{})
	if path := match["Path"].(map[string]interface{}); path["Type"] != "PathPrefix" || path["Value"] != "/api" {
		t.Errorf("path payload: %v", path)
	}
	filter := rule["Filters"].([]interface{})[0].(map[string]interface{})
	if hm := filter["RequestHeaderModifier"].(map[string]interface{}); hm["Remove"].([]interface{})[0] != "x-debug" {
		t.Errorf("header modifier payload: %v", hm)
	}
	if _, ok := filter["RequestRedirect"]; ok {
		t.Error("an unset redirect must be left out")
	}
	backend := rule["BackendRefs"].([]interface{})[0].(map[string]interface{})["BackendRef"].(map[string]interface{})
	if backend["Weight"] != float64(1) {
		t.Errorf("backend weight: %v", backend["Weight"])
	}
	if obj := backend["BackendObjectReference"].(map[string]interface{}); obj["Name"] != "api" || obj["Port"] != float64(8080) {
		t.Errorf("backend payload: %v", obj)
	}

	// Defaults filled in by Kubernetes come back without a diff-causing gap.
	if got := d.Get("parent_ref.0.group"); got != "gateway.networking.k8s.io" {
		t.Errorf("parent_ref.0.group: got %v", got)
	}
	if got := d.Get("rule.0.match.0.header.0.type"); got != "Exact" {
		t.Errorf("header type: got %v", got)
	}
	if got := d.Get("rule.0.backend_ref.0.kind"); got != "Service" {
		t.Errorf("backend kind: got %v", got)
	}
	if got := d.Get("resource_version"); got != "7" {
		t.Errorf("resource_version: got %v", got)
	}
}

// TestKubernetesGatewayHTTPRoute_ZeroWeightIsSent verifies weight 0 (no
// traffic) is sent rather than dropped as unset.
func TestKubernetesGatewayHTTPRoute_ZeroWeightIsSent(t *testing.T) {
	r := resourceKubernetesGatewayHTTPRoute()
	d := r.TestResourceData()
	_ = d.Set("rule", []interface{}{map[string]interface{}{
		"backend_ref": []interface{}{map[string]interface{}{"name": "canary", "weight": 0}},
	}})
	route := buildKubernetesGatewayHTTPRoute(d)
	if w := route.Rules[0].BackendRefs[0].BackendRef.Weight; w != 0 {
		t.Errorf("weight: got %d", w)
	}
	if route.Rules[0].Matches == nil || route.ParentRefs == nil {
		t.Error("lists must be sent empty, not null")
	}
}

func TestKubernetesGatewayHTTPRouteCreate_ExplainsOldServer(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
		"ServerVersion": "2.45.3", "ServerEdition": "BE",
	}))
	r := resourceKubernetesGatewayHTTPRoute()
	d := r.TestResourceData()
	_ = d.Set("environment_id", 1)
	_ = d.Set("namespace", "web")
	_ = d.Set("name", "app")
	err := rcCreate(r, d, mock.Client())
	if err == nil || !strings.Contains(err.Error(), "portainer_kubernetes_gateway_http_route requires") {
		t.Fatalf("expected the requirement to be named, got %v", err)
	}
}

// TestKubernetesGatewayHTTPRouteRead_Import verifies a read from the ID alone
// fills the nested rule, including a redirect filter.
func TestKubernetesGatewayHTTPRouteRead_Import(t *testing.T) {
	listing := httpRouteListing()
	listing[0]["Rules"] = []map[string]interface{}{{
		"Filters": []map[string]interface{}{{
			"Type":            "RequestRedirect",
			"RequestRedirect": map[string]interface{}{"Scheme": "https", "StatusCode": 301},
		}},
	}}
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/1/namespaces/web/gatewayhttproutes", RespondJSON(http.StatusOK, listing))

	r := resourceKubernetesGatewayHTTPRoute()
	d := r.TestResourceData()
	d.SetId("1:web:app")
	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("rule.0.filter.0.request_redirect.0.status_code"); got != 301 {
		t.Errorf("status_code: got %v", got)
	}
	if got := d.Get("hostnames.0"); got != "app.example.com" {
		t.Errorf("hostnames: got %v", got)
	}
	if d.Get("name") != "app" || d.Get("environment_id") != 1 {
		t.Error("identity not filled")
	}
}

func TestKubernetesGatewayHTTPRouteRead_DropsMissing(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/1/namespaces/web/gatewayhttproutes", RespondJSON(http.StatusOK, []interface{}{}))

	r := resourceKubernetesGatewayHTTPRoute()
	d := r.TestResourceData()
	d.SetId("1:web:app")
	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected ID cleared, got %q", d.Id())
	}

	failing := NewMockServer(t)
	failing.On("GET", "/kubernetes/1/namespaces/web/gatewayhttproutes", RespondString(http.StatusInternalServerError, "", "boom"))
	d.SetId("1:web:app")
	if err := rcRead(r, d, failing.Client()); err == nil {
		t.Error("expected an error on 500")
	}
	d.SetId("bad")
	if err := rcRead(r, d, failing.Client()); err == nil {
		t.Error("expected an error for a malformed ID")
	}
}

func TestKubernetesGatewayHTTPRouteUpdate(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/kubernetes/1/namespaces/web/gatewayhttproutes", RespondString(http.StatusNoContent, "", ""))
	mock.On("GET", "/kubernetes/1/namespaces/web/gatewayhttproutes", RespondJSON(http.StatusOK, httpRouteListing()))

	r := resourceKubernetesGatewayHTTPRoute()
	d := r.TestResourceData()
	d.SetId("1:web:app")
	_ = d.Set("namespace", "web")
	_ = d.Set("name", "app")
	_ = d.Set("resource_version", "6")
	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}
	var payload map[string]interface{}
	if err := mock.FindRequest("PUT", "/kubernetes/1/namespaces/web/gatewayhttproutes").DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if payload["ResourceVersion"] != "6" {
		t.Errorf("payload.ResourceVersion: got %v", payload["ResourceVersion"])
	}

	failing := NewMockServer(t)
	failing.On("PUT", "/kubernetes/1/namespaces/web/gatewayhttproutes", RespondString(http.StatusConflict, "", "stale"))
	if err := rcUpdate(r, d, failing.Client()); err == nil {
		t.Error("expected an error on 409")
	}
}

func TestKubernetesGatewayHTTPRouteDelete(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/1/gatewayhttproutes/delete", RespondString(http.StatusNoContent, "", ""))

	r := resourceKubernetesGatewayHTTPRoute()
	d := r.TestResourceData()
	d.SetId("1:web:app")
	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	var payload map[string][]string
	if err := mock.FindRequest("POST", "/kubernetes/1/gatewayhttproutes/delete").DecodeJSON(&payload); err != nil {
		t.Fatalf("decode body: %v", err)
	}
	if names := payload["web"]; len(names) != 1 || names[0] != "app" {
		t.Errorf("payload: expected {web: [app]}, got %v", payload)
	}

	failing := NewMockServer(t)
	failing.On("POST", "/kubernetes/1/gatewayhttproutes/delete", RespondString(http.StatusInternalServerError, "", "boom"))
	d.SetId("1:web:app")
	if err := rcDelete(r, d, failing.Client()); err == nil {
		t.Error("expected an error on 500")
	}
}
