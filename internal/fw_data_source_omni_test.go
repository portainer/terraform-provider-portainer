package internal

import (
	"fmt"
	"net/http"
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// respondOmniMachine is the machine shape both the list and the inspect
// endpoint return.
func respondOmniMachine() map[string]interface{} {
	return map[string]interface{}{
		"machineName": "cp-1",
		"labels":      map[string]interface{}{"rack": "a1"},
		"spec": map[string]interface{}{
			"cluster": "edge-fleet", "connected": true, "maintenance": false,
			"management_address": "10.0.0.11", "power_state": "on",
			"role": "controlplane", "talos_version": "v1.8.0", "last_error": "",
			"hardware": map[string]interface{}{
				"arch": "amd64",
				"blockdevices": []map[string]interface{}{
					{"linux_name": "/dev/sda", "model": "SAMSUNG", "size": 512110190592, "type": "disk", "system_disk": true},
					{"linux_name": "/dev/sdb", "model": "SAMSUNG", "size": 1024220381184, "type": "disk", "system_disk": false},
				},
				"memory_modules": []map[string]interface{}{
					{"description": "DIMM DDR4", "size_mb": 32768},
				},
				"processors": []map[string]interface{}{
					{"description": "Xeon", "manufacturer": "Intel", "core_count": 8, "thread_count": 16, "frequency": 3200},
				},
			},
			"network": map[string]interface{}{
				"hostname": "cp-1", "domainname": "example.com",
				"addresses": []string{"10.0.0.11/24"}, "default_gateways": []string{"10.0.0.1"},
				"network_links": []map[string]interface{}{
					{"linux_name": "eth0", "hardware_address": "aa:bb:cc:dd:ee:ff", "link_up": true, "speed_mbps": 1000},
				},
			},
			"platform_metadata": map[string]interface{}{
				"platform": "metal", "hostname": "cp-1", "instance_id": "i-123",
				"instance_type": "bare", "region": "eu-central", "zone": "a",
			},
		},
	}
}

func omniDataSourceTest(t *testing.T, mock *MockServer, config string, check resource.TestCheckFunc) {
	t.Helper()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps:                    []resource.TestStep{{Config: mockProviderConfig(mock) + config, Check: check}},
	})
}

// TestDataSourceOmniMachines_ListsSummary pins the list view, which is
// deliberately the summary: you list to find a machine, then inspect it.
func TestDataSourceOmniMachines_ListsSummary(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/omni/3/machines", RespondJSON(http.StatusOK, []interface{}{
		respondOmniMachine(),
	}))

	const ds = "data.portainer_omni_machines.m"
	omniDataSourceTest(t, mock, `data "portainer_omni_machines" "m" { credential_id = 3 }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "id", "portainer-omni-machines-3"),
		resource.TestCheckResourceAttr(ds, "machines.#", "1"),
		resource.TestCheckResourceAttr(ds, "machines.0.machine_name", "cp-1"),
		resource.TestCheckResourceAttr(ds, "machines.0.cluster", "edge-fleet"),
		resource.TestCheckResourceAttr(ds, "machines.0.role", "controlplane"),
		resource.TestCheckResourceAttr(ds, "machines.0.talos_version", "v1.8.0"),
		resource.TestCheckResourceAttr(ds, "machines.0.connected", "true"),
		resource.TestCheckResourceAttr(ds, "machines.0.labels.rack", "a1"),
	))
}

// TestDataSourceOmniMachine_UnpacksHardware covers the inspect view, whose
// whole purpose is the hardware and network detail a cluster machine block
// needs: which disk to install on, which interface to configure.
func TestDataSourceOmniMachine_UnpacksHardware(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/omni/3/machine/cp-1", RespondJSON(http.StatusOK, respondOmniMachine()))

	const ds = "data.portainer_omni_machine.m"
	omniDataSourceTest(t, mock, `data "portainer_omni_machine" "m" {
  credential_id = 3
  machine_name  = "cp-1"
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "id", "portainer-omni-machine-3-cp-1"),
		resource.TestCheckResourceAttr(ds, "architecture", "amd64"),
		resource.TestCheckResourceAttr(ds, "platform", "metal"),
		resource.TestCheckResourceAttr(ds, "cluster", "edge-fleet"),
		resource.TestCheckResourceAttr(ds, "labels.rack", "a1"),
		resource.TestCheckResourceAttr(ds, "domain_name", "example.com"),
		resource.TestCheckResourceAttr(ds, "addresses.#", "1"),
		resource.TestCheckResourceAttr(ds, "addresses.0", "10.0.0.11/24"),
		resource.TestCheckResourceAttr(ds, "default_gateways.0", "10.0.0.1"),
		resource.TestCheckResourceAttr(ds, "block_devices.#", "2"),
		resource.TestCheckResourceAttr(ds, "block_devices.0.linux_name", "/dev/sda"),
		resource.TestCheckResourceAttr(ds, "block_devices.0.system_disk", "true"),
		resource.TestCheckResourceAttr(ds, "block_devices.0.size", "512110190592"),
		resource.TestCheckResourceAttr(ds, "block_devices.1.system_disk", "false"),
		resource.TestCheckResourceAttr(ds, "memory_modules.#", "1"),
		resource.TestCheckResourceAttr(ds, "memory_modules.0.size_mb", "32768"),
		resource.TestCheckResourceAttr(ds, "processors.#", "1"),
		resource.TestCheckResourceAttr(ds, "processors.0.core_count", "8"),
		resource.TestCheckResourceAttr(ds, "network_links.#", "1"),
		resource.TestCheckResourceAttr(ds, "network_links.0.linux_name", "eth0"),
		resource.TestCheckResourceAttr(ds, "network_links.0.link_up", "true"),
		resource.TestCheckResourceAttr(ds, "network_links.0.speed_mbps", "1000"),
	))
}

