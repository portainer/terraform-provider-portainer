package internal

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

// =========================================================================
// Omni / Talos clusters (Business Edition).
// =========================================================================

func omniMachineBlock(name string) map[string]interface{} {
	return map[string]interface{}{
		"name":             name,
		"hostname":         name,
		"install_disk":     "/dev/sda",
		"nameservers":      []interface{}{"1.1.1.1"},
		"system_disk_size": 50,
		"user_disk": []interface{}{map[string]interface{}{
			"volume_name": "data", "size": 0,
		}},
		"interface": []interface{}{map[string]interface{}{
			"interface": "eth0",
			"addresses": []interface{}{"10.0.0.10/24"},
			"route": []interface{}{map[string]interface{}{
				"network": "0.0.0.0/0", "gateway": "10.0.0.1",
			}},
		}},
	}
}

func omniClusterData(t *testing.T) (*schema.Resource, *schema.ResourceData) {
	t.Helper()
	r := resourceOmniCluster()
	d := r.TestResourceData()
	_ = d.Set("credential_id", 3)
	_ = d.Set("name", "edge-fleet")
	_ = d.Set("talos_version", "v1.8.0")
	_ = d.Set("kubernetes_version", "v1.31.0")
	_ = d.Set("validate_before_create", true)
	_ = d.Set("control_plane", []interface{}{map[string]interface{}{
		"machine": []interface{}{omniMachineBlock("cp-1")},
	}})
	_ = d.Set("worker", []interface{}{map[string]interface{}{
		"name":    "pool-a",
		"machine": []interface{}{omniMachineBlock("worker-1")},
	}})
	return r, d
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

// TestOmniCluster_ValidatesBeforeProvisioning is the reason the validation
// call is on by default: provisioning touches real machines and takes a long
// time, so a rejected configuration must be caught before any of that starts.
func TestOmniCluster_ValidatesBeforeProvisioning(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/omni/3/cluster/validate", RespondString(http.StatusBadRequest,
		"application/json", `{"message":"control plane must have an odd number of machines"}`))
	mock.On("POST", "/omni/3/cluster/create", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 12}))

	r, d := omniClusterData(t)
	err := rcCreate(r, d, mock.Client())
	if err == nil {
		t.Fatal("a rejected configuration must fail before provisioning starts")
	}
	if !strings.Contains(err.Error(), "validate_before_create") {
		t.Errorf("the error should point at the switch that turns the check off, got: %v", err)
	}
	if mock.FindRequest("POST", "/omni/3/cluster/create") != nil {
		t.Error("no cluster must be created after a failed validation")
	}
}

// TestOmniCluster_ValidationCanBeSkipped covers the escape hatch.
func TestOmniCluster_ValidationCanBeSkipped(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/omni/3/cluster/create", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 12}))
	mock.On("GET", "/omni/3/cluster/edge-fleet", respondOmniCluster())

	r, d := omniClusterData(t)
	_ = d.Set("validate_before_create", false)

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if mock.FindRequest("POST", "/omni/3/cluster/validate") != nil {
		t.Error("validation must be skipped when it is turned off")
	}
}

