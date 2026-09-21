package internal

import (
	"net/http"
	"strings"
	"testing"
)

// =========================================================================
// Wave three, Kubernetes storage and workload data sources. Unlike the rest
// of this work most of these endpoints exist in Portainer CE too; only the
// Endpoints listing and the application inspect are Business Edition only.
// =========================================================================

func storageClassResponse() map[string]interface{} {
	return map[string]interface{}{
		"name": "fast-ssd", "provisioner": "kubernetes.io/aws-ebs",
		"reclaimPolicy": "Delete", "isDefault": true, "allowVolumeExpansion": true,
		"mountOptions": []string{"noatime"},
		"parameters":   map[string]string{"type": "gp3"},
		"labels":       map[string]string{"tier": "fast"},
		"annotations":  map[string]string{"owner": "platform"},
		"creationDate": "2026-01-01",
	}
}

// TestDataSourceKubernetesStorageClasses_ListAndInspect covers both storage
// class data sources and the map and list fields they flatten.
func TestDataSourceKubernetesStorageClasses_ListAndInspect(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/5/storage_classes", RespondJSON(http.StatusOK, []interface{}{storageClassResponse()}))
	mock.On("GET", "/kubernetes/5/storage_classes/fast-ssd", RespondJSON(http.StatusOK, storageClassResponse()))

	list := dataSourceKubernetesStorageClasses()
	dList := list.TestResourceData()
	_ = dList.Set("endpoint_id", 5)
	if err := rcRead(list, dList, mock.Client()); err != nil {
		t.Fatalf("list read failed: %v", err)
	}
	classes := dList.Get("storage_classes").([]interface{})
	if len(classes) != 1 {
		t.Fatalf("expected one storage class, got %d", len(classes))
	}
	class := classes[0].(map[string]interface{})
	if class["name"] != "fast-ssd" || class["is_default"] != true {
		t.Errorf("the storage class was not read: %+v", class)
	}
	if params := class["parameters"].(map[string]interface{}); params["type"] != "gp3" {
		t.Errorf("the parameters were not read: %v", params)
	}
	if opts := class["mount_options"].([]interface{}); len(opts) != 1 {
		t.Errorf("the mount options were not read: %v", opts)
	}

	single := dataSourceKubernetesStorageClass()
	dSingle := single.TestResourceData()
	_ = dSingle.Set("endpoint_id", 5)
	_ = dSingle.Set("name", "fast-ssd")
	if err := rcRead(single, dSingle, mock.Client()); err != nil {
		t.Fatalf("inspect read failed: %v", err)
	}
	if got := dSingle.Get("provisioner_name"); got != "kubernetes.io/aws-ebs" {
		t.Errorf("provisioner: got %v", got)
	}
	if got := dSingle.Get("allow_volume_expansion"); got != true {
		t.Errorf("allow_volume_expansion: got %v", got)
	}
	// The name is an argument, so a response is never allowed to rewrite it.
	if got := dSingle.Get("name"); got != "fast-ssd" {
		t.Errorf("name: the argument must survive the read, got %v", got)
	}
}

// TestDataSourceKubernetesPVCs_NamespaceSelectsPath pins the two listing paths
// Portainer has: one for the whole cluster, one for a single namespace.
func TestDataSourceKubernetesPVCs_NamespaceSelectsPath(t *testing.T) {
	claim := map[string]interface{}{
		"id": "uid-1", "name": "data", "namespace": "apps", "phase": "Bound",
		"storageClass": "fast-ssd", "storageRequest": "10Gi", "storage": 10737418240,
		"volumeName": "pv-1", "volumeMode": "Filesystem",
		"accessModes": []string{"ReadWriteOnce"}, "humanReadableAccessModes": []string{"RWO"},
		"owningApplications": []string{"web"},
		"labels":             map[string]string{"app": "web"},
		"creationDate":       "2026-01-01",
	}

	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/5/persistent_volume_claims", RespondJSON(http.StatusOK, []interface{}{claim}))
	mock.On("GET", "/kubernetes/5/namespaces/apps/persistent_volume_claims", RespondJSON(http.StatusOK, []interface{}{claim}))

	ds := dataSourceKubernetesPersistentVolumeClaims()

	cluster := ds.TestResourceData()
	_ = cluster.Set("endpoint_id", 5)
	if err := rcRead(ds, cluster, mock.Client()); err != nil {
		t.Fatalf("cluster-wide read failed: %v", err)
	}
	if mock.FindRequest("GET", "/kubernetes/5/persistent_volume_claims") == nil {
		t.Error("a read without a namespace must list the whole cluster")
	}
	claims := cluster.Get("claims").([]interface{})
	entry := claims[0].(map[string]interface{})
	if entry["phase"] != "Bound" || entry["storage_request"] != "10Gi" {
		t.Errorf("the claim was not read: %+v", entry)
	}
	if entry["storage"] != 10737418240 {
		t.Errorf("storage: expected the byte figure, got %v", entry["storage"])
	}

	scoped := ds.TestResourceData()
	_ = scoped.Set("endpoint_id", 5)
	_ = scoped.Set("namespace", "apps")
	if err := rcRead(ds, scoped, mock.Client()); err != nil {
		t.Fatalf("namespaced read failed: %v", err)
	}
	if mock.FindRequest("GET", "/kubernetes/5/namespaces/apps/persistent_volume_claims") == nil {
		t.Error("a read with a namespace must use the namespaced path")
	}
}

