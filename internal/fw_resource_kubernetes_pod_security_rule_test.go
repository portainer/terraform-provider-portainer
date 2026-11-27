package internal

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
)

// fakePodSecurityRule stores the rule of environment 5 as it was last put.
func fakePodSecurityRule(mock *MockServer) *a2Store {
	s := newA2Store()
	s.put("rule", map[string]interface{}{"enabled": false, "endPointID": 5})
	s.serve(mock, "/kubernetes/5/opa", "rule")
	mock.On("PUT", "/kubernetes/5/opa", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		a2Decode(r, &body)
		s.put("rule", body)
		RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
	})
	return s
}

const podSecurityRuleFull = `
resource "portainer_kubernetes_pod_security_rule" "r" {
  endpoint_id           = 5
  privileged_containers = %t
  host_namespaces       = true

  allow_proc_mount {
    proc_mount_type = "Default"
  }
  allow_flex_volumes {
    allowed_volumes = ["cifs"]
  }
  app_armor {
    types = ["runtime/default"]
  }
  capabilities {
    allowed       = ["NET_BIND_SERVICE"]
    required_drop = ["ALL"]
  }
  forbidden_sysctls {
    sysctls = ["kernel.shm*"]
  }
  host_filesystem {
    allowed_path {
      path_prefix = "/var/log"
      readonly    = true
    }
  }
  host_ports {
    min = 30000
    max = 32767
  }
  sec_comp {
    types = ["runtime/default"]
  }
  selinux {
    allowed_context {
      user = "system_u"
    }
  }
  volume_types {
    allowed_types = ["configMap"]
  }
  users {
    run_as_user {
      type = "MustRunAs"
      id_range {
        min = 1000
        max = 2000
      }
    }
    fs_groups {}
  }
}
`

func podSecuritySectionOf(payload map[string]interface{}, key string) (map[string]interface{}, error) {
	section, ok := payload[key].(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("%s: got %v", key, payload[key])
	}
	return section, nil
}