// TestDataSourceOmniMachine_ErrorIsReported: a missing machine fails the read.
func TestDataSourceOmniMachine_ErrorIsReported(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_omni_machine" "m" {
  credential_id = 3
  machine_name  = "cp-9"
}`,
			ExpectError: regexp.MustCompile(`failed to read Omni machine cp-9`),
		}},
	})
}

// TestDataSourceOmniMachineLogs_ReadsRawBody pins the one endpoint in this
// block that answers with text rather than a JSON document.
func TestDataSourceOmniMachineLogs_ReadsRawBody(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/omni/3/machine/cp-1/logs", RespondString(http.StatusOK,
		"text/plain", "boot: ok\nkubelet: started\n"))

	omniDataSourceTest(t, mock, `data "portainer_omni_machine_logs" "l" {
  credential_id = 3
  machine_name  = "cp-1"
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_omni_machine_logs.l", "logs", "boot: ok\nkubelet: started\n"),
		resource.TestCheckResourceAttr("data.portainer_omni_machine_logs.l", "id", "portainer-omni-machine-logs-3-cp-1"),
	))
}

// TestDataSourceOmniTalosVersions_SortsKeys covers the map Portainer answers
// with. Go iterates a map in a random order, so the provider sorts - otherwise
// the data source's output would differ between two identical plans.
func TestDataSourceOmniTalosVersions_SortsKeys(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/omni/3/cluster/version/talos", RespondJSON(http.StatusOK, map[string]interface{}{
		"v1.9.0": []string{"v1.32.0"},
		"v1.7.0": []string{"v1.30.0", "v1.29.0"},
		"v1.8.0": []string{"v1.31.0"},
	}))

	const ds = "data.portainer_omni_talos_versions.v"
	omniDataSourceTest(t, mock, `data "portainer_omni_talos_versions" "v" { credential_id = 3 }`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "id", "portainer-omni-talos-versions-3"),
		resource.TestCheckResourceAttr(ds, "talos_versions.#", "3"),
		resource.TestCheckResourceAttr(ds, "talos_versions.0", "v1.7.0"),
		resource.TestCheckResourceAttr(ds, "talos_versions.1", "v1.8.0"),
		resource.TestCheckResourceAttr(ds, "talos_versions.2", "v1.9.0"),
		// The compatibility list must follow the same order.
		resource.TestCheckResourceAttr(ds, "compatibility.0.talos_version", "v1.7.0"),
		resource.TestCheckResourceAttr(ds, "compatibility.0.kubernetes_versions.#", "2"),
		resource.TestCheckResourceAttr(ds, "compatibility.2.kubernetes_versions.0", "v1.32.0"),
	))
}