// TestDataSourceKubernetesPVC_Single covers the inspect data source.
func TestDataSourceKubernetesPVC_Single(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/5/namespaces/apps/persistent_volume_claims/data",
		RespondJSON(http.StatusOK, map[string]interface{}{
			"id": "uid-1", "name": "data", "namespace": "apps", "phase": "Pending",
			"storageClass": "fast-ssd", "storageRequest": "10Gi",
			"owningApplications": []string{},
		}))

	ds := dataSourceKubernetesPersistentVolumeClaim()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 5)
	_ = d.Set("namespace", "apps")
	_ = d.Set("name", "data")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("phase"); got != "Pending" {
		t.Errorf("phase: got %v", got)
	}
	// name and namespace are arguments and must survive the read.
	if d.Get("name") != "data" || d.Get("namespace") != "apps" {
		t.Errorf("the arguments must survive the read: %v/%v", d.Get("namespace"), d.Get("name"))
	}
}

// TestDataSourceKubernetesVolumes_CarriesJSON pins the decision to pass this
// listing through as JSON: Portainer gives it no fixed shape in its own
// specification, so flattening it would invent one.
func TestDataSourceKubernetesVolumes_CarriesJSON(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/5/volumes", RespondJSON(http.StatusOK, map[string]interface{}{
		"items": []map[string]interface{}{{"persistentVolumeClaim": map[string]interface{}{"name": "data"}}},
	}))
	mock.On("GET", "/kubernetes/5/namespaces/apps/volumes", RespondJSON(http.StatusOK, map[string]interface{}{"items": []interface{}{}}))

	ds := dataSourceKubernetesVolumes()

	cluster := ds.TestResourceData()
	_ = cluster.Set("endpoint_id", 5)
	_ = cluster.Set("with_applications", true)
	if err := rcRead(ds, cluster, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	req := mock.FindRequest("GET", "/kubernetes/5/volumes")
	if req == nil || !strings.Contains(req.Query, "withApplications=true") {
		t.Errorf("expected the applications flag in the query, got %q", req.Query)
	}
	if !strings.Contains(cluster.Get("volumes").(string), "persistentVolumeClaim") {
		t.Errorf("the response must be carried through verbatim, got %v", cluster.Get("volumes"))
	}

	scoped := ds.TestResourceData()
	_ = scoped.Set("endpoint_id", 5)
	_ = scoped.Set("namespace", "apps")
	if err := rcRead(ds, scoped, mock.Client()); err != nil {
		t.Fatalf("namespaced read failed: %v", err)
	}
	if mock.FindRequest("GET", "/kubernetes/5/namespaces/apps/volumes") == nil {
		t.Error("a read with a namespace must use the namespaced path")
	}
}

