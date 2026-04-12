package internal

import (
	"testing"
)

// --------------- convertMapString ---------------

func TestConvertMapString(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]string
	}{
		{
			name:     "string values",
			input:    map[string]interface{}{"driver": "local", "type": "nfs"},
			expected: map[string]string{"driver": "local", "type": "nfs"},
		},
		{
			name:     "integer values",
			input:    map[string]interface{}{"size": 100},
			expected: map[string]string{"size": "100"},
		},
		{
			name:     "empty map",
			input:    map[string]interface{}{},
			expected: map[string]string{},
		},
		{
			name:     "boolean value",
			input:    map[string]interface{}{"encrypted": true},
			expected: map[string]string{"encrypted": "true"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertMapString(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected length %d, got %d", len(tt.expected), len(result))
			}
			for k, v := range tt.expected {
				if result[k] != v {
					t.Errorf("key %q: expected %q, got %q", k, v, result[k])
				}
			}
		})
	}
}

// --------------- expandClusterVolumeSpec ---------------

func TestExpandClusterVolumeSpec(t *testing.T) {
	t.Run("normal input", func(t *testing.T) {
		input := map[string]interface{}{
			"group":        "test-group",
			"availability": "active",
		}
		result := expandClusterVolumeSpec(input)
		if result == nil {
			t.Fatal("expected non-nil result")
			return
		}
		if result.Group != "test-group" {
			t.Errorf("expected Group='test-group', got %q", result.Group)
		}
		if result.Availability != "active" {
			t.Errorf("expected Availability='active', got %q", result.Availability)
		}
	})

	t.Run("empty strings", func(t *testing.T) {
		input := map[string]interface{}{
			"group":        "",
			"availability": "",
		}
		result := expandClusterVolumeSpec(input)
		if result == nil {
			t.Fatal("expected non-nil result")
			return
		}
		if result.Group != "" {
			t.Errorf("expected empty Group, got %q", result.Group)
		}
		if result.Availability != "" {
			t.Errorf("expected empty Availability, got %q", result.Availability)
		}
	})
}
