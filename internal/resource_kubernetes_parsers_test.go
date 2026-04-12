package internal

import (
	"testing"
)

// --------------- parseClusterRolesID ---------------

func TestParseClusterRolesID(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantEndpointID int
		wantName       string
	}{
		{"valid", "5:my-role", 5, "my-role"},
		{"large endpoint ID", "12345:admin-role", 12345, "admin-role"},
		{"zero endpoint ID", "0:role-name", 0, "role-name"},
		{"missing name", "5", 0, ""},
		{"empty string", "", 0, ""},
		{"name with special chars", "3:my-role-with-dashes", 3, "my-role-with-dashes"},
		{"three parts uses first two", "1:name:extra", 1, "name"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpointID, name := parseClusterRolesID(tt.input)
			if endpointID != tt.wantEndpointID {
				t.Errorf("endpointID: expected %d, got %d", tt.wantEndpointID, endpointID)
			}
			if name != tt.wantName {
				t.Errorf("name: expected %q, got %q", tt.wantName, name)
			}
		})
	}
}

// --------------- parseClusterRolesBindingsID ---------------

func TestParseClusterRolesBindingsID(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantEndpointID int
		wantName       string
	}{
		{"valid", "5:my-binding", 5, "my-binding"},
		{"large endpoint ID", "99999:binding", 99999, "binding"},
		{"missing name", "5", 0, ""},
		{"empty string", "", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpointID, name := parseClusterRolesBindingsID(tt.input)
			if endpointID != tt.wantEndpointID {
				t.Errorf("endpointID: expected %d, got %d", tt.wantEndpointID, endpointID)
			}
			if name != tt.wantName {
				t.Errorf("name: expected %q, got %q", tt.wantName, name)
			}
		})
	}
}

// --------------- parseSecretsID ---------------

func TestParseSecretsID(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantEndpointID int
		wantNamespace  string
		wantName       string
	}{
		{"valid", "1:default:my-secret", 1, "default", "my-secret"},
		{"kube-system namespace", "5:kube-system:tls-cert", 5, "kube-system", "tls-cert"},
		{"missing parts returns zeros", "1:default", 0, "", ""},
		{"single part returns zeros", "1", 0, "", ""},
		{"empty string returns zeros", "", 0, "", ""},
		{"name with colons in third part", "1:ns:name:extra", 1, "ns", "name:extra"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpointID, namespace, name := parseSecretsID(tt.input)
			if endpointID != tt.wantEndpointID {
				t.Errorf("endpointID: expected %d, got %d", tt.wantEndpointID, endpointID)
			}
			if namespace != tt.wantNamespace {
				t.Errorf("namespace: expected %q, got %q", tt.wantNamespace, namespace)
			}
			if name != tt.wantName {
				t.Errorf("name: expected %q, got %q", tt.wantName, name)
			}
		})
	}
}

// --------------- parseConfigMapsID ---------------

func TestParseConfigMapsID(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantEndpointID int
		wantNamespace  string
		wantName       string
	}{
		{"valid", "2:default:my-configmap", 2, "default", "my-configmap"},
		{"missing parts", "2:default", 0, "", ""},
		{"empty", "", 0, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpointID, namespace, name := parseConfigMapsID(tt.input)
			if endpointID != tt.wantEndpointID {
				t.Errorf("endpointID: expected %d, got %d", tt.wantEndpointID, endpointID)
			}
			if namespace != tt.wantNamespace {
				t.Errorf("namespace: expected %q, got %q", tt.wantNamespace, namespace)
			}
			if name != tt.wantName {
				t.Errorf("name: expected %q, got %q", tt.wantName, name)
			}
		})
	}
}

// --------------- parseCronJobID ---------------