// TestDataSourceKubernetesVolume_FlattensAndKeepsRaw covers the single volume
// view: the fields worth acting on are flattened, and the whole response is
// still available for everything else.
func TestDataSourceKubernetesVolume_FlattensAndKeepsRaw(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/5/volumes/apps/data", RespondJSON(http.StatusOK, map[string]interface{}{
		"persistentVolume": map[string]interface{}{
			"name": "pv-1", "status": "Bound", "persistentVolumeReclaimPolicy": "Retain",
			"csi": map[string]interface{}{"driver": "ebs.csi.aws.com"},
		},
		"persistentVolumeClaim": map[string]interface{}{
			"name": "data", "phase": "Bound", "storageRequest": "10Gi",
		},
		"storageClass": map[string]interface{}{"name": "fast-ssd", "provisioner": "ebs.csi.aws.com"},
	}))

	ds := dataSourceKubernetesVolume()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 5)
	_ = d.Set("namespace", "apps")
	_ = d.Set("volume", "data")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("persistent_volume_reclaim_policy"); got != "Retain" {
		t.Errorf("persistent_volume_reclaim_policy: got %v", got)
	}
	if got := d.Get("claim_storage_request"); got != "10Gi" {
		t.Errorf("claim_storage_request: got %v", got)
	}
	if got := d.Get("storage_class_provisioner"); got != "ebs.csi.aws.com" {
		t.Errorf("storage_class_provisioner: got %v", got)
	}
	// The CSI block is not flattened, so it has to survive in the raw details.
	if !strings.Contains(d.Get("details").(string), "ebs.csi.aws.com") {
		t.Error("the whole response must be kept for what the attributes do not cover")
	}
}

// TestDataSourceKubernetesCronJobs_FiltersSystem covers the default that keeps
// Kubernetes' own jobs out of a listing a configuration would iterate.
func TestDataSourceKubernetesCronJobs_FiltersSystem(t *testing.T) {
	jobs := []map[string]interface{}{
		{"Id": "1", "Name": "backup", "Namespace": "apps", "Schedule": "0 2 * * *", "Suspend": false, "IsSystem": false},
		{"Id": "2", "Name": "system-job", "Namespace": "kube-system", "Schedule": "* * * * *", "IsSystem": true},
	}

	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/5/cron_jobs", RespondJSON(http.StatusOK, jobs))

	ds := dataSourceKubernetesCronJobs()

	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 5)
	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	listed := d.Get("cron_jobs").([]interface{})
	if len(listed) != 1 || listed[0].(map[string]interface{})["name"] != "backup" {
		t.Errorf("system jobs must be filtered out by default, got %v", listed)
	}

	withSystem := ds.TestResourceData()
	_ = withSystem.Set("endpoint_id", 5)
	_ = withSystem.Set("include_system", true)
	if err := rcRead(ds, withSystem, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := withSystem.Get("cron_jobs").([]interface{}); len(got) != 2 {
		t.Errorf("include_system must bring the system jobs back, got %d", len(got))
	}
}

// TestDataSourceKubernetesEndpoints_ReadsAddressesAndPorts covers the nested
// port list, and the empty-address case that identifies a service with no
// ready backends.
func TestDataSourceKubernetesEndpoints_ReadsAddressesAndPorts(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/5/endpoints", RespondJSON(http.StatusOK, []map[string]interface{}{
		{
			"uid": "uid-1", "name": "web", "namespace": "apps",
			"addresses": []string{"10.1.0.4"},
			"ports":     []map[string]interface{}{{"name": "http", "port": 8080, "protocol": "TCP"}},
		},
		{"uid": "uid-2", "name": "broken", "namespace": "apps", "addresses": []string{}},
	}))

	ds := dataSourceKubernetesEndpoints()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 5)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	endpoints := d.Get("endpoints").([]interface{})
	if len(endpoints) != 2 {
		t.Fatalf("expected two endpoints, got %d", len(endpoints))
	}
	web := endpoints[0].(map[string]interface{})
	ports := web["ports"].([]interface{})
	if len(ports) != 1 || ports[0].(map[string]interface{})["port"] != 8080 {
		t.Errorf("the ports were not read: %v", ports)
	}
	broken := endpoints[1].(map[string]interface{})
	if len(broken["addresses"].([]interface{})) != 0 {
		t.Errorf("a service with no backends must read as an empty address list: %v", broken)
	}
}

// TestDataSourceKubernetesServiceAccount_ReadsReferences covers the account
// inspect, including the secret references that are names rather than values.
func TestDataSourceKubernetesServiceAccount_ReadsReferences(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/5/namespaces/apps/service_accounts/deployer",
		RespondJSON(http.StatusOK, map[string]interface{}{
			"uid": "uid-1", "isSystem": false, "automountServiceAccountToken": true,
			"imagePullSecrets": []string{"registry-creds"},
			"labels":           map[string]string{"app": "deployer"},
			"annotations":      map[string]string{"owner": "platform"},
			"creationDate":     "2026-01-01",
		}))

	ds := dataSourceKubernetesServiceAccount()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 5)
	_ = d.Set("namespace", "apps")
	_ = d.Set("name", "deployer")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if got := d.Get("automount_service_account_token"); got != true {
		t.Errorf("automount_service_account_token: got %v", got)
	}
	secrets := d.Get("image_pull_secrets").([]interface{})
	if len(secrets) != 1 || secrets[0] != "registry-creds" {
		t.Errorf("the image pull secret references were not read: %v", secrets)
	}
}

