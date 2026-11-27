package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// fakeKubernetesEndpoint serves environment 9 and takes the Kubernetes
// object a PUT sends.
func fakeKubernetesEndpoint(mock *MockServer, storageClasses bool) *a2Store {
	s := newA2Store()
	config := map[string]interface{}{
		"UseLoadBalancer":                 false,
		"UseServerMetrics":                true,
		"EnableResourceOverCommit":        true,
		"ResourceOverCommitPercentage":    80,
		"RestrictDefaultNamespace":        false,
		"IngressAvailabilityPerNamespace": true,
		"AllowNoneIngressClass":           false,
		"IngressClasses":                  []map[string]interface{}{{"Name": "traefik", "Type": "traefik", "Blocked": false}},
	}
	if storageClasses {
		config["StorageClasses"] = []map[string]interface{}{{"Name": "local-path", "Provisioner": "rancher.io/local-path", "AccessModes": []string{"RWO"}, "AllowVolumeExpansion": false}}
	}
	s.put("endpoint", map[string]interface{}{
		"Id":   9,
		"Type": 5,
		"Kubernetes": map[string]interface{}{
			"Snapshots":     []map[string]interface{}{{"KubernetesVersion": "v1.31.0", "NodeCount": 1}},
			"Flags":         map[string]interface{}{"IsServerMetricsDetected": true, "IsServerStorageDetected": true},
			"Configuration": config,
		},
	})
	s.serve(mock, "/endpoints/9", "endpoint")
	mock.On("PUT", "/endpoints/9", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		a2Decode(r, &body)
		v, _ := s.get("endpoint")
		endpoint := v.(map[string]interface{})
		endpoint["Kubernetes"] = body["Kubernetes"]
		s.put("endpoint", endpoint)
		RespondJSON(http.StatusOK, map[string]interface{}{"Id": 9})(w, r)
	})
	return s
}

const environmentConfigConfig = `
resource "portainer_kubernetes_environment_config" "c" {
  endpoint_id = 9
  %s
}
`

// TestKubernetesEnvironmentConfig_Lifecycle: Portainer replaces the whole
// Kubernetes object on update, so everything this resource does not manage
// (snapshots, flags, ingress classes, settings left out) is sent back as it
// was read; storage classes replace the list when set; import reads it all.
func TestKubernetesEnvironmentConfig_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fakeKubernetesEndpoint(mock, true)
	managed := `
  use_load_balancer          = true
  restrict_default_namespace = true
  storage_class {
    name                   = "fast"
    provisioner            = "ebs.csi.aws.com"
    access_modes           = ["RWO", "RWX"]
    allow_volume_expansion = true
  }
`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(environmentConfigConfig, managed),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_environment_config.c", "id", "9"),
					resource.TestCheckResourceAttr("portainer_kubernetes_environment_config.c", "use_server_metrics", "true"),
					resource.TestCheckResourceAttr("portainer_kubernetes_environment_config.c", "resource_over_commit_percentage", "80"),
					a2Check(func() error {
						var body map[string]interface{}
						a2LastBody(t, mock, "PUT", "/endpoints/9", &body)
						kube, _ := body["Kubernetes"].(map[string]interface{})
						if snaps, _ := kube["Snapshots"].([]interface{}); len(snaps) != 1 {
							return fmt.Errorf("Snapshots must be sent back unchanged: %v", kube["Snapshots"])
						}
						if flags, _ := kube["Flags"].(map[string]interface{}); flags["IsServerMetricsDetected"] != true {
							return fmt.Errorf("Flags must be sent back unchanged: %v", kube["Flags"])
						}
						config, _ := kube["Configuration"].(map[string]interface{})
						if config["UseLoadBalancer"] != true || config["RestrictDefaultNamespace"] != true {
							return fmt.Errorf("configured switches not sent: %v", config)
						}
						if config["UseServerMetrics"] != true || config["ResourceOverCommitPercentage"] != float64(80) {
							return fmt.Errorf("unconfigured settings must be kept: %v", config)
						}
						if ingress, _ := config["IngressClasses"].([]interface{}); len(ingress) != 1 {
							return fmt.Errorf("IngressClasses must be kept: %v", config["IngressClasses"])
						}
						classes, _ := config["StorageClasses"].([]interface{})
						first, _ := classes[0].(map[string]interface{})
						if len(classes) != 1 || first["Name"] != "fast" || first["AllowVolumeExpansion"] != true || len(first["AccessModes"].([]interface{})) != 2 {
							return fmt.Errorf("StorageClasses: %v", config["StorageClasses"])
						}
						return nil
					}),
				),
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(environmentConfigConfig, managed+"\n  resource_over_commit_percentage = 50\n  enable_resource_over_commit = false\n"),
				Check: a2Check(func() error {
					var body map[string]interface{}
					a2LastBody(t, mock, "PUT", "/endpoints/9", &body)
					config := body["Kubernetes"].(map[string]interface{})["Configuration"].(map[string]interface{})
					if config["ResourceOverCommitPercentage"] != float64(50) || config["EnableResourceOverCommit"] != false {
						return fmt.Errorf("update payload: %v", config)
					}
					return nil
				}),
			},
			{
				ResourceName:      "portainer_kubernetes_environment_config.c",
				ImportState:       true,
				ImportStateVerify: true,
			},
		},
	})
	if n := a2Count(mock, "DELETE", "/endpoints/9"); n != 0 {
		t.Errorf("destroy must only forget the configuration, got %d DELETE", n)
	}
}

