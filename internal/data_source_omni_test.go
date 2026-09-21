package internal

import (
	"net/http"
	"strings"
	"testing"
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

// TestDataSourceOmniMachines_ListsSummary pins the list view, which is
// deliberately the summary: you list to find a machine, then inspect it.
func TestDataSourceOmniMachines_ListsSummary(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/omni/3/machines", RespondJSON(http.StatusOK, []interface{}{
		respondOmniMachine(),
	}))

	ds := dataSourceOmniMachines()
	d := ds.TestResourceData()
	_ = d.Set("credential_id", 3)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	machines := d.Get("machines").([]interface{})
	if len(machines) != 1 {
		t.Fatalf("expected one machine, got %d", len(machines))
	}
	machine := machines[0].(map[string]interface{})
	if machine["machine_name"] != "cp-1" || machine["cluster"] != "edge-fleet" {
		t.Errorf("the summary was not read: %+v", machine)
	}
	if machine["role"] != "controlplane" || machine["talos_version"] != "v1.8.0" {
		t.Errorf("the role and version were not read: %+v", machine)
	}
	labels := machine["labels"].(map[string]interface{})
	if labels["rack"] != "a1" {
		t.Errorf("the labels were not read: %v", labels)
	}
}

// TestDataSourceOmniMachine_UnpacksHardware covers the inspect view, whose
// whole purpose is the hardware and network detail a cluster machine block
// needs: which disk to install on, which interface to configure.
func TestDataSourceOmniMachine_UnpacksHardware(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/omni/3/machine/cp-1", RespondJSON(http.StatusOK, respondOmniMachine()))

	ds := dataSourceOmniMachine()
	d := ds.TestResourceData()
	_ = d.Set("credential_id", 3)
	_ = d.Set("machine_name", "cp-1")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("architecture"); got != "amd64" {
		t.Errorf("architecture: got %v", got)
	}
	if got := d.Get("platform"); got != "metal" {
		t.Errorf("platform: got %v", got)
	}
	if got := d.Get("addresses").([]interface{}); len(got) != 1 || got[0] != "10.0.0.11/24" {
		t.Errorf("addresses: got %v", got)
	}

	devices := d.Get("block_devices").([]interface{})
	if len(devices) != 2 {
		t.Fatalf("expected two block devices, got %d", len(devices))
	}
	system := devices[0].(map[string]interface{})
	if system["linux_name"] != "/dev/sda" || system["system_disk"] != true {
		t.Errorf("the system disk was not read: %+v", system)
	}
	if devices[1].(map[string]interface{})["system_disk"] != false {
		t.Errorf("the second device must not be flagged as the system disk: %+v", devices[1])
	}

	modules := d.Get("memory_modules").([]interface{})
	if len(modules) != 1 || modules[0].(map[string]interface{})["size_mb"] != 32768 {
		t.Errorf("the memory modules were not read: %v", modules)
	}

	processors := d.Get("processors").([]interface{})
	if len(processors) != 1 || processors[0].(map[string]interface{})["core_count"] != 8 {
		t.Errorf("the processors were not read: %v", processors)
	}

	links := d.Get("network_links").([]interface{})
	if len(links) != 1 {
		t.Fatalf("expected one network link, got %d", len(links))
	}
	link := links[0].(map[string]interface{})
	if link["linux_name"] != "eth0" || link["link_up"] != true || link["speed_mbps"] != 1000 {
		t.Errorf("the network link was not read: %+v", link)
	}
}

// TestDataSourceOmniMachineLogs_ReadsRawBody pins the one endpoint in this
// block that answers with text rather than a JSON document.
func TestDataSourceOmniMachineLogs_ReadsRawBody(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/omni/3/machine/cp-1/logs", RespondString(http.StatusOK,
		"text/plain", "boot: ok\nkubelet: started\n"))

	ds := dataSourceOmniMachineLogs()
	d := ds.TestResourceData()
	_ = d.Set("credential_id", 3)
	_ = d.Set("machine_name", "cp-1")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	logs := d.Get("logs").(string)
	if !strings.Contains(logs, "kubelet: started") {
		t.Errorf("the raw log text must be read through unchanged, got %q", logs)
	}
}