// TestDataSourceKubernetesApplication_FlattensAndKeepsRaw covers the
// application inspect, whose deep structures stay in the raw details.
func TestDataSourceKubernetesApplication_FlattensAndKeepsRaw(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/5/namespaces/apps/applications/web",
		RespondJSON(http.StatusOK, map[string]interface{}{
			"Uid": "uid-1", "Kind": "Deployment", "Image": "nginx:1.27",
			"Status": "Running", "RunningPodsCount": 2, "TotalPodsCount": 3,
			"ApplicationType": "Deployment", "ApplicationOwner": "platform",
			"DeploymentType": "Git", "StackId": "9", "StackName": "web",
			"ServiceName": "web", "ServiceType": "LoadBalancer",
			"LoadBalancerIPAddress": "10.0.0.50",
			"Labels":                map[string]string{"app": "web"},
			"CreationDate":          "2026-01-01",
			"Pods":                  []map[string]interface{}{{"Name": "web-0"}},
		}))

	ds := dataSourceKubernetesApplication()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 5)
	_ = d.Set("namespace", "apps")
	_ = d.Set("name", "web")
	_ = d.Set("resource_type", "Deployment")

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	req := mock.FindRequest("GET", "/kubernetes/5/namespaces/apps/applications/web")
	if req == nil || !strings.Contains(req.Query, "resourceType=Deployment") {
		t.Errorf("expected the resource type in the query, got %q", req.Query)
	}
	if got := d.Get("running_pods"); got != 2 {
		t.Errorf("running_pods: got %v", got)
	}
	if got := d.Get("total_pods"); got != 3 {
		t.Errorf("total_pods: got %v", got)
	}
	if got := d.Get("load_balancer_ip"); got != "10.0.0.50" {
		t.Errorf("load_balancer_ip: got %v", got)
	}
	// Pods are not flattened, so they have to survive in the raw details.
	if !strings.Contains(d.Get("details").(string), "web-0") {
		t.Error("the pod detail must be kept in the raw response")
	}
}

// TestDataSourceKubernetesResourceCounts_ReadsEveryCount pins the seven calls
// this data source folds together, and that each lands on its own endpoint.
func TestDataSourceKubernetesResourceCounts_ReadsEveryCount(t *testing.T) {
	mock := NewMockServer(t)
	for path, count := range map[string]int{
		"/kubernetes/5/applications/count": 12,
		"/kubernetes/5/namespaces/count":   4,
		"/kubernetes/5/services/count":     20,
		"/kubernetes/5/ingresses/count":    3,
		"/kubernetes/5/configmaps/count":   17,
		"/kubernetes/5/secrets/count":      9,
		"/kubernetes/5/volumes/count":      6,
	} {
		mock.On("GET", path, RespondJSON(http.StatusOK, count))
	}

	ds := dataSourceKubernetesResourceCounts()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 5)

	if err := rcRead(ds, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}

	for attribute, want := range map[string]int{
		"applications": 12, "namespaces": 4, "services": 20,
		"ingresses": 3, "config_maps": 17, "secrets": 9, "volumes": 6,
	} {
		if got := d.Get(attribute); got != want {
			t.Errorf("%s: expected %d, got %v", attribute, want, got)
		}
	}
	if len(mock.Requests()) != 7 {
		t.Errorf("expected one call per count, got %d", len(mock.Requests()))
	}
}

// TestDataSourceKubernetesResourceCounts_NamesTheFailingCount keeps a single
// failing count from being reported as an opaque error.
func TestDataSourceKubernetesResourceCounts_NamesTheFailingCount(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/kubernetes/5/applications/count", RespondJSON(http.StatusOK, 12))
	mock.On("GET", "/kubernetes/5/namespaces/count", RespondString(http.StatusForbidden,
		"application/json", `{"message":"forbidden"}`))

	ds := dataSourceKubernetesResourceCounts()
	d := ds.TestResourceData()
	_ = d.Set("endpoint_id", 5)

	err := rcRead(ds, d, mock.Client())
	if err == nil {
		t.Fatal("a failing count must fail the read")
	}
	if !strings.Contains(err.Error(), "namespaces") {
		t.Errorf("the error should name which count failed, got: %v", err)
	}
}