func TestParseCronJobID(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantEndpointID int
		wantNamespace  string
		wantName       string
	}{
		{"valid", "3:production:nightly-backup", 3, "production", "nightly-backup"},
		{"missing parts", "3", 0, "", ""},
		{"empty", "", 0, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpointID, namespace, name := parseCronJobID(tt.input)
			if endpointID != tt.wantEndpointID {
				t.Errorf("endpointID: expected %d, got %d", tt.wantEndpointID, endpointID)
			}
			if namespace != tt.wantNamespace {
				t.Errorf("namespace: expected %q, got %q", tt.wantNamespace, namespace)
			}
			if name != tt.wantName {
				t.Errorf("name: expected %q, got %q", tt.wantName, name)
			}
		})
	}
}

// --------------- parseJobID ---------------

func TestParseJobID(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantEndpointID int
		wantNamespace  string
		wantName       string
	}{
		{"valid", "10:staging:db-migration", 10, "staging", "db-migration"},
		{"missing parts", "10:staging", 0, "", ""},
		{"empty", "", 0, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpointID, namespace, name := parseJobID(tt.input)
			if endpointID != tt.wantEndpointID {
				t.Errorf("endpointID: expected %d, got %d", tt.wantEndpointID, endpointID)
			}
			if namespace != tt.wantNamespace {
				t.Errorf("namespace: expected %q, got %q", tt.wantNamespace, namespace)
			}
			if name != tt.wantName {
				t.Errorf("name: expected %q, got %q", tt.wantName, name)
			}
		})
	}
}

// --------------- parseRolesID ---------------

func TestParseRolesID(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantEndpointID int
		wantNamespace  string
		wantName       string
	}{
		{"valid", "7:default:pod-reader", 7, "default", "pod-reader"},
		{"missing parts", "7", 0, "", ""},
		{"empty", "", 0, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpointID, namespace, name := parseRolesID(tt.input)
			if endpointID != tt.wantEndpointID {
				t.Errorf("endpointID: expected %d, got %d", tt.wantEndpointID, endpointID)
			}
			if namespace != tt.wantNamespace {
				t.Errorf("namespace: expected %q, got %q", tt.wantNamespace, namespace)
			}
			if name != tt.wantName {
				t.Errorf("name: expected %q, got %q", tt.wantName, name)
			}
		})
	}
}

// --------------- parseRoleBindingsID ---------------

func TestParseRoleBindingsID(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantEndpointID int
		wantNamespace  string
		wantName       string
	}{
		{"valid", "4:kube-system:admin-binding", 4, "kube-system", "admin-binding"},
		{"missing parts", "4:ns", 0, "", ""},
		{"empty", "", 0, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpointID, namespace, name := parseRoleBindingsID(tt.input)
			if endpointID != tt.wantEndpointID {
				t.Errorf("endpointID: expected %d, got %d", tt.wantEndpointID, endpointID)
			}
			if namespace != tt.wantNamespace {
				t.Errorf("namespace: expected %q, got %q", tt.wantNamespace, namespace)
			}
			if name != tt.wantName {
				t.Errorf("name: expected %q, got %q", tt.wantName, name)
			}
		})
	}
}

// --------------- parseServiceID ---------------

func TestParseServiceID(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantEndpointID int
		wantNamespace  string
		wantName       string
	}{
		{"valid", "1:default:my-service", 1, "default", "my-service"},
		{"missing parts", "1", 0, "", ""},
		{"empty", "", 0, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpointID, namespace, name := parseServiceID(tt.input)
			if endpointID != tt.wantEndpointID {
				t.Errorf("endpointID: expected %d, got %d", tt.wantEndpointID, endpointID)
			}
			if namespace != tt.wantNamespace {
				t.Errorf("namespace: expected %q, got %q", tt.wantNamespace, namespace)
			}
			if name != tt.wantName {
				t.Errorf("name: expected %q, got %q", tt.wantName, name)
			}
		})
	}
}

// --------------- parseServiceAccountsID ---------------

