package internal

import (
	"strings"
	"testing"
)

// --------------- volumeAPIURL ---------------

func TestVolumeAPIURL(t *testing.T) {
	base := "https://portainer.example.com/api"

	tests := []struct {
		name      string
		base      string
		endpoint  int
		namespace string
		volType   string
		withName  bool
		nameArgs  []string
		wantURL   string
		wantErr   bool
	}{
		{
			name:      "PVC without name",
			base:      base,
			endpoint:  1,
			namespace: "default",
			volType:   "persistent-volume-claim",
			withName:  false,
			wantURL:   base + "/endpoints/1/kubernetes/api/v1/namespaces/default/persistentvolumeclaims",
		},
		{
			name:      "PVC with name",
			base:      base,
			endpoint:  1,
			namespace: "default",
			volType:   "persistent-volume-claim",
			withName:  true,
			nameArgs:  []string{"my-pvc"},
			wantURL:   base + "/endpoints/1/kubernetes/api/v1/namespaces/default/persistentvolumeclaims/my-pvc",
		},
		{
			name:      "PV without name",
			base:      base,
			endpoint:  2,
			namespace: "kube-system",
			volType:   "persistent-volume",
			withName:  false,
			wantURL:   base + "/endpoints/2/kubernetes/api/v1/persistentvolumes",
		},
		{
			name:      "PV with name",
			base:      base,
			endpoint:  2,
			namespace: "kube-system",
			volType:   "persistent-volume",
			withName:  true,
			nameArgs:  []string{"data-pv"},
			wantURL:   base + "/endpoints/2/kubernetes/api/v1/persistentvolumes/data-pv",
		},
		{
			name:      "volume-attachment without name",
			base:      base,
			endpoint:  3,
			namespace: "default",
			volType:   "volume-attachment",
			withName:  false,
			wantURL:   base + "/endpoints/3/kubernetes/apis/storage.k8s.io/v1/volumeattachments",
		},
		{
			name:      "volume-attachment with name",
			base:      base,
			endpoint:  3,
			namespace: "default",
			volType:   "volume-attachment",
			withName:  true,
			nameArgs:  []string{"my-va"},
			wantURL:   base + "/endpoints/3/kubernetes/apis/storage.k8s.io/v1/volumeattachments/my-va",
		},
		{
			name:      "unsupported type",
			base:      base,
			endpoint:  1,
			namespace: "default",
			volType:   "unknown-type",
			withName:  false,
			wantErr:   true,
		},
		{
			name:      "empty type",
			base:      base,
			endpoint:  1,
			namespace: "default",
			volType:   "",
			withName:  false,
			wantErr:   true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := volumeAPIURL(tt.base, tt.endpoint, tt.namespace, tt.volType, tt.withName, tt.nameArgs...)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), "unsupported volume type") {
					t.Errorf("expected 'unsupported volume type' in error, got %q", err.Error())
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result != tt.wantURL {
				t.Errorf("expected URL %q, got %q", tt.wantURL, result)
			}
		})
	}
}
