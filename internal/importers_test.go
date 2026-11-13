package internal

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

func runImport(t *testing.T, r *schema.Resource, id string) (*schema.ResourceData, error) {
	t.Helper()
	d := r.TestResourceData()
	d.SetId(id)
	out, err := r.Importer.StateContext(context.Background(), d, nil)
	if err != nil {
		return nil, err
	}
	return out[0], nil
}

func TestImporters_SplitCompositeIDs(t *testing.T) {
	cases := []struct {
		name   string
		r      *schema.Resource
		id     string
		wantID string
		want   map[string]interface{}
	}{
		{"registry access team", resourceRegistryAccess(), "3:1:team:7", "3:1:team:7", map[string]interface{}{"registry_id": 3, "endpoint_id": 1, "team_id": 7}},
		{"registry access user", resourceRegistryAccess(), "3:1:user:9", "3:1:user:9", map[string]interface{}{"registry_id": 3, "endpoint_id": 1, "user_id": 9}},
		{"registry access legacy", resourceRegistryAccess(), "3/1/team/7", "3:1:team:7", map[string]interface{}{"registry_id": 3, "endpoint_id": 1, "team_id": 7}},
		{"endpoint group access", resourceEndpointGroupAccess(), "2:team:4", "2:team:4", map[string]interface{}{"endpoint_group_id": 2, "team_id": 4}},
		{"endpoint group access legacy", resourceEndpointGroupAccess(), "2/user/4", "2:user:4", map[string]interface{}{"endpoint_group_id": 2, "user_id": 4}},
		{"docker node", resourceDockerNode(), "1:abc123xyz", "1:abc123xyz", map[string]interface{}{"endpoint_id": 1, "node_id": "abc123xyz"}},
		{"docker node legacy", resourceDockerNode(), "1-abc123xyz", "1:abc123xyz", map[string]interface{}{"endpoint_id": 1, "node_id": "abc123xyz"}},
		{"persistent volume", resourceKubernetesPersistentVolume(), "5:pv-data", "5:pv-data", map[string]interface{}{"environment_id": 5, "name": "pv-data"}},
		{"persistent volume legacy", resourceKubernetesPersistentVolume(), "5/pv-data", "5:pv-data", map[string]interface{}{"environment_id": 5, "name": "pv-data"}},
		{"ingress controllers", resourceKubernetesIngressControllers(), "5", "5", map[string]interface{}{"environment_id": 5}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d, err := runImport(t, tc.r, tc.id)
			if err != nil {
				t.Fatalf("import %q failed: %v", tc.id, err)
			}
			if d.Id() != tc.wantID {
				t.Errorf("ID: expected %q, got %q", tc.wantID, d.Id())
			}
			for k, v := range tc.want {
				if got := d.Get(k); got != v {
					t.Errorf("%s: expected %v, got %v", k, v, got)
				}
			}
		})
	}
}

func TestImporters_RejectMalformedIDs(t *testing.T) {
	cases := []struct {
		name string
		r    *schema.Resource
		id   string
	}{
		{"registry access missing principal", resourceRegistryAccess(), "3:1"},
		{"registry access legacy missing principal", resourceRegistryAccess(), "3/1"},
		{"registry access unknown principal", resourceRegistryAccess(), "3/1/group/7"},
		{"registry access non-numeric", resourceRegistryAccess(), "x/1/team/7"},
		{"endpoint group access empty part", resourceEndpointGroupAccess(), "2:team:"},
		{"docker node without node", resourceDockerNode(), "1"},
		{"persistent volume without name", resourceKubernetesPersistentVolume(), "5"},
		{"ingress controllers non-numeric", resourceKubernetesIngressControllers(), "prod"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := runImport(t, tc.r, tc.id); err == nil {
				t.Errorf("expected import of %q to fail", tc.id)
			}
		})
	}
}

func TestImporters_SingletonsUseFixedID(t *testing.T) {
	for r, want := range map[*schema.Resource]string{
		resourceBackupAzureSettings(): "portainer-backup-azure-settings",
		resourceBackupLocalSettings(): "portainer-backup-local-settings",
	} {
		d, err := runImport(t, r, "anything")
		if err != nil {
			t.Fatalf("import failed: %v", err)
		}
		if d.Id() != want {
			t.Errorf("expected ID %q, got %q", want, d.Id())
		}
	}
}
