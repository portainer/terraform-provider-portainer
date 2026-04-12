package internal

import (
	"testing"
)

// --------------- toIntSlices ---------------

func TestToIntSlices(t *testing.T) {
	tests := []struct {
		name     string
		input    []interface{}
		expected []int
	}{
		{"normal", []interface{}{1, 2, 3}, []int{1, 2, 3}},
		{"empty", []interface{}{}, []int{}},
		{"single", []interface{}{42}, []int{42}},
		{"negative values", []interface{}{-1, 0, 1}, []int{-1, 0, 1}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := toIntSlices(tt.input)
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

// --------------- containsColon ---------------

func TestContainsColon(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{"has colon", "abc:def", true},
		{"no colon", "abcdef", false},
		{"empty string", "", false},
		{"only colon", ":", true},
		{"colon at start", ":abc", true},
		{"colon at end", "abc:", true},
		{"multiple colons", "a:b:c", true},
		{"unicode no colon", "hello", false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := containsColon(tt.input)
			if result != tt.expected {
				t.Errorf("containsColon(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}