// TestDataSourceOmniUpgradeStatus_MapsComponentToPath pins the path spelling:
// Portainer calls the Kubernetes half "k8s" even though the argument does not.
func TestDataSourceOmniUpgradeStatus_MapsComponentToPath(t *testing.T) {
	t.Parallel()
	for component, path := range map[string]string{
		"kubernetes": "/omni/3/cluster/edge-fleet/upgrade/status/k8s",
		"talos":      "/omni/3/cluster/edge-fleet/upgrade/status/talos",
	} {
		t.Run(component, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("GET", path, RespondJSON(http.StatusOK, map[string]interface{}{
				"status": "upgrading", "step": "draining", "phase": 2, "error": "",
				"current_upgrade_version": "v1.31.0", "last_upgrade_version": "v1.30.0",
				"upgrade_versions": []string{"v1.31.0", "v1.32.0"},
			}))

			const ds = "data.portainer_omni_upgrade_status.s"
			omniDataSourceTest(t, mock, fmt.Sprintf(`data "portainer_omni_upgrade_status" "s" {
  credential_id = 3
  cluster       = "edge-fleet"
  component     = %q
}`, component), resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr(ds, "id", "portainer-omni-upgrade-status-3-edge-fleet-"+component),
				resource.TestCheckResourceAttr(ds, "step", "draining"),
				resource.TestCheckResourceAttr(ds, "phase", "2"),
				resource.TestCheckResourceAttr(ds, "current_version", "v1.31.0"),
				resource.TestCheckResourceAttr(ds, "available_versions.#", "2"),
				func(*terraform.State) error {
					if mock.FindRequest("GET", path) == nil {
						return fmt.Errorf("expected the request to go to %s", path)
					}
					return nil
				},
			))
		})
	}
}

func TestDataSourceOmniUpgradeStatus_RejectsUnknownComponent(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + `data "portainer_omni_upgrade_status" "s" {
  credential_id = 3
  cluster       = "edge-fleet"
  component     = "etcd"
}`,
			ExpectError: regexp.MustCompile(`kubernetes`),
		}},
	})
}

func omniServiceAccountHCL(key, failOnError string) string {
	return fmt.Sprintf(`data "portainer_omni_service_account" "s" {
  endpoint            = "https://omni.example.com"
  service_account_key = %q
  %s
}`, key, failOnError)
}

// TestDataSourceOmniServiceAccount_FailsByDefault covers the default that
// stops a bad credential before a cluster resource tries to use it.
func TestDataSourceOmniServiceAccount_FailsByDefault(t *testing.T) {
	t.Parallel()
	for name, failOnError := range map[string]string{
		"unset": "",
		"true":  "fail_on_error = true",
	} {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("GET", "/omni/serviceaccount/validate", RespondString(http.StatusUnauthorized,
				"application/json", `{"message":"invalid service account key"}`))
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{{
					Config:      mockProviderConfig(mock) + omniServiceAccountHCL("bad-key", failOnError),
					ExpectError: regexp.MustCompile(`fail_on_error`),
				}},
			})
		})
	}
}

// TestDataSourceOmniServiceAccount_CanReportInstead covers the other side of
// that switch, and the query the endpoint takes.
func TestDataSourceOmniServiceAccount_CanReportInstead(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/omni/serviceaccount/validate", RespondString(http.StatusUnauthorized,
		"application/json", `{"message":"invalid service account key"}`))

	const ds = "data.portainer_omni_service_account.s"
	omniDataSourceTest(t, mock, omniServiceAccountHCL("bad-key", "fail_on_error = false"), resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "valid", "false"),
		resource.TestMatchResourceAttr(ds, "error", regexp.MustCompile(`invalid service account key`)),
		func(*terraform.State) error {
			req := mock.FindRequest("GET", "/omni/serviceaccount/validate")
			if req == nil {
				return fmt.Errorf("no validation request was sent")
			}
			if !strings.Contains(req.Query, "serviceAccountKey=bad-key") {
				return fmt.Errorf("expected the key in the query, got %q", req.Query)
			}
			if !strings.Contains(req.Query, "endpoint=https") {
				return fmt.Errorf("expected the endpoint in the query, got %q", req.Query)
			}
			return nil
		},
	))
}

