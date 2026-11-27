package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// fakeIngresses stores the ingresses of environment 1 by namespace/name, as
// the networking.k8s.io/v1 objects the Kubernetes proxy serves.
func fakeIngresses(mock *MockServer) *a2Store {
	s := newA2Store()
	store := func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		a2Decode(r, &body)
		s.put(fmt.Sprintf("%v/%v", body["Namespace"], body["Name"]), ingressK8sObject(body))
		w.WriteHeader(http.StatusNoContent)
	}
	mock.On("POST", "/kubernetes/1/namespaces/default/ingresses", store)
	mock.On("PUT", "/kubernetes/1/namespaces/default/ingresses", store)
	mock.On("POST", "/kubernetes/1/ingresses/delete", func(w http.ResponseWriter, r *http.Request) {
		var body map[string][]string
		a2Decode(r, &body)
		for ns, names := range body {
			for _, n := range names {
				s.del(ns + "/" + n)
			}
		}
		w.WriteHeader(http.StatusNoContent)
	})
	s.serve(mock, "/endpoints/1/kubernetes/apis/networking.k8s.io/v1/namespaces/default/ingresses/my-ingress", "default/my-ingress")
	return s
}

// ingressK8sObject builds the Ingress the way Portainer's convertToK8sIngress
// does: labels replaced by Portainer's owner label, no class when it is
// empty, one rule per path host - here in reverse order, as Portainer's map
// iteration gives no order - then a rule for each host without paths.
func ingressK8sObject(body map[string]interface{}) map[string]interface{} {
	spec := map[string]interface{}{}
	if class, _ := body["ClassName"].(string); class != "" {
		spec["ingressClassName"] = class
	}
	tlsBody, _ := body["TLS"].([]interface{})
	tls := make([]interface{}, 0, len(tlsBody))
	for _, raw := range tlsBody {
		t := raw.(map[string]interface{})
		item := map[string]interface{}{"secretName": t["SecretName"]}
		if hosts, ok := t["Hosts"].([]interface{}); ok && len(hosts) > 0 {
			item["hosts"] = hosts
		}
		tls = append(tls, item)
	}
	spec["tls"] = tls
	var order []string
	byHost := map[string][]interface{}{}
	for _, raw := range body["Paths"].([]interface{}) {
		p := raw.(map[string]interface{})
		host := p["Host"].(string)
		if _, ok := byHost[host]; !ok {
			order = append([]string{host}, order...)
		}
		byHost[host] = append(byHost[host], map[string]interface{}{
			"path":     p["Path"],
			"pathType": p["PathType"],
			"backend":  map[string]interface{}{"service": map[string]interface{}{"name": p["ServiceName"], "port": map[string]interface{}{"number": p["Port"]}}},
		})
	}
	rules := []interface{}{}
	for _, host := range order {
		rules = append(rules, map[string]interface{}{"host": host, "http": map[string]interface{}{"paths": byHost[host]}})
	}
	for _, raw := range body["Hosts"].([]interface{}) {
		if _, ok := byHost[raw.(string)]; !ok {
			rules = append(rules, map[string]interface{}{"host": raw})
		}
	}
	if len(rules) > 0 {
		spec["rules"] = rules
	}
	return map[string]interface{}{
		"apiVersion": "networking.k8s.io/v1",
		"kind":       "Ingress",
		"metadata": map[string]interface{}{
			"name":        body["Name"],
			"namespace":   body["Namespace"],
			"annotations": body["Annotations"],
			"labels":      map[string]interface{}{"io.portainer.kubernetes.ingress.owner": "admin"},
		},
		"spec": spec,
	}
}

// editIngress changes the stored Ingress outside Terraform.
func editIngress(s *a2Store, change func(obj map[string]interface{})) func() {
	return func() {
		v, _ := s.get("default/my-ingress")
		obj := v.(map[string]interface{})
		change(obj)
		s.put("default/my-ingress", obj)
	}
}

const ingressFull = `
resource "portainer_kubernetes_ingresses" "i" {
  environment_id = 1
  namespace      = "default"
  name           = "my-ingress"
  class_name     = "nginx"
  hosts          = ["app.example.com"]
  annotations    = { "nginx.ingress.kubernetes.io/rewrite-target" = "/" }
  labels         = { app = "web" }
  tls {
    hosts       = ["app.example.com"]
    secret_name = "app-tls"
  }
  paths {
    host         = "app.example.com"
    path         = "/"
    path_type    = "Prefix"
    port         = %d
    service_name = "web"
  }
}
`

