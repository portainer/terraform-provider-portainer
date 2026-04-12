package internal

import (
	"testing"
)

// --------------- edgeConfigTypeDiffSuppress ---------------

func TestEdgeConfigTypeDiffSuppress(t *testing.T) {
	tests := []struct {
		name     string
		old      string
		new      string
		expected bool
	}{
		{
			name:     "numeric old matches string new",
			old:      "1",
			new:      "general",
			expected: true,
		},
		{
			name:     "string old matches numeric new",
			old:      "general",
			new:      "1",
			expected: true,
		},
		{
			name:     "same string",
			old:      "general",
			new:      "general",
			expected: true,
		},
		{
			name: "same numeric string known type returns false",
			old:  "1",
			new:  "1",
			// old=1 maps to "general", compares "general"=="1" -> false
			expected: false,
		},
		{
			name:     "different strings",
			old:      "general",
			new:      "specific",
			expected: false,
		},
		{
			name:     "unknown numeric old",
			old:      "99",
			new:      "general",
			expected: false,
		},
		{
			name:     "unknown numeric new",
			old:      "general",
			new:      "99",
			expected: false,
		},
		{
			name:     "both unknown numerics",
			old:      "99",
			new:      "88",
			expected: false,
		},
		{
			name:     "empty strings",
			old:      "",
			new:      "",
			expected: true,
		},
		{
			name:     "old empty new general",
			old:      "",
			new:      "general",
			expected: false,
		},
		{
			name:     "numeric 0 not in map",
			old:      "0",
			new:      "general",
			expected: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := edgeConfigTypeDiffSuppress("type", tt.old, tt.new, nil)
			if result != tt.expected {
				t.Errorf("edgeConfigTypeDiffSuppress(_, %q, %q, _) = %v, want %v", tt.old, tt.new, result, tt.expected)
			}
		})
	}
}

// --------------- edgeConfigTypeToString map ---------------

func TestEdgeConfigTypeToStringMap(t *testing.T) {
	t.Run("key 1 maps to general", func(t *testing.T) {
		val, ok := edgeConfigTypeToString[1]
		if !ok {
			t.Fatal("expected key 1 to exist in edgeConfigTypeToString")
		}
		if val != "general" {
			t.Errorf("expected 'general', got %q", val)
		}
	})

	t.Run("key 0 does not exist", func(t *testing.T) {
		_, ok := edgeConfigTypeToString[0]
		if ok {
			t.Error("expected key 0 to not exist in edgeConfigTypeToString")
		}
	})

	t.Run("key 2 does not exist", func(t *testing.T) {
		_, ok := edgeConfigTypeToString[2]
		if ok {
			t.Error("expected key 2 to not exist in edgeConfigTypeToString")
		}
	})

	t.Run("map has exactly one entry", func(t *testing.T) {
		if len(edgeConfigTypeToString) != 1 {
			t.Errorf("expected map length 1, got %d", len(edgeConfigTypeToString))
		}
	})
}

// --------------- convertToIntSlice ---------------

func TestConvertToIntSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []interface{}
		expected []int
	}{
		{"normal", []interface{}{1, 2, 3}, []int{1, 2, 3}},
		{"empty", []interface{}{}, []int{}},
		{"single", []interface{}{42}, []int{42}},
		{"negative values", []interface{}{-5, -10}, []int{-5, -10}},
		{"zero", []interface{}{0}, []int{0}},
		{"large slice", []interface{}{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}, []int{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := convertToIntSlice(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected length %d, got %d", len(tt.expected), len(result))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("at index %d: expected %d, got %d", i, tt.expected[i], v)
				}
			}
		})
	}
}
