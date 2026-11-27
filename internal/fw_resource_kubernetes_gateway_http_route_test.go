package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// httpRouteDefaults fills what Kubernetes defaults in an HTTPRoute.
func httpRouteDefaults(obj map[string]interface{}) {
	parents, _ := obj["ParentRefs"].([]interface{})
	for _, p := range parents {
		pm := p.(map[string]interface{})
		if pm["Group"] == nil || pm["Group"] == "" {
			pm["Group"] = "gateway.networking.k8s.io"
		}
	}
	rules, _ := obj["Rules"].([]interface{})
	for _, r := range rules {
		rm := r.(map[string]interface{})
		matches, _ := rm["Matches"].([]interface{})
		if len(matches) == 0 {
			matches = []interface{}{map[string]interface{}{}}
		}
		for _, m := range matches {
			mm := m.(map[string]interface{})
			if mm["Path"] == nil {
				mm["Path"] = map[string]interface{}{"Type": "PathPrefix", "Value": "/"}
			}
			for _, key := range []string{"Headers", "QueryParams"} {
				list, _ := mm[key].([]interface{})
				for _, h := range list {
					hm := h.(map[string]interface{})
					if hm["Type"] == nil || hm["Type"] == "" {
						hm["Type"] = "Exact"
					}
				}
			}
		}
		rm["Matches"] = matches
		backends, _ := rm["BackendRefs"].([]interface{})
		for _, b := range backends {
			ref := b.(map[string]interface{})["BackendRef"].(map[string]interface{})["BackendObjectReference"].(map[string]interface{})
			if ref["Kind"] == nil || ref["Kind"] == "" {
				ref["Kind"] = "Service"
			}
		}
	}
}

const httpRouteFull = `
resource "portainer_kubernetes_gateway_http_route" "r" {
  environment_id = 1
  namespace      = "web"
  name           = "app"
  hostnames      = ["app.example.com"]
  parent_ref {
    name         = "edge"
    section_name = "http"
  }
  rule {
    match {
      path_type  = "PathPrefix"
      path_value = "/api"
      header {
        name  = "x-env"
        value = "prod"
      }
    }
    filter {
      type = "RequestHeaderModifier"
      request_header_modifier {
        set {
          name  = "x-from"
          value = "gw"
        }
        remove = ["x-debug"]
      }
    }
    backend_ref {
      name = "api"
      port = %d
    }
    backend_ref {
      name   = "canary"
      port   = 8080
      weight = 0
    }
  }
  rule {
    filter {
      type = "RequestRedirect"
      request_redirect {
        scheme      = "https"
        status_code = 301
      }
    }
  }
}
`