const ingressMinimal = `
resource "portainer_kubernetes_ingresses" "i" {
  environment_id = 1
  namespace      = "default"
  name           = "my-ingress"
}
`

// TestKubernetesIngress_Lifecycle: the payload carries the ingress name in
// every path, an update PUTs, import reads every argument but labels back
// (Portainer does not store them) and destroy uses the bulk delete.
func TestKubernetesIngress_Lifecycle(t *testing.T) {
	mock := NewMockServer(t)
	s := fakeIngresses(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(ingressFull, 80),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_ingresses.i", "id", "1:default:my-ingress"),
					a2Check(func() error {
						var body map[string]interface{}
						a2LastBody(t, mock, "POST", "/kubernetes/1/namespaces/default/ingresses", &body)
						if body["Name"] != "my-ingress" || body["Namespace"] != "default" || body["ClassName"] != "nginx" {
							return fmt.Errorf("payload: %v", body)
						}
						path := body["Paths"].([]interface{})[0].(map[string]interface{})
						if path["IngressName"] != "my-ingress" || path["HasService"] != true || path["Port"] != float64(80) || path["PathType"] != "Prefix" {
							return fmt.Errorf("path payload: %v", path)
						}
						tls := body["TLS"].([]interface{})[0].(map[string]interface{})
						if tls["SecretName"] != "app-tls" || len(tls["Hosts"].([]interface{})) != 1 {
							return fmt.Errorf("tls payload: %v", tls)
						}
						if body["Labels"].(map[string]interface{})["app"] != "web" || len(body["Annotations"].(map[string]interface{})) != 1 {
							return fmt.Errorf("labels/annotations payload: %v", body)
						}
						return nil
					}),
				),
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(ingressFull, 8080),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_kubernetes_ingresses.i", plancheck.ResourceActionUpdate),
				}},
				Check: a2Check(func() error {
					if a2Count(mock, "PUT", "/kubernetes/1/namespaces/default/ingresses") != 1 {
						return fmt.Errorf("expected one PUT")
					}
					return nil
				}),
			},
			{
				ResourceName:            "portainer_kubernetes_ingresses.i",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"labels"},
			},
			{
				PreConfig:          func() { s.del("default/my-ingress") },
				Config:             mockProviderConfig(mock) + fmt.Sprintf(ingressFull, 8080),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestKubernetesIngress_Destroy(t *testing.T) {
	mock := NewMockServer(t)
	fakeIngresses(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + ingressMinimal,
				Check: a2Check(func() error {
					var body map[string]interface{}
					a2LastBody(t, mock, "POST", "/kubernetes/1/namespaces/default/ingresses", &body)
					for _, key := range []string{"Hosts", "TLS", "Paths"} {
						if l, ok := body[key].([]interface{}); !ok || len(l) != 0 {
							return fmt.Errorf("%s must be sent empty, got %v", key, body[key])
						}
					}
					if body["ClassName"] != "" {
						return fmt.Errorf("ClassName: %v", body["ClassName"])
					}
					return nil
				}),
			},
		},
		CheckDestroy: a2Check(func() error {
			var body map[string][]string
			a2LastBody(t, mock, "POST", "/kubernetes/1/ingresses/delete", &body)
			if names := body["default"]; len(names) != 1 || names[0] != "my-ingress" {
				return fmt.Errorf("delete payload: %v", body)
			}
			return nil
		}),
	})
}

func TestKubernetesIngress_Errors(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/kubernetes/1/namespaces/default/ingresses", RespondString(http.StatusConflict, "application/json", `{"message":"exists"}`))
	mock.On("GET", "/endpoints/1/kubernetes/apis/networking.k8s.io/v1/namespaces/default/ingresses/my-ingress", RespondString(http.StatusInternalServerError, "", "boom"))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:      mockProviderConfig(mock) + ingressMinimal,
				ExpectError: regexp.MustCompile(`failed to post ingress`),
			},
			{
				Config:        mockProviderConfig(mock) + ingressMinimal,
				ResourceName:  "portainer_kubernetes_ingresses.i",
				ImportState:   true,
				ImportStateId: "1:default:my-ingress",
				ExpectError:   regexp.MustCompile(`failed to read ingress my-ingress`),
			},
			{
				Config:        mockProviderConfig(mock) + ingressMinimal,
				ResourceName:  "portainer_kubernetes_ingresses.i",
				ImportState:   true,
				ImportStateId: "x:default:my-ingress",
				ExpectError:   regexp.MustCompile(`invalid ID format, expected 'envID:namespace:name'`),
			},
		},
	})
}

