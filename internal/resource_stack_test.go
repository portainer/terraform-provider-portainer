package internal

import (
	"testing"
)

// --------------- expandStringList ---------------

func TestExpandStringList(t *testing.T) {
	tests := []struct {
		name     string
		input    []interface{}
		expected []string
	}{
		{"normal", []interface{}{"a", "b", "c"}, []string{"a", "b", "c"}},
		{"empty", []interface{}{}, []string{}},
		{"single", []interface{}{"hello"}, []string{"hello"}},
		{"with empty strings", []interface{}{"", "a", ""}, []string{"", "a", ""}},
		{"unicode strings", []interface{}{"hello", "welt", "swiat"}, []string{"hello", "welt", "swiat"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandStringList(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected length %d, got %d", len(tt.expected), len(result))
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("at index %d: expected %q, got %q", i, tt.expected[i], v)
				}
			}
		})
	}
}

// --------------- expandIntList ---------------

func TestExpandIntList(t *testing.T) {
	tests := []struct {
		name     string
		input    []interface{}
		expected []int
	}{
		{"normal", []interface{}{1, 2, 3}, []int{1, 2, 3}},
		{"empty", []interface{}{}, []int{}},
		{"single", []interface{}{99}, []int{99}},
		{"with zero", []interface{}{0, 1, 0}, []int{0, 1, 0}},
		{"negative", []interface{}{-1, -2, -3}, []int{-1, -2, -3}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := expandIntList(tt.input)
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

// --------------- flattenEnvList ---------------

func TestFlattenEnvList(t *testing.T) {
	t.Run("normal env list", func(t *testing.T) {
		input := []interface{}{
			map[string]interface{}{"name": "FOO", "value": "bar"},
			map[string]interface{}{"name": "BAZ", "value": "qux"},
		}
		result := flattenEnvList(input)
		if len(result) != 2 {
			t.Fatalf("expected 2 entries, got %d", len(result))
		}
		if result[0]["name"] != "FOO" || result[0]["value"] != "bar" {
			t.Errorf("entry 0: expected FOO=bar, got %v", result[0])
		}
		if result[1]["name"] != "BAZ" || result[1]["value"] != "qux" {
			t.Errorf("entry 1: expected BAZ=qux, got %v", result[1])
		}
	})

	t.Run("empty env list", func(t *testing.T) {
		input := []interface{}{}
		result := flattenEnvList(input)
		if len(result) != 0 {
			t.Errorf("expected nil or empty, got %v", result)
		}
	})

	t.Run("single env entry", func(t *testing.T) {
		input := []interface{}{
			map[string]interface{}{"name": "KEY", "value": "val"},
		}
		result := flattenEnvList(input)
		if len(result) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(result))
		}
		if result[0]["name"] != "KEY" {
			t.Errorf("expected name=KEY, got %q", result[0]["name"])
		}
		if result[0]["value"] != "val" {
			t.Errorf("expected value=val, got %q", result[0]["value"])
		}
	})

	t.Run("env with empty values", func(t *testing.T) {
		input := []interface{}{
			map[string]interface{}{"name": "", "value": ""},
		}
		result := flattenEnvList(input)
		if len(result) != 1 {
			t.Fatalf("expected 1 entry, got %d", len(result))
		}
		if result[0]["name"] != "" {
			t.Errorf("expected empty name, got %q", result[0]["name"])
		}
		if result[0]["value"] != "" {
			t.Errorf("expected empty value, got %q", result[0]["value"])
		}
	})
}
