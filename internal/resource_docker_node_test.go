package internal

import (
	"testing"
)

// --------------- convertMapsString ---------------

func TestConvertMapsString(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]interface{}
		expected map[string]string
	}{
		{
			name:     "string values",
			input:    map[string]interface{}{"key1": "val1", "key2": "val2"},
			expected: map[string]string{"key1": "val1", "key2": "val2"},
		},
		{
			name:     "integer values",
			input:    map[string]interface{}{"port": 8080, "count": 3},
			expected: map[string]string{"port": "8080", "count": "3"},
		},
		{
			name:     "boolean values",
			input:    map[string]interface{}{"enabled": true, "debug": false},
			expected: map[string]string{"enabled": "true", "debug": "false"},
		},
		{
			name:     "mixed types",
			input:    map[string]interface{}{"name": "test", "port": 443, "ssl": true},
			expected: map[string]string{"name": "test", "port": "443", "ssl": "true"},
		},
		{
			name:     "empty map",
			input:    map[string]interface{}{},
			expected: map[string]string{},
		},
		{
			name:     "single entry",
			input:    map[string]interface{}{"only": "one"},
			expected: map[string]string{"only": "one"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertMapsString(tt.input)
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