// TestOmniCluster_CreateSendsMachineSpec pins the payload, which is the part
// of this resource with real translation in it: flat Terraform blocks become
// Omni's nested machine specification.
func TestOmniCluster_CreateSendsMachineSpec(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/omni/3/cluster/validate", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("POST", "/omni/3/cluster/create", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 12}))
	mock.On("GET", "/omni/3/cluster/edge-fleet", respondOmniCluster())

	r, d := omniClusterData(t)
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

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
	if err := mock.FindRequest("POST", "/omni/3/cluster/create").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}

	if payload.Cluster.Name != "edge-fleet" {
		t.Errorf("cluster name: got %q", payload.Cluster.Name)
	}
	if payload.Cluster.Talos["version"] != "v1.8.0" || payload.Cluster.Kubernetes["version"] != "v1.31.0" {
		t.Errorf("the versions must be nested under their own objects: %+v", payload.Cluster)
	}
	if len(payload.ControlPlane.Machines) != 1 {
		t.Fatalf("expected one control plane machine, got %d", len(payload.ControlPlane.Machines))
	}
	machine := payload.ControlPlane.Machines[0]
	if machine["name"] != "cp-1" {
		t.Errorf("machine name: got %v", machine["name"])
	}
	install, ok := machine["install"].(map[string]interface{})
	if !ok || install["disk"] != "/dev/sda" {
		t.Errorf("install_disk must be nested under install: %v", machine["install"])
	}
	if machine["systemDiskSize"] != float64(50) {
		t.Errorf("systemDiskSize: got %v", machine["systemDiskSize"])
	}
	interfaces, ok := machine["interfaces"].([]interface{})
	if !ok || len(interfaces) != 1 {
		t.Fatalf("expected one interface, got %v", machine["interfaces"])
	}
	iface := interfaces[0].(map[string]interface{})
	routes, ok := iface["routes"].([]interface{})
	if !ok || len(routes) != 1 {
		t.Fatalf("expected one route, got %v", iface["routes"])
	}
	if routes[0].(map[string]interface{})["gateway"] != "10.0.0.1" {
		t.Errorf("the route was not sent: %v", routes[0])
	}
	if payload.Worker.Name != "pool-a" || len(payload.Worker.Machines) != 1 {
		t.Errorf("the worker pool was not sent: %+v", payload.Worker)
	}

	if got := d.Get("endpoint_id"); got != 12 {
		t.Errorf("endpoint_id: expected the environment Portainer created, got %v", got)
	}
	if d.Id() != "3/edge-fleet" {
		t.Errorf("the resource ID must carry both the credential and the name, got %q", d.Id())
	}
}

// TestOmniCluster_UserDiskSizeZeroIsSent covers the one numeric field where
// zero is meaningful: Omni reads it as "grow into the remaining space".
func TestOmniCluster_UserDiskSizeZeroIsSent(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/omni/3/cluster/validate", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("POST", "/omni/3/cluster/create", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 12}))
	mock.On("GET", "/omni/3/cluster/edge-fleet", respondOmniCluster())

	r, d := omniClusterData(t)
	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}

	var payload struct {
		ControlPlane struct {
			Machines []map[string]interface{} `json:"machines"`
		} `json:"controlPlane"`
	}
	if err := mock.FindRequest("POST", "/omni/3/cluster/create").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	disk, ok := payload.ControlPlane.Machines[0]["userDisk"].(map[string]interface{})
	if !ok {
		t.Fatalf("the user disk must be sent, got %v", payload.ControlPlane.Machines[0]["userDisk"])
	}
	if _, ok := disk["size"]; !ok {
		t.Error("a zero user disk size must be sent: Omni reads it as 'use the remaining space'")
	}
}

// TestOmniCluster_ReadLeavesMachineBlocksAlone pins the limitation worth being
// explicit about: Portainer reports the cluster's spec and status but not the
// machines it was built from, so the read must not invent them.
func TestOmniCluster_ReadLeavesMachineBlocksAlone(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/omni/3/cluster/edge-fleet", respondOmniCluster())

	r := resourceOmniCluster()
	d := r.TestResourceData()
	d.SetId("3/edge-fleet")
	_ = d.Set("control_plane", []interface{}{map[string]interface{}{
		"machine": []interface{}{omniMachineBlock("cp-1")},
	}})

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	if got := d.Get("ready"); got != true {
		t.Errorf("ready: expected the status to be read, got %v", got)
	}
	if got := d.Get("machines_healthy"); got != 2 {
		t.Errorf("machines_healthy: expected 2, got %v", got)
	}
	if got := d.Get("backup_interval"); got != "1h" {
		t.Errorf("backup_interval: expected 1h, got %v", got)
	}

	blocks := d.Get("control_plane").([]interface{})
	if len(blocks) != 1 {
		t.Fatalf("the configured control plane must survive the read, got %d", len(blocks))
	}
	machines := blocks[0].(map[string]interface{})["machine"].([]interface{})
	if len(machines) != 1 || machines[0].(map[string]interface{})["name"] != "cp-1" {
		t.Errorf("the read must not touch the machine blocks, got %v", machines)
	}
}

