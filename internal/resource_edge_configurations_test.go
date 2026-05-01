package internal

import (
	"os"
	"path/filepath"
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

// --------------- sha256File ---------------

func TestSha256File(t *testing.T) {
	dir := t.TempDir()

	t.Run("known content produces expected hash", func(t *testing.T) {
		// "abc" → sha256 = ba7816bf... (RFC 6234 test vector)
		path := filepath.Join(dir, "abc.txt")
		if err := os.WriteFile(path, []byte("abc"), 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		got, err := sha256File(path)
		if err != nil {
			t.Fatalf("sha256File: %v", err)
		}
		want := "ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("empty file produces sha256 of empty input", func(t *testing.T) {
		path := filepath.Join(dir, "empty.txt")
		if err := os.WriteFile(path, []byte{}, 0o600); err != nil {
			t.Fatalf("write: %v", err)
		}
		got, err := sha256File(path)
		if err != nil {
			t.Fatalf("sha256File: %v", err)
		}
		want := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("identical content yields identical hash regardless of path", func(t *testing.T) {
		// Reproduces the issue #116 motivation: content-based identity, not
		// path-based.
		a := filepath.Join(dir, "a.txt")
		b := filepath.Join(dir, "b.txt")
		payload := []byte("same content, different filename")
		if err := os.WriteFile(a, payload, 0o600); err != nil {
			t.Fatalf("write a: %v", err)
		}
		if err := os.WriteFile(b, payload, 0o600); err != nil {
			t.Fatalf("write b: %v", err)
		}
		ha, err := sha256File(a)
		if err != nil {
			t.Fatalf("hash a: %v", err)
		}
		hb, err := sha256File(b)
		if err != nil {
			t.Fatalf("hash b: %v", err)
		}
		if ha != hb {
			t.Errorf("expected equal hashes, got %q vs %q", ha, hb)
		}
	})

	t.Run("changed content yields different hash for same path", func(t *testing.T) {
		// Reproduces issue #116 directly: same path, mutated bytes — the
		// provider must see a diff.
		path := filepath.Join(dir, "mutating.txt")
		if err := os.WriteFile(path, []byte("v1"), 0o600); err != nil {
			t.Fatalf("write v1: %v", err)
		}
		h1, err := sha256File(path)
		if err != nil {
			t.Fatalf("hash v1: %v", err)
		}
		if err := os.WriteFile(path, []byte("v2"), 0o600); err != nil {
			t.Fatalf("write v2: %v", err)
		}
		h2, err := sha256File(path)
		if err != nil {
			t.Fatalf("hash v2: %v", err)
		}
		if h1 == h2 {
			t.Errorf("expected different hashes, both got %q", h1)
		}
	})

	t.Run("missing file returns error", func(t *testing.T) {
		if _, err := sha256File(filepath.Join(dir, "does-not-exist")); err == nil {
			t.Error("expected error, got nil")
		}
	})
}

// --------------- resolveCreatedEdgeConfigID ---------------

func TestResolveCreatedEdgeConfigID(t *testing.T) {
	tests := []struct {
		name           string
		configs        []EdgeConfiguration
		configName     string
		preExistingIDs map[int]struct{}
		wantID         int
		wantErr        bool
	}{
		{
			name: "single new entry, pre-existing same-name config is ignored",
			// Reproduces issue #115: a same-name config existed before create.
			// The pre-existing one (ID 7) must NOT be picked.
			configs: []EdgeConfiguration{
				{ID: 7, Name: "Test", Created: 1000},
				{ID: 9, Name: "Test", Created: 2000},
			},
			configName:     "Test",
			preExistingIDs: map[int]struct{}{7: {}},
			wantID:         9,
		},
		{
			name: "no pre-existing, single match returns that match",
			configs: []EdgeConfiguration{
				{ID: 5, Name: "Test", Created: 1000},
			},
			configName:     "Test",
			preExistingIDs: map[int]struct{}{},
			wantID:         5,
		},
		{
			name: "multiple new entries with same name, newest by created wins",
			configs: []EdgeConfiguration{
				{ID: 10, Name: "Test", Created: 1000},
				{ID: 11, Name: "Test", Created: 3000},
				{ID: 12, Name: "Test", Created: 2000},
			},
			configName:     "Test",
			preExistingIDs: map[int]struct{}{},
			wantID:         11,
		},
		{
			name: "no new entries, falls back to most recently created same-name",
			// Server returned no new entry (e.g. replication lag). Best-effort
			// fallback to the most recently created matching name.
			configs: []EdgeConfiguration{
				{ID: 1, Name: "Test", Created: 100},
				{ID: 2, Name: "Test", Created: 500},
			},
			configName:     "Test",
			preExistingIDs: map[int]struct{}{1: {}, 2: {}},
			wantID:         2,
		},
		{
			name: "no matching name returns error",
			configs: []EdgeConfiguration{
				{ID: 1, Name: "Other", Created: 100},
			},
			configName:     "Test",
			preExistingIDs: map[int]struct{}{},
			wantErr:        true,
		},
		{
			name:           "empty list returns error",
			configs:        nil,
			configName:     "Test",
			preExistingIDs: map[int]struct{}{},
			wantErr:        true,
		},
		{
			name: "ignores entries with non-matching names",
			configs: []EdgeConfiguration{
				{ID: 1, Name: "Other", Created: 9999},
				{ID: 2, Name: "Test", Created: 100},
			},
			configName:     "Test",
			preExistingIDs: map[int]struct{}{},
			wantID:         2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := resolveCreatedEdgeConfigID(tt.configs, tt.configName, tt.preExistingIDs)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil (got=%v)", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.ID != tt.wantID {
				t.Errorf("got ID %d, want %d", got.ID, tt.wantID)
			}
		})
	}
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
