package internal

import (
	"fmt"
	"net/http"
	"reflect"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// fakePersistentVolume serves the volume pv-data of environment 5.
func fakePersistentVolume(mock *MockServer) *a2Store {
	s := newA2Store()
	s.put("pv-data", map[string]interface{}{
		"spec": map[string]interface{}{
			"capacity":                      map[string]string{"storage": "10Gi"},
			"accessModes":                   []string{"ReadWriteOnce"},
			"persistentVolumeReclaimPolicy": "Delete",
			"storageClassName":              "local-path",
			"claimRef":                      map[string]string{"namespace": "web", "name": "data"},
		},
		"status": map[string]string{"phase": "Bound"},
	})
	s.serve(mock, "/kubernetes/5/persistent_volumes/pv-data", "pv-data")
	mock.On("PUT", "/kubernetes/5/persistent_volumes/reclaim_policy", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]string
		a2Decode(r, &body)
		v, _ := s.get(body["name"])
		v.(map[string]interface{})["spec"].(map[string]interface{})["persistentVolumeReclaimPolicy"] = body["reclaimPolicy"]
		s.put(body["name"], v)
		w.WriteHeader(http.StatusNoContent)
	})
	mock.On("POST", "/kubernetes/5/persistent_volumes/delete", func(w http.ResponseWriter, r *http.Request) {
		var names []string
		a2Decode(r, &names)
		for _, n := range names {
			s.del(n)
		}
		w.WriteHeader(http.StatusNoContent)
	})
	return s
}

const persistentVolumeConfig = `
resource "portainer_kubernetes_persistent_volume" "pv" {
  environment_id = 5
  name           = "pv-data"
  reclaim_policy = %q
}
`

// TestKubernetesPersistentVolume_Lifecycle: the reclaim policy is set in
// place, the volume's details are read back, both import ID forms work and
// destroy sends the volume as a one-element list.
func TestKubernetesPersistentVolume_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	s := fakePersistentVolume(mock)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(persistentVolumeConfig, "Retain"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_kubernetes_persistent_volume.pv", "id", "5:pv-data"),
					resource.TestCheckResourceAttr("portainer_kubernetes_persistent_volume.pv", "phase", "Bound"),
					resource.TestCheckResourceAttr("portainer_kubernetes_persistent_volume.pv", "capacity", "10Gi"),
					resource.TestCheckResourceAttr("portainer_kubernetes_persistent_volume.pv", "claim_ref", "web/data"),
					resource.TestCheckResourceAttr("portainer_kubernetes_persistent_volume.pv", "access_modes.0", "ReadWriteOnce"),
					a2Check(func() error {
						var body map[string]interface{}
						a2LastBody(t, mock, "PUT", "/kubernetes/5/persistent_volumes/reclaim_policy", &body)
						if !reflect.DeepEqual(body, map[string]interface{}{"name": "pv-data", "reclaimPolicy": "Retain"}) {
							return fmt.Errorf("payload: %v", body)
						}
						return nil
					}),
				),
			},
			{
				Config: mockProviderConfig(mock) + fmt.Sprintf(persistentVolumeConfig, "Delete"),
				ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
					plancheck.ExpectResourceAction("portainer_kubernetes_persistent_volume.pv", plancheck.ResourceActionUpdate),
				}},
			},
			{
				ResourceName:      "portainer_kubernetes_persistent_volume.pv",
				ImportState:       true,
				ImportStateVerify: true,
			},
			{
				ResourceName:      "portainer_kubernetes_persistent_volume.pv",
				ImportState:       true,
				ImportStateId:     "5/pv-data",
				ImportStateVerify: true,
			},
		},
		CheckDestroy: a2Check(func() error {
			var names []string
			a2LastBody(t, mock, "POST", "/kubernetes/5/persistent_volumes/delete", &names)
			if !reflect.DeepEqual(names, []string{"pv-data"}) {
				return fmt.Errorf("delete payload: %v", names)
			}
			if _, ok := s.get("pv-data"); ok {
				return fmt.Errorf("volume not deleted")
			}
			return nil
		}),
	})
}

func TestKubernetesPersistentVolume_GoneAndErrors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	s := fakePersistentVolume(mock)
	config := mockProviderConfig(mock) + fmt.Sprintf(persistentVolumeConfig, "Retain")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			// Rejected while planning, before the volume goes: the post-test
			// destroy runs with the last applied (valid) configuration.
			{
				Config:      mockProviderConfig(mock) + fmt.Sprintf(persistentVolumeConfig, "Keep"),
				ExpectError: regexp.MustCompile(`value must be one of`),
			},
			{
				PreConfig:          func() { s.del("pv-data") },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{
				Config:        config,
				ResourceName:  "portainer_kubernetes_persistent_volume.pv",
				ImportState:   true,
				ImportStateId: "5",
				ExpectError:   regexp.MustCompile(`expected <environment_id>:<name>`),
			},
		},
	})
}

func TestKubernetesPersistentVolume_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fakePersistentVolume(mock)
	a2Upgrade(t, mock, fmt.Sprintf(persistentVolumeConfig, "Retain"))
}