func TestKubernetesIngress_UpgradeFromV300(t *testing.T) {
	mock := NewMockServer(t)
	fakeIngresses(mock)
	a2Upgrade(t, mock, ingressMinimal, fmt.Sprintf(ingressFull, 80))
}

// TestKubernetesIngress_Drift: what the server adds or reorders is no drift
// (rule order, the owner label, a default class or a controller's
// annotation when not configured); a configured argument changed or removed
// in the cluster is, and the next apply puts the configuration back.
func TestKubernetesIngress_Drift(t *testing.T) {
	mock := NewMockServer(t)
	s := fakeIngresses(mock)
	const twoHosts = `
resource "portainer_kubernetes_ingresses" "i" {
  environment_id = 1
  namespace      = "default"
  name           = "my-ingress"
  class_name     = "nginx"
  hosts          = ["a.example.com", "b.example.com", "bare.example.com"]
  annotations    = { "nginx.ingress.kubernetes.io/rewrite-target" = "/" }
  tls {
    hosts       = ["a.example.com"]
    secret_name = "a-tls"
  }
  paths {
    host         = "a.example.com"
    path         = "/"
    path_type    = "Prefix"
    port         = 80
    service_name = "web"
  }
  paths {
    host         = "b.example.com"
    path         = "/api"
    path_type    = "Exact"
    port         = 8080
    service_name = "api"
  }
}
`
	config := mockProviderConfig(mock) + twoHosts
	drift := func(_ string, change func(obj map[string]interface{})) []resource.TestStep {
		return []resource.TestStep{
			{
				PreConfig:          editIngress(s, change),
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				// Applying puts the configuration back.
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_kubernetes_ingresses.i", plancheck.ResourceActionUpdate),
				}},
			},
		}
	}
	spec := func(obj map[string]interface{}) map[string]interface{} { return obj["spec"].(map[string]interface{}) }
	rules := func(obj map[string]interface{}) []interface{} { return spec(obj)["rules"].([]interface{}) }
	steps := make([]resource.TestStep, 0, 20)
	steps = append(steps,
		resource.TestStep{Config: config},
		resource.TestStep{
			// Foreign annotations and labels and reordered rules are no drift.
			PreConfig: editIngress(s, func(obj map[string]interface{}) {
				meta := obj["metadata"].(map[string]interface{})
				meta["annotations"].(map[string]interface{})["cert-manager.io/issued"] = "yes"
				meta["labels"].(map[string]interface{})["app"] = "web"
				r := rules(obj)
				r[0], r[1] = r[1], r[0]
			}),
			Config:   config,
			PlanOnly: true,
		},
	)
	steps = append(steps, drift("class", func(obj map[string]interface{}) { spec(obj)["ingressClassName"] = "traefik" })...)
	steps = append(steps, drift("class removed", func(obj map[string]interface{}) { delete(spec(obj), "ingressClassName") })...)
	steps = append(steps, drift("annotation", func(obj map[string]interface{}) {
		obj["metadata"].(map[string]interface{})["annotations"] = map[string]interface{}{"other": "x"}
	})...)
	steps = append(steps, drift("tls", func(obj map[string]interface{}) {
		spec(obj)["tls"].([]interface{})[0].(map[string]interface{})["secretName"] = "other-tls"
	})...)
	steps = append(steps, drift("tls removed", func(obj map[string]interface{}) { delete(spec(obj), "tls") })...)
	steps = append(steps, drift("path port", func(obj map[string]interface{}) {
		for _, raw := range rules(obj) {
			rule := raw.(map[string]interface{})
			if http, ok := rule["http"].(map[string]interface{}); ok && rule["host"] == "b.example.com" {
				http["paths"].([]interface{})[0].(map[string]interface{})["backend"] = map[string]interface{}{
					"service": map[string]interface{}{"name": "api", "port": map[string]interface{}{"number": 9090}},
				}
			}
		}
	})...)
	steps = append(steps, drift("host without paths removed", func(obj map[string]interface{}) {
		kept := []interface{}{}
		for _, raw := range rules(obj) {
			if raw.(map[string]interface{})["host"] != "bare.example.com" {
				kept = append(kept, raw)
			}
		}
		spec(obj)["rules"] = kept
	})...)
	steps = append(steps, drift("rule added", func(obj map[string]interface{}) {
		spec(obj)["rules"] = append(rules(obj), map[string]interface{}{"host": "extra.example.com"})
	})...)
	steps = append(steps, drift("rule with path removed", func(obj map[string]interface{}) {
		kept := []interface{}{}
		for _, raw := range rules(obj) {
			if raw.(map[string]interface{})["host"] != "a.example.com" {
				kept = append(kept, raw)
			}
		}
		spec(obj)["rules"] = kept
	})...)
	steps = append(steps, resource.TestStep{Config: config, PlanOnly: true})
	resource.UnitTest(t, resource.TestCase{ProtoV5ProviderFactories: testProtoV5Factories, Steps: steps})
}

