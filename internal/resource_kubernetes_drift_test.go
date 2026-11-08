package internal

import (
	"net/http"
	"strings"
	"testing"
)

const driftDeployment = `
apiVersion: apps/v1
kind: Deployment
metadata:
  name: web
  labels:
    app: web
spec:
  replicas: 2
  template:
    spec:
      containers:
        - name: web
          image: nginx:1.29
          resources:
            limits:
              cpu: 0.5
              memory: 1Gi
          ports:
            - containerPort: 80
`

// liveDeployment is driftDeployment as the API server returns it: defaults,
// status and metadata added, quantities in canonical form.
func liveDeployment(image string, replicas int) map[string]interface{} {
	return map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name": "web", "namespace": "default", "resourceVersion": "4711",
			"labels":      map[string]interface{}{"app": "web"},
			"annotations": map[string]interface{}{"deployment.kubernetes.io/revision": "1"},
		},
		"spec": map[string]interface{}{
			"replicas":             float64(replicas),
			"revisionHistoryLimit": float64(10),
			"template": map[string]interface{}{"spec": map[string]interface{}{
				"restartPolicy": "Always",
				"containers": []interface{}{map[string]interface{}{
					"name": "web", "image": image, "imagePullPolicy": "IfNotPresent",
					"resources": map[string]interface{}{"limits": map[string]interface{}{"cpu": "500m", "memory": "1024Mi"}},
					"ports":     []interface{}{map[string]interface{}{"containerPort": float64(80), "protocol": "TCP"}},
				}},
			}},
		},
		"status": map[string]interface{}{"readyReplicas": float64(replicas)},
	}
}

// TestK8sReadManifestDrift covers content drift for manifest resources, which
// used to detect only a deleted object. Fields the server adds and canonical
// quantities are not drift; a changed authored field is, and the plan shows
// it with its live value.
func TestK8sReadManifestDrift(t *testing.T) {
	cases := []struct {
		name      string
		live      map[string]interface{}
		wantDrift string
	}{
		{"server defaults and canonical quantities", liveDeployment("nginx:1.29", 2), ""},
		{"image changed outside Terraform", liveDeployment("nginx:1.30", 2), "image: nginx:1.30"},
		// Replicas are left to HPAs and portainer_kubernetes_deployment_scale.
		{"scaled outside Terraform", liveDeployment("nginx:1.29", 5), ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("GET", "/endpoints/1/kubernetes/apis/apps/v1/namespaces/default/deployments/web", RespondJSON(http.StatusOK, tc.live))

			r := resourceKubernetesApplication()
			d := r.TestResourceData()
			d.SetId("1:default:web")
			_ = d.Set("endpoint_id", 1)
			_ = d.Set("namespace", "default")
			_ = d.Set("manifest", driftDeployment)

			if err := rcRead(r, d, mock.Client()); err != nil {
				t.Fatalf("Read failed: %v", err)
			}
			got := d.Get("manifest").(string)
			if tc.wantDrift == "" {
				if got != driftDeployment {
					t.Errorf("no drift: the authored manifest must stay untouched, got:\n%s", got)
				}
				return
			}
			if !strings.Contains(got, tc.wantDrift) {
				t.Errorf("expected %q in the drifted manifest, got:\n%s", tc.wantDrift, got)
			}
			for _, serverOnly := range []string{"resourceVersion", "status", "imagePullPolicy", "revisionHistoryLimit"} {
				if strings.Contains(got, serverOnly) {
					t.Errorf("the drifted manifest must only hold authored fields, found %q in:\n%s", serverOnly, got)
				}
			}
		})
	}
}

func TestK8sReadManifestDrift_Deleted(t *testing.T) {
	mock := NewMockServer(t)
	mock.On("GET", "/endpoints/1/kubernetes/api/v1/namespaces/default/configmaps/cfg", RespondJSON(http.StatusNotFound, map[string]interface{}{}))

	r := resourceKubernetesConfigMaps()
	d := r.TestResourceData()
	d.SetId("1:default:cfg")
	_ = d.Set("manifest", "kind: ConfigMap\nmetadata:\n  name: cfg\n")

	if err := rcRead(r, d, mock.Client()); err != nil {
		t.Fatalf("Read failed: %v", err)
	}
	if d.Id() != "" {
		t.Errorf("expected the deleted object to drop out of state, got ID %q", d.Id())
	}
}

