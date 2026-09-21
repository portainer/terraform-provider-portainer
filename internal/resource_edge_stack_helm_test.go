package internal

import (
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// =========================================================================
// Edge stacks deployed from a Helm chart repository (Business Edition).
//
// This is a fourth deployment source on the existing portainer_edge_stack
// rather than a resource of its own: it is the same object, reached through
// its own create and update endpoints, and the resource already selects the
// source by which argument is populated.
// =========================================================================

func helmEdgeStackData(t *testing.T, block map[string]interface{}) (*schema.Resource, *schema.ResourceData) {
	t.Helper()
	r := resourceEdgeStack()
	d := r.TestResourceData()
	_ = d.Set("name", "monitoring")
	_ = d.Set("edge_groups", []interface{}{1, 2})
	_ = d.Set("helm_config", []interface{}{block})
	return r, d
}

// respondHelmEdgeStack is the stack as Portainer reports it back.
func respondHelmEdgeStack() http.HandlerFunc {
	return RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 9, "Name": "monitoring", "EdgeGroups": []int{1, 2}, "DeploymentType": 1,
		"HelmConfig": map[string]interface{}{
			"ChartURL": "https://charts.example.com", "ChartName": "kube-prometheus-stack",
			"ChartVersion": "51.2.0", "Namespace": "monitoring",
			"ValuesInline": "grafana:\n  enabled: true\n", "Atomic": true, "Timeout": "5m0s",
		},
	})
}

// TestEdgeStackHelm_CreateUsesHelmEndpoint pins the create path and the
// payload, whose keys are PascalCase where the other edge stack payloads are
// camelCase.
func TestEdgeStackHelm_CreateUsesHelmEndpoint(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []interface{}{}))
	mock.On("POST", "/edge_stacks/create/helmRepo", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 9}))
	mock.On("GET", "/edge_stacks/9", respondHelmEdgeStack())

	r, d := helmEdgeStackData(t, map[string]interface{}{
		"chart_url":     "https://charts.example.com",
		"chart_name":    "kube-prometheus-stack",
		"chart_version": "51.2.0",
		"namespace":     "monitoring",
		"values_inline": "grafana:\n  enabled: true\n",
		"atomic":        true,
		"timeout":       "5m0s",
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	req := mock.FindRequest("POST", "/edge_stacks/create/helmRepo")
	if req == nil {
		t.Fatal("a Helm stack must be created through the Helm endpoint")
	}
	var payload struct {
		Name       string `json:"Name"`
		EdgeGroups []int  `json:"EdgeGroups"`
		HelmConfig struct {
			ChartURL     string `json:"ChartURL"`
			ChartName    string `json:"ChartName"`
			ChartVersion string `json:"ChartVersion"`
			Namespace    string `json:"Namespace"`
			ValuesInline string `json:"ValuesInline"`
			Atomic       bool   `json:"Atomic"`
			Timeout      string `json:"Timeout"`
		} `json:"HelmConfig"`
	}
	if err := req.DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if payload.Name != "monitoring" || len(payload.EdgeGroups) != 2 {
		t.Errorf("the stack identity was not sent: %+v", payload)
	}
	if payload.HelmConfig.ChartURL != "https://charts.example.com" || payload.HelmConfig.ChartName != "kube-prometheus-stack" {
		t.Errorf("the required chart fields were not sent: %+v", payload.HelmConfig)
	}
	if payload.HelmConfig.ChartVersion != "51.2.0" || payload.HelmConfig.Namespace != "monitoring" {
		t.Errorf("the optional chart fields were not sent: %+v", payload.HelmConfig)
	}
	if !payload.HelmConfig.Atomic || payload.HelmConfig.Timeout != "5m0s" {
		t.Errorf("the Helm flags were not sent: %+v", payload.HelmConfig)
	}

	// The other create endpoints must stay untouched.
	for _, path := range []string{"/edge_stacks/create/string", "/edge_stacks/create/file", "/edge_stacks/create/repository"} {
		if mock.FindRequest("POST", path) != nil {
			t.Errorf("a Helm stack must not be sent to %s", path)
		}
	}
}

// TestEdgeStackHelm_OmitsEmptyOptionals keeps an update from clearing a chart
// setting that was configured outside Terraform.
func TestEdgeStackHelm_OmitsEmptyOptionals(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []interface{}{}))
	mock.On("POST", "/edge_stacks/create/helmRepo", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 9}))
	mock.On("GET", "/edge_stacks/9", respondHelmEdgeStack())

	r, d := helmEdgeStackData(t, map[string]interface{}{
		"chart_url":  "https://charts.example.com",
		"chart_name": "kube-prometheus-stack",
	})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload struct {
		HelmConfig map[string]interface{} `json:"HelmConfig"`
	}
	if err := mock.FindRequest("POST", "/edge_stacks/create/helmRepo").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	for _, key := range []string{"ChartVersion", "Namespace", "ValuesInline", "Timeout", "Atomic"} {
		if _, ok := payload.HelmConfig[key]; ok {
			t.Errorf("%s was not configured, so it must not be sent: %v", key, payload.HelmConfig[key])
		}
	}
	for _, key := range []string{"ChartURL", "ChartName"} {
		if _, ok := payload.HelmConfig[key]; !ok {
			t.Errorf("%s is required and must always be sent", key)
		}
	}
}

