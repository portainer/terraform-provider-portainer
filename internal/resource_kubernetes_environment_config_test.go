package internal

import (
	"context"
	"net/http"
	"testing"
)

func kubernetesEndpointBody() map[string]interface{} {
	return map[string]interface{}{
		"Id":   9,
		"Type": 5,
		"Kubernetes": map[string]interface{}{
			"Snapshots": []map[string]interface{}{{"KubernetesVersion": "v1.31.0", "NodeCount": 1}},
			"Flags":     map[string]interface{}{"IsServerMetricsDetected": true, "IsServerStorageDetected": true},
			"Configuration": map[string]interface{}{
				"UseLoadBalancer":                 false,
				"UseServerMetrics":                true,
				"EnableResourceOverCommit":        true,
				"ResourceOverCommitPercentage":    80,
				"RestrictDefaultNamespace":        false,
				"IngressAvailabilityPerNamespace": true,
				"AllowNoneIngressClass":           false,
				"IngressClasses":                  []map[string]interface{}{{"Name": "traefik", "Type": "traefik", "Blocked": false}},
				"StorageClasses":                  []map[string]interface{}{{"Name": "local-path", "Provisioner": "rancher.io/local-path", "AccessModes": []string{"RWO"}, "AllowVolumeExpansion": false}},
			},
		},
	}
}

// TestKubernetesEnvironmentConfig_KeepsSnapshotsAndIngressClasses: Portainer
// replaces the whole Kubernetes object on update, so everything this resource
// does not manage has to be sent back as it was read.
func TestKubernetesEnvironmentConfig_KeepsSnapshotsAndIngressClasses(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/9", RespondJSON(http.StatusOK, kubernetesEndpointBody()))
	mock.On("PUT", "/endpoints/9", RespondJSON(http.StatusOK, map[string]interface{}{"Id": 9}))

	r := resourceKubernetesEnvironmentConfig()
	d := r.TestResourceData()
	_ = d.Set("endpoint_id", 9)
	_ = d.Set("use_load_balancer", true)
	_ = d.Set("restrict_default_namespace", true)
	_ = d.Set("storage_class", []interface{}{map[string]interface{}{
		"name": "fast", "provisioner": "ebs.csi.aws.com", "access_modes": []interface{}{"RWO", "RWX"}, "allow_volume_expansion": true,
	}})

	if err := rcCreate(r, d, mock.Client()); err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	kube, _ := decodeBody(t, mock, "PUT", "/endpoints/9")["Kubernetes"].(map[string]interface{})
	if kube == nil {
		t.Fatal("expected the Kubernetes object in the PUT body")
	}
	if snaps, _ := kube["Snapshots"].([]interface{}); len(snaps) != 1 {
		t.Errorf("Snapshots: expected them sent back unchanged, got %v", kube["Snapshots"])
	}
	if flags, _ := kube["Flags"].(map[string]interface{}); flags["IsServerMetricsDetected"] != true {
		t.Errorf("Flags: expected them sent back unchanged, got %v", kube["Flags"])
	}
	config, _ := kube["Configuration"].(map[string]interface{})
	if config["UseLoadBalancer"] != true || config["RestrictDefaultNamespace"] != true {
		t.Errorf("configured switches not sent: %v", config)
	}
	// Attributes left out keep Portainer's values instead of being reset to
	// false and 0, which the first version did on create.
	if config["UseServerMetrics"] != true || config["ResourceOverCommitPercentage"] != float64(80) {
		t.Errorf("unconfigured settings must be kept: metrics=%v percentage=%v", config["UseServerMetrics"], config["ResourceOverCommitPercentage"])
	}
	if ingress, _ := config["IngressClasses"].([]interface{}); len(ingress) != 1 {
		t.Errorf("IngressClasses: owned by portainer_kubernetes_ingresscontrollers, expected them kept, got %v", config["IngressClasses"])
	}
	classes, _ := config["StorageClasses"].([]interface{})
	first, _ := classes[0].(map[string]interface{})
	if len(classes) != 1 || first["Name"] != "fast" || first["AllowVolumeExpansion"] != true {
		t.Errorf("StorageClasses: expected the configured list, got %v", config["StorageClasses"])
	}
}

func TestKubernetesEnvironmentConfig_ReadAndImport(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/9", RespondJSON(http.StatusOK, kubernetesEndpointBody()))

	r := resourceKubernetesEnvironmentConfig()
	d := r.TestResourceData()
	d.SetId("9")
	out, err := r.Importer.StateContext(context.Background(), d, mock.Client())
	if err != nil {
		t.Fatalf("import failed: %v", err)
	}
	d = out[0]
	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Get("resource_over_commit_percentage").(int) != 80 || !d.Get("use_server_metrics").(bool) {
		t.Errorf("configuration not read: percentage=%v metrics=%v", d.Get("resource_over_commit_percentage"), d.Get("use_server_metrics"))
	}
	if d.Get("storage_class.0.name") != "local-path" || d.Get("storage_class.0.access_modes.0") != "RWO" {
		t.Errorf("storage classes not read: %v", d.Get("storage_class"))
	}
}

func TestKubernetesEnvironmentConfig_EnvironmentGone(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/9", RespondJSON(http.StatusNotFound, map[string]string{"message": "not found"}))

	r := resourceKubernetesEnvironmentConfig()
	d := r.TestResourceData()
	d.SetId("9")
	_ = d.Set("endpoint_id", 9)
	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected the resource dropped with its environment, got ID %q", d.Id())
	}
	if err := rcCreate(r, d, mock.Client()); err == nil {
		t.Error("expected Create to fail for a missing environment")
	}
}

// TestKubernetesEnvironmentConfig_DeleteKeepsConfiguration: an environment
// always has a configuration, so destroy only forgets it.
func TestKubernetesEnvironmentConfig_DeleteKeepsConfiguration(t *testing.T) {
	mock := NewMockServer(t)
	r := resourceKubernetesEnvironmentConfig()
	d := r.TestResourceData()
	d.SetId("9")
	if err := rcDelete(r, d, mock.Client()); err != nil {
		t.Fatalf("Delete failed: %v", err)
	}
	if d.Id() != "" || len(mock.Requests()) != 0 {
		t.Errorf("expected a state-only delete, got ID %q and %d requests", d.Id(), len(mock.Requests()))
	}
}
