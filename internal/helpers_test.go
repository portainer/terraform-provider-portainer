package internal

import (
	"testing"
)

// --------------- toIntSlice ---------------

func TestToIntSlice(t *testing.T) {
	tests := []struct {
		name     string
		input    []interface{}
		expected []int
	}{
		{"normal", []interface{}{1, 2, 3}, []int{1, 2, 3}},
		{"empty", []interface{}{}, []int{}},
		{"single", []interface{}{42}, []int{42}},
		{"negative values", []interface{}{-1, 0, 1}, []int{-1, 0, 1}},
		{"large values", []interface{}{1000000, 2000000}, []int{1000000, 2000000}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toIntSlice(tt.input)
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

// --------------- splitAndTrimCSV ---------------

func TestSplitAndTrimCSV(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
	}{
		{"normal", "a,b,c", []string{"a", "b", "c"}},
		{"with spaces", " a , b , c ", []string{"a", "b", "c"}},
		{"single value", "hello", []string{"hello"}},
		{"empty parts filtered", "a,,b", []string{"a", "b"}},
		{"only commas", ",,,", []string{}},
		{"empty string", "", []string{}},
		{"spaces only between commas", " , , ", []string{}},
		{"mixed spacing", "foo,  bar  ,baz", []string{"foo", "bar", "baz"}},
		{"trailing comma", "a,b,", []string{"a", "b"}},
		{"leading comma", ",a,b", []string{"a", "b"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := splitAndTrimCSV(tt.input)
			if len(result) != len(tt.expected) {
				t.Fatalf("expected length %d, got %d; result=%v", len(tt.expected), len(result), result)
			}
			for i, v := range result {
				if v != tt.expected[i] {
					t.Errorf("at index %d: expected %q, got %q", i, tt.expected[i], v)
				}
			}
		})
	}
}

// --------------- contains ---------------

func TestContains(t *testing.T) {
	tests := []struct {
		name     string
		arr      []string
		val      string
		expected bool
	}{
		{"found", []string{"a", "b", "c"}, "b", true},
		{"not found", []string{"a", "b", "c"}, "d", false},
		{"empty slice", []string{}, "a", false},
		{"single element found", []string{"x"}, "x", true},
		{"single element not found", []string{"x"}, "y", false},
		{"empty string in slice", []string{""}, "", true},
		{"empty string not in slice", []string{"a"}, "", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := contains(tt.arr, tt.val)
			if result != tt.expected {
				t.Errorf("contains(%v, %q) = %v, want %v", tt.arr, tt.val, result, tt.expected)
			}
		})
	}
}

// --------------- mustMap ---------------

func TestMustMap(t *testing.T) {
	t.Run("nil returns empty map", func(t *testing.T) {
		result := mustMap(nil)
		if result == nil {
			t.Fatal("expected non-nil map, got nil")
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got length %d", len(result))
		}
	})

	t.Run("valid map returns same map", func(t *testing.T) {
		input := map[string]interface{}{"key": "value"}
		result := mustMap(input)
		if result == nil {
			t.Fatal("expected non-nil map")
		}
		if result["key"] != "value" {
			t.Errorf("expected key=value, got key=%v", result["key"])
		}
	})

	t.Run("non-map returns empty map", func(t *testing.T) {
		result := mustMap("not a map")
		if result == nil {
			t.Fatal("expected non-nil map, got nil")
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got length %d", len(result))
		}
	})

	t.Run("integer returns empty map", func(t *testing.T) {
		result := mustMap(42)
		if result == nil {
			t.Fatal("expected non-nil map, got nil")
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got length %d", len(result))
		}
	})

	t.Run("empty map returns empty map", func(t *testing.T) {
		input := map[string]interface{}{}
		result := mustMap(input)
		if result == nil {
			t.Fatal("expected non-nil map, got nil")
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got length %d", len(result))
		}
	})
}

// --------------- parseManifest ---------------

func TestParseManifest(t *testing.T) {
	t.Run("valid JSON", func(t *testing.T) {
		input := `{"apiVersion": "v1", "kind": "Pod"}`
		result, err := parseManifest(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result["apiVersion"] != "v1" {
			t.Errorf("expected apiVersion=v1, got %v", result["apiVersion"])
		}
		if result["kind"] != "Pod" {
			t.Errorf("expected kind=Pod, got %v", result["kind"])
		}
	})

	t.Run("valid YAML", func(t *testing.T) {
		input := "apiVersion: v1\nkind: Service\n"
		result, err := parseManifest(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result["apiVersion"] != "v1" {
			t.Errorf("expected apiVersion=v1, got %v", result["apiVersion"])
		}
		if result["kind"] != "Service" {
			t.Errorf("expected kind=Service, got %v", result["kind"])
		}
	})

	t.Run("empty JSON object", func(t *testing.T) {
		input := `{}`
		result, err := parseManifest(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(result) != 0 {
			t.Errorf("expected empty map, got %v", result)
		}
	})

	t.Run("empty string returns nil map", func(t *testing.T) {
		// YAML library parses empty string as nil without error,
		// so parseManifest returns (nil, nil) for empty input.
		result, err := parseManifest("")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result != nil {
			t.Errorf("expected nil result, got %v", result)
		}
	})

	t.Run("JSON with nested structure", func(t *testing.T) {
		input := `{"metadata": {"name": "test", "labels": {"app": "web"}}}`
		result, err := parseManifest(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result["metadata"] == nil {
			t.Error("expected metadata key to exist")
		}
	})

	t.Run("YAML multiline", func(t *testing.T) {
		input := "apiVersion: apps/v1\nkind: Deployment\nmetadata:\n  name: nginx\n"
		result, err := parseManifest(input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if result["apiVersion"] != "apps/v1" {
			t.Errorf("expected apiVersion='apps/v1', got %v", result["apiVersion"])
		}
		if result["kind"] != "Deployment" {
			t.Errorf("expected kind='Deployment', got %v", result["kind"])
		}
	})
}