// TestK8sComparable_ClusterScopedNamespace covers a namespace written on a
// cluster-scoped object, which the API server drops: it is not drift.
func TestK8sComparable_ClusterScopedNamespace(t *testing.T) {
	authored := map[string]interface{}{"kind": "ClusterRole", "metadata": map[string]interface{}{"name": "r", "namespace": "devops-tools"}}
	live := map[string]interface{}{"kind": "ClusterRole", "metadata": map[string]interface{}{"name": "r", "uid": "x"}}
	if !k8sSubsetEqual(k8sComparable(authored, live), live) {
		t.Error("a namespace the server dropped must not be drift")
	}
	if _, ok := authored["metadata"].(map[string]interface{})["namespace"]; !ok {
		t.Error("the authored manifest must not be modified")
	}
}

func TestK8sSubsetEqual(t *testing.T) {
	cases := []struct {
		name     string
		authored map[string]interface{}
		live     map[string]interface{}
		want     bool
	}{
		{
			name:     "secret stringData matches base64 data",
			authored: k8sNormalizeAuthored(map[string]interface{}{"kind": "Secret", "stringData": map[string]interface{}{"password": "s3cret"}}),
			live:     map[string]interface{}{"kind": "Secret", "data": map[string]interface{}{"password": "czNjcmV0"}, "type": "Opaque"},
			want:     true,
		},
		{
			name:     "secret value changed",
			authored: k8sNormalizeAuthored(map[string]interface{}{"kind": "Secret", "stringData": map[string]interface{}{"password": "s3cret"}}),
			live:     map[string]interface{}{"kind": "Secret", "data": map[string]interface{}{"password": "b3RoZXI="}},
			want:     false,
		},
		{
			name:     "configmap key added outside Terraform is not drift of authored fields",
			authored: map[string]interface{}{"data": map[string]interface{}{"a": "1"}},
			live:     map[string]interface{}{"data": map[string]interface{}{"a": "1", "b": "2"}},
			want:     true,
		},
		{
			name:     "configmap key removed",
			authored: map[string]interface{}{"data": map[string]interface{}{"a": "1", "b": "2"}},
			live:     map[string]interface{}{"data": map[string]interface{}{"a": "1"}},
			want:     false,
		},
		{
			name:     "rule added to a role",
			authored: map[string]interface{}{"rules": []interface{}{map[string]interface{}{"verbs": []interface{}{"get"}}}},
			live:     map[string]interface{}{"rules": []interface{}{map[string]interface{}{"verbs": []interface{}{"get"}}, map[string]interface{}{"verbs": []interface{}{"list"}}}},
			want:     false,
		},
		{
			name:     "empty authored map and missing live field",
			authored: map[string]interface{}{"metadata": map[string]interface{}{"labels": map[string]interface{}{}}},
			live:     map[string]interface{}{"metadata": map[string]interface{}{}},
			want:     true,
		},
		{
			name:     "boolean changed",
			authored: map[string]interface{}{"allowVolumeExpansion": true},
			live:     map[string]interface{}{"allowVolumeExpansion": false},
			want:     false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := k8sSubsetEqual(tc.authored, tc.live); got != tc.want {
				t.Errorf("expected %v, got %v", tc.want, got)
			}
		})
	}
}

func TestK8sScalarEqual_Quantities(t *testing.T) {
	cases := []struct {
		a, b interface{}
		want bool
	}{
		{0.5, "500m", true},
		{"1Gi", "1024Mi", true},
		{"1G", "1000M", true},
		{"1G", "1Gi", false},
		{1, float64(1), true},
		{"100m", "200m", false},
		{"nginx:1.29", "nginx:1.30", false},
	}
	for _, tc := range cases {
		if got := k8sScalarEqual(tc.a, tc.b); got != tc.want {
			t.Errorf("k8sScalarEqual(%v, %v) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}