// TestKubernetesPodSecurityRule_Lifecycle: every section goes out under
// Portainer's own key names (the sysctls under requiredDropCapabilities),
// the switches are read back, a section left unset in a strategy is left out,
// import restores the switches and destroy switches the rule off.
func TestKubernetesPodSecurityRule_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fakePodSecurityRule(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(podSecurityRuleFull, true),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_pod_security_rule.r", "id", "5"),
					resource.TestCheckResourceAttr("portainer_kubernetes_pod_security_rule.r", "enabled", "true"),
					resource.TestCheckResourceAttr("portainer_kubernetes_pod_security_rule.r", "host_ports.0.enabled", "true"),
					a2Check(func() error {
						var payload map[string]interface{}
						a2LastBody(t, mock, "PUT", "/kubernetes/5/opa", &payload)
						if payload["endPointID"] != float64(5) || payload["enabled"] != true {
							return fmt.Errorf("the rule identity was not sent: %v", payload)
						}
						if p, _ := podSecuritySectionOf(payload, "privilegedContainers"); p == nil || p["enabled"] != true {
							return fmt.Errorf("a plain switch must be sent as an object: %v", payload["privilegedContainers"])
						}
						caps, err := podSecuritySectionOf(payload, "capabilities")
						if err != nil {
							return err
						}
						if allowed, ok := caps["allowedCapabilities"].([]interface{}); !ok || len(allowed) != 1 {
							return fmt.Errorf("allowed capabilities: %v", caps)
						}
						ports, err := podSecuritySectionOf(payload, "hostPorts")
						if err != nil || ports["min"] != float64(30000) || ports["max"] != float64(32767) || ports["hostNetwork"] != false {
							return fmt.Errorf("host ports: %v %w", ports, err)
						}
						fs, err := podSecuritySectionOf(payload, "hostFilesystem")
						if err != nil {
							return err
						}
						paths, _ := fs["allowedPaths"].([]interface{})
						if len(paths) != 1 || paths[0].(map[string]interface{})["pathPrefix"] != "/var/log" || paths[0].(map[string]interface{})["readonly"] != true {
							return fmt.Errorf("allowed paths: %v", fs)
						}
						sysctls, err := podSecuritySectionOf(payload, "forbiddenSysctlsList")
						if err != nil {
							return err
						}
						if l, _ := sysctls["requiredDropCapabilities"].([]interface{}); len(l) != 1 || l[0] != "kernel.shm*" {
							return fmt.Errorf("the sysctls must go under Portainer's own key name: %v", sysctls)
						}
						proc, _ := podSecuritySectionOf(payload, "allowProcMount")
						if proc["procMountType"] != "Default" || proc["enabled"] != true {
							return fmt.Errorf("proc mount: %v", proc)
						}
						selinux, _ := podSecuritySectionOf(payload, "selinux")
						if ctxs, _ := selinux["allowedCapabilities"].([]interface{}); len(ctxs) != 1 || ctxs[0].(map[string]interface{})["user"] != "system_u" {
							return fmt.Errorf("selinux: %v", selinux)
						}
						for key, field := range map[string]string{"allowFlexVolumes": "allowedVolumes", "appArmor": "AppArmorType", "secComp": "secCompType", "volumeTypes": "allowedTypes"} {
							section, err := podSecuritySectionOf(payload, key)
							if err != nil {
								return err
							}
							if l, _ := section[field].([]interface{}); len(l) != 1 {
								return fmt.Errorf("%s.%s: %v", key, field, section)
							}
						}
						users, err := podSecuritySectionOf(payload, "users")
						if err != nil {
							return err
						}
						runAsUser, ok := users["runAsUser"].(map[string]interface{})
						if !ok || runAsUser["type"] != "MustRunAs" {
							return fmt.Errorf("run-as strategy: %v", users)
						}
						if ranges, _ := runAsUser["idrange"].([]interface{}); len(ranges) != 1 || ranges[0].(map[string]interface{})["min"] != float64(1000) {
							return fmt.Errorf("identifier ranges: %v", runAsUser)
						}
						if _, ok := users["fsGroups"]; ok {
							return fmt.Errorf("an empty strategy block must be left out: %v", users)
						}
						if _, ok := users["runAsGroup"]; ok {
							return fmt.Errorf("an unset strategy must be left out: %v", users)
						}
						return nil
					}),
				),
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(podSecurityRuleFull, false),
				Check:  resource.TestCheckResourceAttr("portainer_kubernetes_pod_security_rule.r", "privileged_containers", "false"),
			},
			{
				ResourceName:      "portainer_kubernetes_pod_security_rule.r",
				ImportState:       true,
				ImportStateVerify: true,
				// The sections are driven by the configuration, not read back.
				ImportStateVerifyIgnore: []string{
					"allow_proc_mount", "allow_flex_volumes", "app_armor", "capabilities", "forbidden_sysctls",
					"host_filesystem", "host_ports", "sec_comp", "selinux", "volume_types", "users",
				},
			},
		},
		CheckDestroy: a2Check(func() error {
			var payload map[string]interface{}
			a2LastBody(t, mock, "PUT", "/kubernetes/5/opa", &payload)
			if payload["enabled"] != false || payload["endPointID"] != float64(5) {
				return fmt.Errorf("destroy must switch the rule off, got %v", payload)
			}
			return nil
		}),
	})
}

const podSecurityRuleMinimal = `
resource "portainer_kubernetes_pod_security_rule" "r" {
  endpoint_id = 5
}
`

// TestKubernetesPodSecurityRule_Drift: a switch changed in Portainer is
// planned back, and a rule Portainer no longer reports leaves state.
func TestKubernetesPodSecurityRule_Drift(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	s := fakePodSecurityRule(mock)
	config := mockProviderConfig(mock) + podSecurityRuleMinimal
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					s.put("rule", map[string]interface{}{"enabled": true, "restrictSecrets": map[string]interface{}{"enabled": true}})
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				PreConfig:          func() { s.del("rule") },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestKubernetesPodSecurityRule_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fakePodSecurityRule(mock)
	a2Upgrade(t, mock, podSecurityRuleMinimal, fmt.Sprintf(podSecurityRuleFull, true))
}
