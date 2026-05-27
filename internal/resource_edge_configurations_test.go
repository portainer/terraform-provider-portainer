package internal

import (
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

// --------------- regression: issue #119 ---------------

// TestEdgeConfigurationsUpdate_FormFieldsRegression119 is a regression test
// for issue #119.
//
// Before the fix, the PUT /edge_configurations/{id} multipart body sent
// separate form fields named "EdgeGroupIDs" and "Type" (capitalized), plus a
// "File" part. The Portainer API actually expects a single JSON form field
// named "edgeConfiguration" (lowercase, with camelCase keys inside) and a
// "file" part (lowercase) — see resource_edge_configurations.go:309-327.
//
// This test mounts a temp file as the upload, drives Update, and inspects the
// recorded multipart body to assert:
//   - Content-Type is multipart
//   - body contains the "edgeConfiguration" form field name
//   - body does NOT contain the old capitalized "EdgeGroupIDs" field name
//   - the JSON inside "edgeConfiguration" has the camelCase "edgeGroupIDs"
//     and "type" keys (the API's expectation)
//   - the file part is named "file" (lowercase), not "File"
func TestEdgeConfigurationsUpdate_FormFieldsRegression119(t *testing.T) {
	mock := NewMockServer(t)

	// Prepare a small file to upload.
	dir := t.TempDir()
	filePath := filepath.Join(dir, "config.txt")
	if err := os.WriteFile(filePath, []byte("hello-edge-config"), 0o600); err != nil {
		t.Fatalf("write tmp file: %v", err)
	}

	// Capture state observed by the inline handler so assertions live in the
	// test body where t.Errorf has clearer scope.
	var (
		gotContentType    string
		gotEdgeConfigJSON string
		gotFilePartName   string
		gotRawBody        []byte
		extraFieldNames   []string
	)

	mock.On("PUT", "/edge_configurations/7", func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		gotRawBody = raw
		gotContentType = r.Header.Get("Content-Type")

		mediaType, params, err := mime.ParseMediaType(gotContentType)
		if err != nil {
			t.Errorf("parse Content-Type: %v", err)
		}
		if !strings.HasPrefix(mediaType, "multipart/") {
			t.Errorf("Content-Type: expected multipart, got %q", mediaType)
		}

		mr := multipart.NewReader(strings.NewReader(string(raw)), params["boundary"])
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			name := part.FormName()
			switch name {
			case "edgeConfiguration":
				b, _ := io.ReadAll(part)
				gotEdgeConfigJSON = string(b)
			case "file":
				gotFilePartName = name
				_, _ = io.Copy(io.Discard, part)
			default:
				extraFieldNames = append(extraFieldNames, name)
				_, _ = io.Copy(io.Discard, part)
			}
		}

		w.WriteHeader(http.StatusOK)
	})

	// Read is called after Update — return a valid response so Update finishes.
	mock.On("GET", "/edge_configurations/7", RespondJSON(http.StatusOK, map[string]interface{}{
		"id":           7,
		"name":         "cfg",
		"type":         1,
		"category":     "configuration",
		"baseDir":      "",
		"edgeGroupIDs": []int{1, 2},
	}))

	r := resourcePortainerEdgeConfigurations()
	d := r.TestResourceData()
	d.SetId("7")
	_ = d.Set("name", "cfg")
	_ = d.Set("type", "general")
	_ = d.Set("category", "configuration")
	_ = d.Set("base_dir", "")
	_ = d.Set("edge_group_ids", []interface{}{1, 2})
	_ = d.Set("file_path", filePath)

	if err := r.Update(d, mock.Client()); err != nil {
		t.Fatalf("Update failed: %v", err)
	}

	// --- assertions ---

	if gotContentType == "" {
		t.Fatal("handler never observed the PUT request")
	}

	// The form field name must be the lowercase "edgeConfiguration". Old
	// behavior shipped capitalized "EdgeGroupIDs" and "Type" as separate
	// fields — those names must NOT be present in the raw body at all.
	bodyStr := string(gotRawBody)
	if !strings.Contains(bodyStr, `name="edgeConfiguration"`) {
		t.Error("regression of issue #119: expected form field name=\"edgeConfiguration\" in multipart body")
	}
	if strings.Contains(bodyStr, `name="EdgeGroupIDs"`) {
		t.Error("regression of issue #119: form must NOT contain separate field name=\"EdgeGroupIDs\"")
	}
	if strings.Contains(bodyStr, `name="Type"`) {
		t.Error("regression of issue #119: form must NOT contain separate field name=\"Type\"")
	}
	if strings.Contains(bodyStr, `name="File"`) {
		t.Error("regression of issue #119: file part must be lowercase name=\"file\", not name=\"File\"")
	}
	if gotFilePartName != "file" {
		t.Errorf("regression of issue #119: expected file part name %q, got %q", "file", gotFilePartName)
	}

	// Validate the JSON inside the edgeConfiguration field carries camelCase keys.
	if gotEdgeConfigJSON == "" {
		t.Fatal("regression of issue #119: edgeConfiguration form field was empty")
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(gotEdgeConfigJSON), &payload); err != nil {
		t.Fatalf("edgeConfiguration JSON: %v (raw: %s)", err, gotEdgeConfigJSON)
	}
	if got, ok := payload["type"]; !ok {
		t.Error("regression of issue #119: expected key \"type\" (camelCase) inside edgeConfiguration JSON")
	} else if got != "general" {
		t.Errorf("edgeConfiguration.type: got %v, want %q", got, "general")
	}
	if _, ok := payload["edgeGroupIDs"]; !ok {
		t.Error("regression of issue #119: expected key \"edgeGroupIDs\" (camelCase) inside edgeConfiguration JSON")
	}
	if _, present := payload["EdgeGroupIDs"]; present {
		t.Error("regression of issue #119: edgeConfiguration JSON must NOT carry capitalized \"EdgeGroupIDs\" key")
	}
	if _, present := payload["Type"]; present {
		t.Error("regression of issue #119: edgeConfiguration JSON must NOT carry capitalized \"Type\" key")
	}

	// Any unexpected extra form fields would indicate stale capitalized fields
	// surviving the fix.
	for _, name := range extraFieldNames {
		t.Errorf("regression of issue #119: unexpected extra form field %q in PUT body (only \"edgeConfiguration\" and \"file\" expected)", name)
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