// TestEdgeStackHelm_UpdateUsesHelmEndpoint pins the update path, which takes
// the chart configuration rather than a stack file.
func TestEdgeStackHelm_UpdateUsesHelmEndpoint(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/edge_stacks/9/helmRepo", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 9}))
	mock.On("GET", "/edge_stacks/9", respondHelmEdgeStack())

	r, d := helmEdgeStackData(t, map[string]interface{}{
		"chart_url":     "https://charts.example.com",
		"chart_name":    "kube-prometheus-stack",
		"chart_version": "51.3.0",
	})
	d.SetId("9")

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	req := mock.FindRequest("PUT", "/edge_stacks/9/helmRepo")
	if req == nil {
		t.Fatal("a Helm stack must be updated through the Helm endpoint")
	}
	var payload struct {
		EdgeGroups []int `json:"EdgeGroups"`
		HelmConfig struct {
			ChartVersion string `json:"ChartVersion"`
		} `json:"HelmConfig"`
	}
	if err := req.DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if payload.HelmConfig.ChartVersion != "51.3.0" {
		t.Errorf("the new chart version was not sent: %+v", payload.HelmConfig)
	}
	if len(payload.EdgeGroups) != 2 {
		t.Errorf("the edge groups must be sent on update too, got %v", payload.EdgeGroups)
	}
	if mock.FindRequest("PUT", "/edge_stacks/9") != nil {
		t.Error("a Helm stack must not be updated through the generic stack endpoint")
	}
}

// TestEdgeStackHelm_ReadPopulatesBlock covers reading the chart configuration
// back, which is what lets an imported Helm stack plan cleanly.
func TestEdgeStackHelm_ReadPopulatesBlock(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_stacks/9", respondHelmEdgeStack())

	r := resourceEdgeStack()
	d := r.TestResourceData()
	d.SetId("9")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	blocks := d.Get("helm_config").([]interface{})
	if len(blocks) != 1 {
		t.Fatalf("expected the Helm block to be populated, got %d", len(blocks))
	}
	block := blocks[0].(map[string]interface{})
	if block["chart_name"] != "kube-prometheus-stack" || block["chart_version"] != "51.2.0" {
		t.Errorf("the chart was not read back: %+v", block)
	}
	if block["atomic"] != true || block["timeout"] != "5m0s" {
		t.Errorf("the Helm flags were not read back: %+v", block)
	}
}

// TestEdgeStackHelm_GitStackKeepsNoHelmBlock is the reason the read is gated
// on ChartURL: Portainer also fills HelmConfig for a git-backed stack that
// deploys a chart from the cloned repository, and a git stack must not grow a
// helm_config it never declared.
func TestEdgeStackHelm_GitStackKeepsNoHelmBlock(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/edge_stacks/9", RespondJSON(http.StatusOK, map[string]interface{}{
		"Id": 9, "Name": "monitoring", "DeploymentType": 1,
		"GitConfig": map[string]interface{}{"URL": "https://github.com/example/charts"},
		// A chart from inside the cloned repository: a path, never a URL.
		"HelmConfig": map[string]interface{}{
			"ChartPath": "charts/monitoring", "Namespace": "monitoring",
		},
	}))

	r := resourceEdgeStack()
	d := r.TestResourceData()
	d.SetId("9")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if blocks := d.Get("helm_config").([]interface{}); len(blocks) != 0 {
		t.Errorf("a git-backed stack must not be given a helm_config block, got %v", blocks)
	}
}