// TestKubernetesGatewayHTTPRoute_Lifecycle: the nested payload, the defaults
// Kubernetes fills (group, header type, backend kind, the default match of a
// rule without one) reading back without a diff, weight 0 sent, the resource
// version sent on update, import and the bulk delete.
func TestKubernetesGatewayHTTPRoute_Lifecycle(t *testing.T) {
	mock := NewMockServer(t)
	fake := newFakeGatewayAPI(mock, "gatewayhttproutes", httpRouteDefaults)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(httpRouteFull, 8080),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_gateway_http_route.r", "id", "1:web:app"),
					resource.TestCheckResourceAttr("portainer_kubernetes_gateway_http_route.r", "parent_ref.0.group", "gateway.networking.k8s.io"),
					resource.TestCheckResourceAttr("portainer_kubernetes_gateway_http_route.r", "rule.0.match.0.header.0.type", "Exact"),
					resource.TestCheckResourceAttr("portainer_kubernetes_gateway_http_route.r", "rule.0.backend_ref.0.kind", "Service"),
					resource.TestCheckResourceAttr("portainer_kubernetes_gateway_http_route.r", "rule.0.backend_ref.0.weight", "1"),
					resource.TestCheckResourceAttr("portainer_kubernetes_gateway_http_route.r", "rule.1.match.#", "0"),
					resource.TestCheckResourceAttr("portainer_kubernetes_gateway_http_route.r", "resource_version", "1"),
					a2Check(func() error {
						var body map[string]interface{}
						a2LastBody(t, mock, "POST", "/kubernetes/1/namespaces/web/gatewayhttproutes", &body)
						parent := body["ParentRefs"].([]interface{})[0].(map[string]interface{})
						if parent["Name"] != "edge" || parent["SectionName"] != "http" {
							return fmt.Errorf("parent ref payload: %v", parent)
						}
						rule := body["Rules"].([]interface{})[0].(map[string]interface{})
						path := rule["Matches"].([]interface{})[0].(map[string]interface{})["Path"].(map[string]interface{})
						if path["Type"] != "PathPrefix" || path["Value"] != "/api" {
							return fmt.Errorf("path payload: %v", path)
						}
						filter := rule["Filters"].([]interface{})[0].(map[string]interface{})
						if hm := filter["RequestHeaderModifier"].(map[string]interface{}); hm["Remove"].([]interface{})[0] != "x-debug" {
							return fmt.Errorf("header modifier payload: %v", hm)
						}
						if _, ok := filter["RequestRedirect"]; ok {
							return fmt.Errorf("an unset redirect must be left out")
						}
						backends := rule["BackendRefs"].([]interface{})
						first := backends[0].(map[string]interface{})["BackendRef"].(map[string]interface{})
						if first["Weight"] != float64(1) || first["BackendObjectReference"].(map[string]interface{})["Port"] != float64(8080) {
							return fmt.Errorf("backend payload: %v", first)
						}
						if w := backends[1].(map[string]interface{})["BackendRef"].(map[string]interface{})["Weight"]; w != float64(0) {
							return fmt.Errorf("weight 0 must be sent, got %v", w)
						}
						second := body["Rules"].([]interface{})[1].(map[string]interface{})
						if m := second["Matches"].([]interface{}); len(m) != 0 {
							return fmt.Errorf("a rule without match must send none: %v", m)
						}
						redirect := second["Filters"].([]interface{})[0].(map[string]interface{})["RequestRedirect"].(map[string]interface{})
						if redirect["StatusCode"] != float64(301) || redirect["Scheme"] != "https" {
							return fmt.Errorf("redirect payload: %v", redirect)
						}
						return nil
					}),
				),
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(httpRouteFull, 9090),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_kubernetes_gateway_http_route.r", plancheck.ResourceActionUpdate),
				}},
				Check: a2Check(func() error {
					var body map[string]interface{}
					a2LastBody(t, mock, "PUT", "/kubernetes/1/namespaces/web/gatewayhttproutes", &body)
					if body["ResourceVersion"] != "1" {
						return fmt.Errorf("update payload.ResourceVersion: %v", body["ResourceVersion"])
					}
					return nil
				}),
			},
			{
				PreConfig: func() {
					fake.edit("app", func(o map[string]interface{}) { o["Hostnames"] = []interface{}{"other.example.com"} })
				},
				Config:             mockProviderConfig(mock) + fmt.Sprintf(httpRouteFull, 9090),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{Config: mockProviderConfig(mock) + fmt.Sprintf(httpRouteFull, 9090)},
			{
				ResourceName:            "portainer_kubernetes_gateway_http_route.r",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"rule.1.match"},
			},
			{
				PreConfig:          fake.clear,
				Config:             mockProviderConfig(mock) + fmt.Sprintf(httpRouteFull, 9090),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

const httpRouteMinimal = `
resource "portainer_kubernetes_gateway_http_route" "r" {
  environment_id = 1
  namespace      = "web"
  name           = "app"
  parent_ref {
    name = "edge"
  }
  rule {
    backend_ref {
      name = "api"
    }
  }
}
`

func TestKubernetesGatewayHTTPRoute_DestroyAndErrors(t *testing.T) {
	mock := NewMockServer(t)
	newFakeGatewayAPI(mock, "gatewayhttproutes", httpRouteDefaults)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + httpRouteMinimal,
				Check: resource.ComposeTestCheckFunc(
					// The default match Kubernetes adds is not in state.
					resource.TestCheckResourceAttr("portainer_kubernetes_gateway_http_route.r", "rule.0.match.#", "0"),
					resource.TestCheckNoResourceAttr("portainer_kubernetes_gateway_http_route.r", "hostnames"),
					resource.TestCheckNoResourceAttr("portainer_kubernetes_gateway_http_route.r", "parent_ref.0.namespace"),
				),
			},
			{
				Config:   mockProviderConfig(mock) + httpRouteMinimal,
				PlanOnly: true,
			},
		},
		CheckDestroy: a2Check(func() error {
			var body map[string][]string
			a2LastBody(t, mock, "POST", "/kubernetes/1/gatewayhttproutes/delete", &body)
			if names := body["web"]; len(names) != 1 || names[0] != "app" {
				return fmt.Errorf("delete payload: %v", body)
			}
			return nil
		}),
	})

	old := NewMockServer(t)
	old.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{"ServerVersion": "2.45.3", "ServerEdition": "BE"}))
	failing := NewMockServer(t)
	failing.On("GET", "/kubernetes/1/namespaces/web/gatewayhttproutes", RespondString(http.StatusInternalServerError, "", "boom"))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:      mockProviderConfig(old) + httpRouteMinimal,
				ExpectError: regexp.MustCompile(`portainer_kubernetes_gateway_http_route\s+requires`),
			},
			{
				Config:        mockProviderConfig(failing) + httpRouteMinimal,
				ResourceName:  "portainer_kubernetes_gateway_http_route.r",
				ImportState:   true,
				ImportStateId: "1:web:app",
				ExpectError:   regexp.MustCompile(`failed to read HTTPRoute web/app`),
			},
			{
				Config:        mockProviderConfig(failing) + httpRouteMinimal,
				ResourceName:  "portainer_kubernetes_gateway_http_route.r",
				ImportState:   true,
				ImportStateId: "bad",
				ExpectError:   regexp.MustCompile(`expected <environment_id>:<namespace>:<name>`),
			},
		},
	})
}

func TestKubernetesGatewayHTTPRoute_UpgradeFromV300(t *testing.T) {
	mock := NewMockServer(t)
	newFakeGatewayAPI(mock, "gatewayhttproutes", httpRouteDefaults)
	a2Upgrade(t, mock, httpRouteMinimal, fmt.Sprintf(httpRouteFull, 8080))
}