// TestDataSourceOmniTalosVersions_SortsKeys covers the map Portainer answers
// with. Go iterates a map in a random order, so the provider sorts - otherwise
// the data source's output would differ between two identical plans.
func TestDataSourceOmniTalosVersions_SortsKeys(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/omni/3/cluster/version/talos", RespondJSON(http.StatusOK, map[string]interface{}{
		"v1.9.0": []string{"v1.32.0"},
		"v1.7.0": []string{"v1.30.0", "v1.29.0"},
		"v1.8.0": []string{"v1.31.0"},
	}))

	ds := dataSourceOmniTalosVersions()
	d := ds.TestResourceData()
	_ = d.Set("credential_id", 3)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	versions := d.Get("talos_versions").([]interface{})
	want := []string{"v1.7.0", "v1.8.0", "v1.9.0"}
	if len(versions) != len(want) {
		t.Fatalf("expected %d versions, got %d", len(want), len(versions))
	}
	for i, version := range want {
		if versions[i] != version {
			t.Errorf("version %d: expected %s, got %v", i, version, versions[i])
		}
	}

	compatibility := d.Get("compatibility").([]interface{})
	first := compatibility[0].(map[string]interface{})
	if first["talos_version"] != "v1.7.0" {
		t.Errorf("the compatibility list must follow the same order, got %v", first["talos_version"])
	}
	if got := first["kubernetes_versions"].([]interface{}); len(got) != 2 {
		t.Errorf("the compatible versions were not read: %v", got)
	}
}

// TestDataSourceOmniUpgradeStatus_MapsComponentToPath pins the path spelling:
// Portainer calls the Kubernetes half "k8s" even though the argument does not.
func TestDataSourceOmniUpgradeStatus_MapsComponentToPath(t *testing.T) {
	for component, path := range map[string]string{
		"kubernetes": "/omni/3/cluster/edge-fleet/upgrade/status/k8s",
		"talos":      "/omni/3/cluster/edge-fleet/upgrade/status/talos",
	} {
		mock := NewMockServer(t)
		mock.On("GET", path, RespondJSON(http.StatusOK, map[string]interface{}{
			"status": "upgrading", "step": "draining", "phase": 2, "error": "",
			"current_upgrade_version": "v1.31.0", "last_upgrade_version": "v1.30.0",
			"upgrade_versions": []string{"v1.31.0", "v1.32.0"},
		}))

		ds := dataSourceOmniUpgradeStatus()
		d := ds.TestResourceData()
		_ = d.Set("credential_id", 3)
		_ = d.Set("cluster", "edge-fleet")
		_ = d.Set("component", component)

		if err := rcRead(ds, d, mock.Client()); err != nil {
			t.Fatalf("%s: Read failed: %v", component, err)
		}
		if mock.FindRequest("GET", path) == nil {
			t.Errorf("%s: expected the request to go to %s", component, path)
		}
		if got := d.Get("step"); got != "draining" {
			t.Errorf("%s: step: got %v", component, got)
		}
		if got := d.Get("available_versions").([]interface{}); len(got) != 2 {
			t.Errorf("%s: available_versions: got %v", component, got)
		}
	}
}

// TestDataSourceOmniServiceAccount_FailsByDefault covers the default that
// stops a bad credential before a cluster resource tries to use it.
func TestDataSourceOmniServiceAccount_FailsByDefault(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/omni/serviceaccount/validate", RespondString(http.StatusUnauthorized,
		"application/json", `{"message":"invalid service account key"}`))

	ds := dataSourceOmniServiceAccount()
	d := ds.TestResourceData()
	_ = d.Set("endpoint", "https://omni.example.com")
	_ = d.Set("service_account_key", "bad-key")
	_ = d.Set("fail_on_error", true)

	err := rcRead(ds, d, mock.Client())
	if err == nil {
		t.Fatal("a rejected credential must fail the plan by default")
	}
	if !strings.Contains(err.Error(), "fail_on_error") {
		t.Errorf("the error should point at the switch that changes this, got: %v", err)
	}
}