// TestKubernetesIngress_DefaultsNotConfigured: with class_name, hosts and
// annotations left out, a default IngressClass and a controller's
// annotations are no drift, but a rule added outside Terraform is.
func TestKubernetesIngress_DefaultsNotConfigured(t *testing.T) {
	mock := NewMockServer(t)
	s := fakeIngresses(mock)
	config := mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_kubernetes_ingresses" "i" {
  environment_id = 1
  namespace      = "default"
  name           = "my-ingress"
  paths {
    host         = "app.example.com"
    path         = "/"
    path_type    = "Prefix"
    port         = %d
    service_name = "web"
  }
}
`, 80)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: editIngress(s, func(obj map[string]interface{}) {
					obj["spec"].(map[string]interface{})["ingressClassName"] = "default-class"
					obj["metadata"].(map[string]interface{})["annotations"] = map[string]interface{}{"controller": "x"}
				}),
				Config:   config,
				PlanOnly: true,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("portainer_kubernetes_ingresses.i", "class_name"),
					resource.TestCheckNoResourceAttr("portainer_kubernetes_ingresses.i", "annotations.%"),
					resource.TestCheckNoResourceAttr("portainer_kubernetes_ingresses.i", "hosts.#"),
				),
			},
			{
				PreConfig: editIngress(s, func(obj map[string]interface{}) {
					spec := obj["spec"].(map[string]interface{})
					spec["rules"] = append(spec["rules"].([]interface{}), map[string]interface{}{"host": "extra.example.com"})
				}),
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestKubernetesIngress_ForeignAnnotations: an update manages only the
// configured annotation keys. Keys someone else added (a controller,
// cert-manager, Portainer itself) are sent back unchanged, a key removed
// from the configuration is deleted, and with annotations left out of the
// configuration every live key is kept.
func TestKubernetesIngress_ForeignAnnotations(t *testing.T) {
	mock := NewMockServer(t)
	s := fakeIngresses(mock)
	config := func(port int, annotations string) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`
resource "portainer_kubernetes_ingresses" "i" {
  environment_id = 1
  namespace      = "default"
  name           = "my-ingress"
  %s
  paths {
    host         = "app.example.com"
    path         = "/"
    path_type    = "Prefix"
    port         = %d
    service_name = "web"
  }
}
`, annotations, port)
	}
	lastAnnotations := func() map[string]interface{} {
		var body map[string]interface{}
		a2LastBody(t, mock, "PUT", "/kubernetes/1/namespaces/default/ingresses", &body)
		m, _ := body["Annotations"].(map[string]interface{})
		return m
	}
	expect := func(want map[string]interface{}) resource.TestCheckFunc {
		return a2Check(func() error {
			got := lastAnnotations()
			if fmt.Sprint(got) != fmt.Sprint(want) {
				return fmt.Errorf("PUT annotations: got %v, want %v", got, want)
			}
			return nil
		})
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config(80, `annotations = { a = "1", b = "2" }`)},
			{
				PreConfig: editIngress(s, func(obj map[string]interface{}) {
					ann := obj["metadata"].(map[string]interface{})["annotations"].(map[string]interface{})
					ann["cert-manager.io/issuer"] = "le"
					ann["io.portainer.kubernetes.ingress.note"] = "x"
				}),
				Config: config(8080, `annotations = { a = "9" }`),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_kubernetes_ingresses.i", plancheck.ResourceActionUpdate),
				}},
				Check: resource.ComposeTestCheckFunc(
					expect(map[string]interface{}{"a": "9", "cert-manager.io/issuer": "le", "io.portainer.kubernetes.ingress.note": "x"}),
					resource.TestCheckResourceAttr("portainer_kubernetes_ingresses.i", "annotations.%", "1"),
				),
			},
			{Config: config(8080, `annotations = { a = "9" }`), PlanOnly: true},
			{
				// annotations left out: the removed managed key goes, the rest stays.
				Config: config(9090, ""),
				Check: resource.ComposeTestCheckFunc(
					expect(map[string]interface{}{"cert-manager.io/issuer": "le", "io.portainer.kubernetes.ingress.note": "x"}),
					resource.TestCheckNoResourceAttr("portainer_kubernetes_ingresses.i", "annotations.%"),
				),
			},
			{Config: config(9090, ""), PlanOnly: true},
		},
	})
}
