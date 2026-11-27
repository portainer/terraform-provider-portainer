package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// =========================================================================
// Omni / Talos clusters (Business Edition).
// =========================================================================

// fakeOmniCluster is a stand-in for Portainer's Omni cluster API: the cluster
// exists from its creation until its deletion.
type fakeOmniCluster struct {
	mu      sync.Mutex
	exists  bool
	deletes int
}

func (f *fakeOmniCluster) register(m *MockServer, credential, name string) {
	m.On("POST", "/omni/"+credential+"/cluster/validate", RespondJSON(http.StatusOK, map[string]interface{}{}))
	m.On("POST", "/omni/"+credential+"/cluster/create", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.exists = true
		f.mu.Unlock()
		RespondJSON(http.StatusOK, map[string]interface{}{"Id": 12})(w, r)
	})
	m.On("PUT", "/omni/"+credential+"/cluster/update", RespondJSON(http.StatusOK, map[string]interface{}{}))
	m.On("GET", "/omni/"+credential+"/cluster/"+name, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		exists := f.exists
		f.mu.Unlock()
		if !exists {
			RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`)(w, r)
			return
		}
		respondOmniCluster()(w, r)
	})
	m.On("DELETE", "/omni/"+credential+"/cluster/delete", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.exists = false
		f.deletes++
		f.mu.Unlock()
		RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
	})
}

func respondOmniCluster() http.HandlerFunc {
	return RespondJSON(http.StatusOK, map[string]interface{}{
		"metadata": map[string]interface{}{
			"kubernetes_version": "v1.31.0",
			"talos_version":      "v1.8.0",
			"features": map[string]interface{}{
				"disk_encryption": true, "enable_workload_proxy": true,
				"use_embedded_discovery_service": false,
			},
			"backup_configuration": map[string]interface{}{"enabled": true, "interval": "1h"},
		},
		"status": map[string]interface{}{
			"available": true, "ready": true, "phase": 3,
			"controlplaneReady": true, "kubernetesAPIReady": true,
			"machines": map[string]interface{}{
				"total": 2, "requested": 2, "connected": 2, "healthy": 2,
			},
		},
	})
}

func omniMachineHCL(name string) string {
	return fmt.Sprintf(`
    machine {
      name             = %q
      hostname         = %q
      install_disk     = "/dev/sda"
      nameservers      = ["1.1.1.1"]
      system_disk_size = 50
      user_disk {
        volume_name = "data"
        size        = 0
      }
      interface {
        interface = "eth0"
        addresses = ["10.0.0.10/24"]
        route {
          network = "0.0.0.0/0"
          gateway = "10.0.0.1"
        }
      }
    }`, name, name)
}

func omniClusterHCL(validate string, workers ...string) string {
	workerMachines := ""
	for _, w := range workers {
		workerMachines += omniMachineHCL(w)
	}
	return fmt.Sprintf(`
resource "portainer_omni_cluster" "c" {
  credential_id          = 3
  name                   = "edge-fleet"
  talos_version          = "v1.8.0"
  kubernetes_version     = "v1.31.0"
  validate_before_create = %s
  control_plane {%s
  }
  worker {
    name = "pool-a"%s
  }
}
`, validate, omniMachineHCL("cp-1"), workerMachines)
}

// TestOmniCluster_ValidatesBeforeProvisioning is the reason the validation
// call is on by default: provisioning touches real machines and takes a long
// time, so a rejected configuration must be caught before any of that starts.
func TestOmniCluster_ValidatesBeforeProvisioning(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("POST", "/omni/3/cluster/validate", RespondString(http.StatusBadRequest,
		"application/json", `{"message":"control plane must have an odd number of machines"}`))
	mock.On("POST", "/omni/3/cluster/create", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 12}))

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      mockProviderConfig(mock) + omniClusterHCL("true", "worker-1"),
			ExpectError: regexp.MustCompile(`validate_before_create`),
		}},
	})
	if mock.FindRequest("POST", "/omni/3/cluster/create") != nil {
		t.Error("no cluster must be created after a failed validation")
	}
}

// TestOmniCluster_ValidationCanBeSkipped covers the escape hatch.
func TestOmniCluster_ValidationCanBeSkipped(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	(&fakeOmniCluster{}).register(mock, "3", "edge-fleet")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + omniClusterHCL("false", "worker-1"),
		}},
	})
	if mock.FindRequest("POST", "/omni/3/cluster/validate") != nil {
		t.Error("validation must be skipped when it is turned off")
	}
}

// TestOmniCluster_CreateSendsMachineSpec pins the payload, which is the part
// of this resource with real translation in it: flat Terraform blocks become
// Omni's nested machine specification. Then the read: Portainer reports the
// cluster's spec and status but not the machines it was built from, so the
// configured blocks must survive it (empty plan), and the import.
func TestOmniCluster_CreateSendsMachineSpec(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := &fakeOmniCluster{}
	fake.register(mock, "3", "edge-fleet")
	config := mockProviderConfig(mock) + omniClusterHCL("true", "worker-1")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_omni_cluster.c", "id", "3:edge-fleet"),
					resource.TestCheckResourceAttr("portainer_omni_cluster.c", "endpoint_id", "12"),
					resource.TestCheckResourceAttr("portainer_omni_cluster.c", "ready", "true"),
					resource.TestCheckResourceAttr("portainer_omni_cluster.c", "machines_healthy", "2"),
					resource.TestCheckResourceAttr("portainer_omni_cluster.c", "backup_interval", "1h"),
					resource.TestCheckResourceAttr("portainer_omni_cluster.c", "control_plane.0.machine.0.name", "cp-1"),
					func(*terraform.State) error { return checkOmniCreatePayload(mock) },
				),
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				ResourceName:      "portainer_omni_cluster.c",
				ImportState:       true,
				ImportStateVerify: true,
				// Configuration-only or only known from the create: never read back.
				ImportStateVerifyIgnore: []string{"control_plane", "worker", "validate_before_create", "endpoint_id"},
			},
		},
		CheckDestroy: func(*terraform.State) error {
			req := mock.FindRequest("DELETE", "/omni/3/cluster/delete")
			if req == nil || !strings.Contains(req.Query, "name=edge-fleet") {
				return fmt.Errorf("expected the cluster name in the delete query, got %+v", req)
			}
			return nil
		},
	})
}

func checkOmniCreatePayload(mock *MockServer) error {
	var payload struct {
		Cluster struct {
			Name       string            `json:"name"`
			Kubernetes map[string]string `json:"kubernetes"`
			Talos      map[string]string `json:"talos"`
		} `json:"cluster"`
		ControlPlane struct {
			Machines []map[string]interface{} `json:"machines"`
		} `json:"controlPlane"`
		Worker struct {
			Name     string                   `json:"name"`
			Machines []map[string]interface{} `json:"machines"`
		} `json:"worker"`
	}
	req := mock.FindRequest("POST", "/omni/3/cluster/create")
	if req == nil {
		return fmt.Errorf("no create request")
	}
	if err := req.DecodeJSON(&payload); err != nil {
		return fmt.Errorf("payload is not JSON: %w", err)
	}
	if payload.Cluster.Name != "edge-fleet" {
		return fmt.Errorf("cluster name: got %q", payload.Cluster.Name)
	}
	if payload.Cluster.Talos["version"] != "v1.8.0" || payload.Cluster.Kubernetes["version"] != "v1.31.0" {
		return fmt.Errorf("the versions must be nested under their own objects: %+v", payload.Cluster)
	}
	if len(payload.ControlPlane.Machines) != 1 {
		return fmt.Errorf("expected one control plane machine, got %d", len(payload.ControlPlane.Machines))
	}
	machine := payload.ControlPlane.Machines[0]
	if machine["name"] != "cp-1" {
		return fmt.Errorf("machine name: got %v", machine["name"])
	}
	if _, ok := machine["kind"]; ok {
		return fmt.Errorf("an unset kind must not be sent: %v", machine)
	}
	install, ok := machine["install"].(map[string]interface{})
	if !ok || install["disk"] != "/dev/sda" {
		return fmt.Errorf("install_disk must be nested under install: %v", machine["install"])
	}
	if machine["systemDiskSize"] != float64(50) {
		return fmt.Errorf("systemDiskSize: got %v", machine["systemDiskSize"])
	}
	// The one numeric field where zero is meaningful: Omni reads it as
	// "grow into the remaining space", so it is always sent.
	disk, ok := machine["userDisk"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("the user disk must be sent, got %v", machine["userDisk"])
	}
	if size, ok := disk["size"]; !ok || size != float64(0) {
		return fmt.Errorf("a zero user disk size must be sent, got %v", disk)
	}
	interfaces, ok := machine["interfaces"].([]interface{})
	if !ok || len(interfaces) != 1 {
		return fmt.Errorf("expected one interface, got %v", machine["interfaces"])
	}
	iface := interfaces[0].(map[string]interface{})
	if _, ok := iface["dhcp"]; ok {
		return fmt.Errorf("dhcp = false must not be sent: %v", iface)
	}
	routes, ok := iface["routes"].([]interface{})
	if !ok || len(routes) != 1 {
		return fmt.Errorf("expected one route, got %v", iface["routes"])
	}
	if routes[0].(map[string]interface{})["gateway"] != "10.0.0.1" {
		return fmt.Errorf("the route was not sent: %v", routes[0])
	}
	if payload.Worker.Name != "pool-a" || len(payload.Worker.Machines) != 1 {
		return fmt.Errorf("the worker pool was not sent: %+v", payload.Worker)
	}
	return nil
}

// TestOmniCluster_UpdateSendsMachineDelta covers the translation the update
// endpoint forces: it is imperative - machines to add, names to remove - so
// the declarative machine list has to be turned into that difference.
func TestOmniCluster_UpdateSendsMachineDelta(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	(&fakeOmniCluster{}).register(mock, "3", "edge-fleet")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + omniClusterHCL("true", "worker-1", "worker-2")},
			{
				Config: mockProviderConfig(mock) + omniClusterHCL("true", "worker-1", "worker-3"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_omni_cluster.c", plancheck.ResourceActionUpdate)},
				},
				Check: func(*terraform.State) error {
					var payload struct {
						Cluster struct {
							Talos map[string]string `json:"talos"`
						} `json:"cluster"`
						WorkersToAdd struct {
							Name     string                   `json:"name"`
							Machines []map[string]interface{} `json:"machines"`
						} `json:"workersToAdd"`
						WorkersToRemove       []string        `json:"workersToRemove"`
						ControlPlanesToAdd    json.RawMessage `json:"controlPlanesToAdd"`
						ControlPlanesToRemove []string        `json:"controlPlanesToRemove"`
					}
					req := mock.FindRequest("PUT", "/omni/3/cluster/update")
					if req == nil {
						return fmt.Errorf("no update request")
					}
					if err := req.DecodeJSON(&payload); err != nil {
						return err
					}
					if len(payload.WorkersToAdd.Machines) != 1 || payload.WorkersToAdd.Machines[0]["name"] != "worker-3" {
						return fmt.Errorf("only the new machine must be added, got %v", payload.WorkersToAdd.Machines)
					}
					if payload.WorkersToAdd.Name != "pool-a" {
						return fmt.Errorf("the pool name must be sent with the added machines, got %q", payload.WorkersToAdd.Name)
					}
					if len(payload.WorkersToRemove) != 1 || payload.WorkersToRemove[0] != "worker-2" {
						return fmt.Errorf("only the dropped machine must be removed, got %v", payload.WorkersToRemove)
					}
					if payload.ControlPlanesToAdd != nil || payload.ControlPlanesToRemove != nil {
						return fmt.Errorf("the unchanged control plane must not be sent")
					}
					if payload.Cluster.Talos["version"] != "v1.8.0" {
						return fmt.Errorf("the cluster spec must be sent with the update: %+v", payload.Cluster)
					}
					return nil
				},
			},
		},
	})
}

// TestOmniCluster_ImportIDs pins the import ID: "<credential>:<name>", the
// "/" form from before v3.0.0 (moved to ":" on read), and a malformed one.
func TestOmniCluster_ImportIDs(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	(&fakeOmniCluster{exists: true}).register(mock, "3", "edge-fleet")
	config := mockProviderConfig(mock) + `
resource "portainer_omni_cluster" "c" {
  credential_id = 3
  name          = "edge-fleet"
}
`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:        config,
				ResourceName:  "portainer_omni_cluster.c",
				ImportState:   true,
				ImportStateId: "edge-fleet",
				ExpectError:   regexp.MustCompile(`<credential id>:<cluster name>`),
			},
			{
				Config:        config,
				ResourceName:  "portainer_omni_cluster.c",
				ImportState:   true,
				ImportStateId: "3/edge-fleet",
				ImportStateCheck: func(states []*terraform.InstanceState) error {
					if len(states) != 1 {
						return fmt.Errorf("expected one imported resource, got %d", len(states))
					}
					s := states[0]
					if s.ID != "3:edge-fleet" || s.Attributes["credential_id"] != "3" || s.Attributes["name"] != "edge-fleet" {
						return fmt.Errorf("the legacy ID must be moved to the current format: %v", s.Attributes)
					}
					return nil
				},
			},
		},
	})
}

// TestOmniCluster_GoneOutsideTerraform: a cluster deleted in Omni is planned
// for creation again.
func TestOmniCluster_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := &fakeOmniCluster{}
	fake.register(mock, "3", "edge-fleet")
	config := mockProviderConfig(mock) + omniClusterHCL("true", "worker-1")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.exists = false
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestOmniCluster_InlinePatchMustBeJSONObject keeps validateJSONObject.
func TestOmniCluster_InlinePatchMustBeJSONObject(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `
resource "portainer_omni_cluster" "c" {
  credential_id = 3
  name          = "edge-fleet"
  cluster_patch {
    inline = "[1, 2]"
  }
}
`,
			ExpectError: regexp.MustCompile(`must be a JSON object`),
		}},
	})
}

// TestOmniCluster_UpgradeFromV300: clusters created by v3.0.0 (SDKv2) against
// the same fake plan no change with this build - one with every block
// argument set, one that leaves the optional ones out (SDKv2 stored "", 0 and
// false for them inside the blocks).
func TestOmniCluster_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	for name, hcl := range map[string]string{
		"full": omniClusterHCL("true", "worker-1") + `
resource "portainer_omni_cluster" "d" {
  credential_id         = 4
  name                  = "edge-fleet"
  kind                  = "Cluster"
  labels                = { env = "prod" }
  cluster_config        = "kind: Cluster"
  portainer_url         = "https://portainer.example.com"
  tunnel_server_address = "portainer.example.com:8000"
  cluster_patch {
    id_override     = "p1"
    annotation_name = "patch one"
    inline          = jsonencode({ machine = { network = {} } })
  }
  control_plane {
    kind = "ControlPlane"
    machine {
      name = "cp-1"
      kind = "Machine"
      user_disk {
        volume_name = "data"
        size        = 20
      }
      interface {
        interface = "eth0"
        dhcp      = true
      }
      patch {
        id_override = "mp"
        inline      = jsonencode({ a = 1 })
      }
    }
  }
}
`,
		"minimal": `
resource "portainer_omni_cluster" "c" {
  credential_id = 3
  name          = "edge-fleet"
  control_plane {
    machine {
      name = "cp-1"
      user_disk { volume_name = "data" }
      interface { interface = "eth0" }
      patch {}
    }
  }
  worker {
    machine { name = "worker-1" }
  }
  cluster_patch {}
}
`,
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			(&fakeOmniCluster{}).register(mock, "3", "edge-fleet")
			(&fakeOmniCluster{}).register(mock, "4", "edge-fleet")
			config := mockProviderConfig(mock) + hcl

			resource.Test(t, resource.TestCase{
				IsUnitTest: true,
				Steps: []resource.TestStep{
					{ExternalProviders: previousRelease, Config: config},
					{
						ProtoV5ProviderFactories: testProtoV5Factories,
						Config:                   config,
						ConfigPlanChecks: resource.ConfigPlanChecks{
							PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
						},
					},
				},
			})
		})
	}
}

// TestOmniCluster_LegacyIDMovedOnRead: a state still holding the "/" ID
// of releases before v3.0.0 is moved to ":" by the read, without a plan.
func TestOmniCluster_LegacyIDMovedOnRead(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	(&fakeOmniCluster{exists: true}).register(mock, "3", "edge-fleet")
	config := mockProviderConfig(mock) + `
resource "portainer_omni_cluster" "c" {
  credential_id = 3
  name          = "edge-fleet"
}
`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:             config,
				ResourceName:       "portainer_omni_cluster.c",
				ImportState:        true,
				ImportStateId:      "3/edge-fleet",
				ImportStatePersist: true,
			},
			{
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					// validate_before_create is only set by a create, as with SDKv2.
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_omni_cluster.c", plancheck.ResourceActionUpdate)},
				},
				Check: resource.TestCheckResourceAttr("portainer_omni_cluster.c", "id", "3:edge-fleet"),
			},
		},
	})
}