// TestDataSourceOmniServiceAccount_Accepts covers the happy path.
func TestDataSourceOmniServiceAccount_Accepts(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/omni/serviceaccount/validate", RespondJSON(http.StatusOK, map[string]interface{}{}))

	const ds = "data.portainer_omni_service_account.s"
	omniDataSourceTest(t, mock, omniServiceAccountHCL("good-key", ""), resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr(ds, "id", "portainer-omni-service-account"),
		resource.TestCheckResourceAttr(ds, "valid", "true"),
		resource.TestCheckResourceAttr(ds, "error", ""),
	))
}

// TestDataSourceOmniServiceAccount_NeverLeaksTheKey is the regression guard
// for the credential leak a review caught. The endpoint takes the service
// account key as a query parameter, and an *apiStatusError renders the whole
// request URL - so reporting the error verbatim put the key into the `error`
// attribute (and therefore state), and into the diagnostic printed to the
// console on the default path.
func TestDataSourceOmniServiceAccount_NeverLeaksTheKey(t *testing.T) {
	t.Parallel()
	// "~" is not escaped in a query string, so the key would show up as is.
	const key = "super~secret~omni~key"

	t.Run("not in state when the result is reported", func(t *testing.T) {
		mock := NewMockServer(t)
		mock.On("GET", "/omni/serviceaccount/validate", RespondString(http.StatusUnauthorized,
			"application/json", `{"message":"invalid service account key"}`))

		omniDataSourceTest(t, mock, omniServiceAccountHCL(key, "fail_on_error = false"), func(s *terraform.State) error {
			got := s.RootModule().Resources["data.portainer_omni_service_account.s"].Primary.Attributes["error"]
			if strings.Contains(got, key) {
				return fmt.Errorf("the service account key must never reach state, got %q", got)
			}
			if !strings.Contains(got, "invalid service account key") {
				return fmt.Errorf("what Omni said must still be reported, got %q", got)
			}
			return nil
		})
	})

	t.Run("not in the diagnostic on the default path", func(t *testing.T) {
		mock := NewMockServer(t)
		mock.On("GET", "/omni/serviceaccount/validate", RespondString(http.StatusUnauthorized,
			"application/json", `{"message":"invalid service account key"}`))

		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{{
				Config: mockProviderConfig(mock) + omniServiceAccountHCL(key, ""),
				// The rejection is reported, and no "~" - so no key - appears in it.
				ExpectError: regexp.MustCompile(`(?s)\A[^~]*rejected[^~]*\z`),
			}},
		})
	})
}

// TestAccOmniTalosVersions needs a Business Edition instance with an Omni
// cloud credential, which no CI instance has.
func TestAccOmniTalosVersions(t *testing.T) {
	if os.Getenv("PORTAINER_ACC_BE") != "1" || os.Getenv("PORTAINER_ACC_OMNI_CREDENTIAL_ID") == "" {
		t.Skip("needs Portainer BE (PORTAINER_ACC_BE=1) and an Omni cloud credential (PORTAINER_ACC_OMNI_CREDENTIAL_ID)")
	}
	resource.Test(t, resource.TestCase{
		PreCheck:                 func() { accPreCheck(t) },
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: accProvider() + fmt.Sprintf(`data "portainer_omni_talos_versions" "v" { credential_id = %s }`, os.Getenv("PORTAINER_ACC_OMNI_CREDENTIAL_ID")),
			Check:  resource.TestCheckResourceAttrSet("data.portainer_omni_talos_versions.v", "talos_versions.#"),
		}},
	})
}