// TestOmniCluster_UpdateSendsMachineDelta covers the translation the update
// endpoint forces: it is imperative - machines to add, names to remove - so
// the declarative machine list has to be turned into that difference.
func TestOmniCluster_UpdateSendsMachineDelta(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("PUT", "/omni/3/cluster/update", RespondJSON(http.StatusOK, map[string]interface{}{}))
	mock.On("GET", "/omni/3/cluster/edge-fleet", respondOmniCluster())

	r := resourceOmniCluster()

	seed := r.TestResourceData()
	seed.SetId("3/edge-fleet")
	_ = seed.Set("credential_id", 3)
	_ = seed.Set("name", "edge-fleet")
	_ = seed.Set("worker", []interface{}{map[string]interface{}{
		"name": "pool-a",
		"machine": []interface{}{
			omniMachineBlock("worker-1"),
			omniMachineBlock("worker-2"),
		},
	}})
	state := seed.State()

	config := terraform.NewResourceConfigRaw(map[string]interface{}{
		"credential_id": 3,
		"name":          "edge-fleet",
		"worker": []interface{}{map[string]interface{}{
			"name": "pool-a",
			"machine": []interface{}{
				omniMachineBlock("worker-1"),
				omniMachineBlock("worker-3"),
			},
		}},
	})
	diff, err := r.Diff(context.Background(), state, config, nil)
	if err != nil {
		t.Fatalf("Diff failed: %v", err)
	}
	d, err := schema.InternalMap(r.Schema).Data(state, diff)
	if err != nil {
		t.Fatalf("building resource data from the diff failed: %v", err)
	}

	if err := rcUpdate(r, d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	var payload struct {
		WorkersToAdd struct {
			Machines []map[string]interface{} `json:"machines"`
		} `json:"workersToAdd"`
		WorkersToRemove []string `json:"workersToRemove"`
	}
	if err := mock.FindRequest("PUT", "/omni/3/cluster/update").DecodeJSON(&payload); err != nil {
		t.Fatalf("payload is not JSON: %v", err)
	}
	if len(payload.WorkersToAdd.Machines) != 1 || payload.WorkersToAdd.Machines[0]["name"] != "worker-3" {
		t.Errorf("only the new machine must be added, got %v", payload.WorkersToAdd.Machines)
	}
	if len(payload.WorkersToRemove) != 1 || payload.WorkersToRemove[0] != "worker-2" {
		t.Errorf("only the dropped machine must be removed, got %v", payload.WorkersToRemove)
	}
}

// TestOmniCluster_DeletePassesNameAsQuery pins the delete, whose cluster name
// is a query parameter rather than part of the path.
func TestOmniCluster_DeletePassesNameAsQuery(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("DELETE", "/omni/3/cluster/delete", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceOmniCluster()
	d := r.TestResourceData()
	d.SetId("3/edge-fleet")

	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	req := mock.FindRequest("DELETE", "/omni/3/cluster/delete")
	if req == nil || !strings.Contains(req.Query, "name=edge-fleet") {
		t.Errorf("expected the cluster name in the query, got %q", req.Query)
	}
}

// TestOmniCluster_MalformedIDIsReported guards the two-part resource ID.
func TestOmniCluster_MalformedIDIsReported(t *testing.T) {
	mock := NewMockServer(t)

	r := resourceOmniCluster()
	d := r.TestResourceData()
	d.SetId("edge-fleet")

	err := rcRead(r, d, mock.Client())
	if err == nil {
		t.Fatal("an ID without a credential must be reported")
	}
	if !strings.Contains(err.Error(), "<credential id>/<cluster name>") {
		t.Errorf("the error should say what the ID looks like, got: %v", err)
	}
}

// TestOmniNodeReboot_PassesClusterAsQuery pins the reboot action.
func TestOmniNodeReboot_PassesClusterAsQuery(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("POST", "/omni/3/node/cp-1", RespondJSON(http.StatusOK, map[string]interface{}{}))

	r := resourceOmniNodeReboot()
	d := r.TestResourceData()
	_ = d.Set("credential_id", 3)
	_ = d.Set("cluster", "edge-fleet")
	_ = d.Set("node", "cp-1")

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	req := mock.FindRequest("POST", "/omni/3/node/cp-1")
	if req == nil || !strings.Contains(req.Query, "cluster=edge-fleet") {
		t.Errorf("expected the cluster name in the query, got %q", req.Query)
	}
	if !strings.HasPrefix(d.Id(), "3/edge-fleet/cp-1/reboot-") {
		t.Errorf("the ID should name the node and the run, got %q", d.Id())
	}
}