// TestKubernetesEnvironmentConfig_StorageClassesLeftOut: without
// storage_class the classes configured in Portainer are neither sent nor
// planned away.
func TestKubernetesEnvironmentConfig_StorageClassesLeftOut(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	s := fakeKubernetesEndpoint(mock, true)
	config := mockProviderConfig(mock) + fmt.Sprintf(environmentConfigConfig, "use_load_balancer = true")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_environment_config.c", "storage_class.#", "0"),
					a2Check(func() error {
						v, _ := s.get("endpoint")
						classes := v.(map[string]interface{})["Kubernetes"].(map[string]interface{})["Configuration"].(map[string]interface{})["StorageClasses"].([]interface{})
						if len(classes) != 1 {
							return fmt.Errorf("the configured classes must be kept: %v", classes)
						}
						return nil
					}),
				),
			},
			{Config: config, PlanOnly: true},
			{
				// A setting changed in Portainer is planned back.
				PreConfig: func() {
					v, _ := s.get("endpoint")
					v.(map[string]interface{})["Kubernetes"].(map[string]interface{})["Configuration"].(map[string]interface{})["UseLoadBalancer"] = false
					s.put("endpoint", v)
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				PreConfig:          func() { s.del("endpoint") },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestKubernetesEnvironmentConfig_Errors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/9", RespondJSON(http.StatusNotFound, map[string]string{"message": "not found"}))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:      mockProviderConfig(mock) + fmt.Sprintf(environmentConfigConfig, ""),
				ExpectError: regexp.MustCompile(`environment 9 not found`),
			},
			{
				Config:      mockProviderConfig(mock) + fmt.Sprintf(environmentConfigConfig, "resource_over_commit_percentage = 0"),
				ExpectError: regexp.MustCompile(`between 1 and 100`),
			},
			{
				Config:        mockProviderConfig(mock) + fmt.Sprintf(environmentConfigConfig, ""),
				ResourceName:  "portainer_kubernetes_environment_config.c",
				ImportState:   true,
				ImportStateId: "prod",
				ExpectError:   regexp.MustCompile(`expected <endpoint_id>`),
			},
		},
	})
}

func TestKubernetesEnvironmentConfig_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fakeKubernetesEndpoint(mock, false)
	a2Upgrade(t, mock,
		fmt.Sprintf(environmentConfigConfig, "use_load_balancer = true"),
		fmt.Sprintf(environmentConfigConfig, `
  restrict_default_namespace = true
  storage_class {
    name        = "fast"
    provisioner = "ebs.csi.aws.com"
  }
  storage_class {
    name                   = "slow"
    provisioner            = "nfs"
    access_modes           = ["RWX"]
    allow_volume_expansion = true
  }
`),
	)
}

// TestKubernetesEnvironmentConfig_UpgradeStorageClassesLeftOut documents the
// one difference from v3.0.0: SDKv2 kept the storage classes configured in
// Portainer in state as a computed block when storage_class was left out. A
// block cannot be computed in the framework, so the first plan after the
// upgrade removes them from state, in place; nothing is sent for them, and
// the plans after it are empty.
func TestKubernetesEnvironmentConfig_UpgradeStorageClassesLeftOut(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	s := fakeKubernetesEndpoint(mock, true)
	config := mockProviderConfig(mock) + fmt.Sprintf(environmentConfigConfig, "use_load_balancer = true")
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_kubernetes_environment_config.c", plancheck.ResourceActionUpdate),
				}},
				Check: a2Check(func() error {
					v, _ := s.get("endpoint")
					classes := v.(map[string]interface{})["Kubernetes"].(map[string]interface{})["Configuration"].(map[string]interface{})["StorageClasses"].([]interface{})
					if len(classes) != 1 {
						return fmt.Errorf("the configured classes must be kept: %v", classes)
					}
					return nil
				}),
			},
			{ProtoV5ProviderFactories: testProtoV5Factories, Config: config, PlanOnly: true},
		},
	})
}