// TestDataSourceOmniServiceAccount_CanReportInstead covers the other side of
// that switch, and the query the endpoint takes.
func TestDataSourceOmniServiceAccount_CanReportInstead(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/omni/serviceaccount/validate", RespondString(http.StatusUnauthorized,
		"application/json", `{"message":"invalid service account key"}`))

	ds := dataSourceOmniServiceAccount()
	d := ds.TestResourceData()
	_ = d.Set("endpoint", "https://omni.example.com")
	_ = d.Set("service_account_key", "bad-key")
	_ = d.Set("fail_on_error", false)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read must not fail when fail_on_error is off: %v", err)
	}
	if got := d.Get("valid"); got != false {
		t.Errorf("valid: expected false, got %v", got)
	}
	if got := d.Get("error").(string); got == "" {
		t.Error("the rejection reason must be reported")
	}

	req := mock.FindRequest("GET", "/omni/serviceaccount/validate")
	if req == nil {
		t.Fatal("no validation request was sent")
	}
	if !strings.Contains(req.Query, "serviceAccountKey=bad-key") {
		t.Errorf("expected the key in the query, got %q", req.Query)
	}
	if !strings.Contains(req.Query, "endpoint=https") {
		t.Errorf("expected the endpoint in the query, got %q", req.Query)
	}
}

// TestDataSourceOmniServiceAccount_Accepts covers the happy path.
func TestDataSourceOmniServiceAccount_Accepts(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/omni/serviceaccount/validate", RespondJSON(http.StatusOK, map[string]interface{}{}))

	ds := dataSourceOmniServiceAccount()
	d := ds.TestResourceData()
	_ = d.Set("endpoint", "https://omni.example.com")
	_ = d.Set("service_account_key", "good-key")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("valid"); got != true {
		t.Errorf("valid: expected true, got %v", got)
	}
	if got := d.Get("error"); got != "" {
		t.Errorf("error: expected empty, got %v", got)
	}
}

// TestDataSourceOmniServiceAccount_NeverLeaksTheKey is the regression guard
// for the credential leak a review caught. The endpoint takes the service
// account key as a query parameter, and an *apiStatusError renders the whole
// request URL - so reporting the error verbatim put the key into the `error`
// attribute (and therefore state), and into the diagnostic printed to the
// console on the default path.
func TestDataSourceOmniServiceAccount_NeverLeaksTheKey(t *testing.T) {
	const key = "super-secret-omni-key"

	t.Run("not in state when the result is reported", func(t *testing.T) {
		mock := NewMockServer(t)
		mock.On("GET", "/omni/serviceaccount/validate", RespondString(http.StatusUnauthorized,
			"application/json", `{"message":"invalid service account key"}`))

		ds := dataSourceOmniServiceAccount()
		d := ds.TestResourceData()
		_ = d.Set("endpoint", "https://omni.example.com")
		_ = d.Set("service_account_key", key)
		_ = d.Set("fail_on_error", false)

		if err := rcRead(ds, d, mock.Client()); err != nil {
			t.Fatalf("Read failed: %v", err)
		}
		if got := d.Get("error").(string); strings.Contains(got, key) {
			t.Errorf("the service account key must never reach state, got %q", got)
		}
		if got := d.Get("error").(string); !strings.Contains(got, "invalid service account key") {
			t.Errorf("what Omni said must still be reported, got %q", got)
		}
	})

	t.Run("not in the diagnostic on the default path", func(t *testing.T) {
		mock := NewMockServer(t)
		mock.On("GET", "/omni/serviceaccount/validate", RespondString(http.StatusUnauthorized,
			"application/json", `{"message":"invalid service account key"}`))

		ds := dataSourceOmniServiceAccount()
		d := ds.TestResourceData()
		_ = d.Set("endpoint", "https://omni.example.com")
		_ = d.Set("service_account_key", key)
		_ = d.Set("fail_on_error", true)

		err := rcRead(ds, d, mock.Client())
		if err == nil {
			t.Fatal("expected the read to fail")
		}
		if strings.Contains(err.Error(), key) {
			t.Errorf("the service account key must never reach a diagnostic, got %q", err.Error())
		}
	})
}