func TestParseServiceAccountsID(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantEndpointID int
		wantNamespace  string
		wantName       string
	}{
		{"valid", "2:monitoring:prometheus", 2, "monitoring", "prometheus"},
		{"missing parts", "2:monitoring", 0, "", ""},
		{"empty", "", 0, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpointID, namespace, name := parseServiceAccountsID(tt.input)
			if endpointID != tt.wantEndpointID {
				t.Errorf("endpointID: expected %d, got %d", tt.wantEndpointID, endpointID)
			}
			if namespace != tt.wantNamespace {
				t.Errorf("namespace: expected %q, got %q", tt.wantNamespace, namespace)
			}
			if name != tt.wantName {
				t.Errorf("name: expected %q, got %q", tt.wantName, name)
			}
		})
	}
}

// --------------- parseApllicationsID (sic - typo in original) ---------------

func TestParseApllicationsID(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantEndpointID int
		wantNamespace  string
		wantName       string
	}{
		{"valid", "6:default:nginx", 6, "default", "nginx"},
		{"missing parts", "6:default", 0, "", ""},
		{"empty", "", 0, "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpointID, namespace, name := parseApllicationsID(tt.input)
			if endpointID != tt.wantEndpointID {
				t.Errorf("endpointID: expected %d, got %d", tt.wantEndpointID, endpointID)
			}
			if namespace != tt.wantNamespace {
				t.Errorf("namespace: expected %q, got %q", tt.wantNamespace, namespace)
			}
			if name != tt.wantName {
				t.Errorf("name: expected %q, got %q", tt.wantName, name)
			}
		})
	}
}

// --------------- parseStorageID ---------------

func TestParseStorageID(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantEndpointID int
		wantName       string
	}{
		{"valid", "3:local-storage", 3, "local-storage"},
		{"missing name", "3", 0, ""},
		{"empty", "", 0, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpointID, name := parseStorageID(tt.input)
			if endpointID != tt.wantEndpointID {
				t.Errorf("endpointID: expected %d, got %d", tt.wantEndpointID, endpointID)
			}
			if name != tt.wantName {
				t.Errorf("name: expected %q, got %q", tt.wantName, name)
			}
		})
	}
}

// --------------- parseVolumesID ---------------

func TestParseVolumesID(t *testing.T) {
	tests := []struct {
		name           string
		input          string
		wantEndpointID int
		wantNamespace  string
		wantVolType    string
		wantName       string
	}{
		{
			name:           "valid PVC",
			input:          "1:default:persistent-volume-claim:my-pvc",
			wantEndpointID: 1,
			wantNamespace:  "default",
			wantVolType:    "persistent-volume-claim",
			wantName:       "my-pvc",
		},
		{
			name:           "valid PV",
			input:          "2:kube-system:persistent-volume:data-pv",
			wantEndpointID: 2,
			wantNamespace:  "kube-system",
			wantVolType:    "persistent-volume",
			wantName:       "data-pv",
		},
		{
			name:           "missing parts",
			input:          "1:default:pvc",
			wantEndpointID: 0,
			wantNamespace:  "",
			wantVolType:    "",
			wantName:       "",
		},
		{
			name:           "empty",
			input:          "",
			wantEndpointID: 0,
			wantNamespace:  "",
			wantVolType:    "",
			wantName:       "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			endpointID, namespace, volType, name := parseVolumesID(tt.input)
			if endpointID != tt.wantEndpointID {
				t.Errorf("endpointID: expected %d, got %d", tt.wantEndpointID, endpointID)
			}
			if namespace != tt.wantNamespace {
				t.Errorf("namespace: expected %q, got %q", tt.wantNamespace, namespace)
			}
			if volType != tt.wantVolType {
				t.Errorf("volType: expected %q, got %q", tt.wantVolType, volType)
			}
			if name != tt.wantName {
				t.Errorf("name: expected %q, got %q", tt.wantName, name)
			}
		})
	}
}
