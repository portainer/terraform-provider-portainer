package internal

import (
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
)

// fakeEdgeConfigurations is a stateful stand-in for Portainer's edge
// configuration API. Like Portainer, its POST answers with an empty body, so
// the resource has to find the new configuration in the listing.
type fakeEdgeConfigurations struct {
	mu      sync.Mutex
	nextID  int
	configs map[int]map[string]interface{}
	files   map[int]string
	// lastForm holds the multipart parts of the last POST or PUT.
	lastForm map[string]string
}

func parseEdgeConfigurationForm(r *http.Request) map[string]string {
	_, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	mr := multipart.NewReader(r.Body, params["boundary"])
	parts := map[string]string{}
	for {
		part, err := mr.NextPart()
		if err != nil {
			break
		}
		data, _ := io.ReadAll(part)
		parts[part.FormName()] = string(data)
	}
	return parts
}

func newFakeEdgeConfigurations(m *MockServer, ids ...int) *fakeEdgeConfigurations {
	f := &fakeEdgeConfigurations{nextID: 6, configs: map[int]map[string]interface{}{}, files: map[int]string{}}
	typeNumber := func(v interface{}) int {
		if s, ok := v.(string); ok && s == "general" {
			return 1
		}
		n, _ := strconv.Atoi(v.(string))
		return n
	}
	m.On("GET", "/edge_configurations", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		list := make([]map[string]interface{}, 0, len(f.configs))
		for _, c := range f.configs {
			list = append(list, c)
		}
		RespondJSON(http.StatusOK, list)(w, r)
	})
	m.On("POST", "/edge_configurations", func(w http.ResponseWriter, r *http.Request) {
		parts := parseEdgeConfigurationForm(r)
		var payload map[string]interface{}
		_ = json.Unmarshal([]byte(parts["edgeConfiguration"]), &payload)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.lastForm = parts
		f.nextID++
		f.configs[f.nextID] = map[string]interface{}{
			"id": f.nextID, "name": payload["name"], "type": typeNumber(payload["type"]), "category": payload["category"],
			"baseDir": payload["baseDir"], "edgeGroupIDs": payload["edgeGroupIDs"], "created": f.nextID,
		}
		f.files[f.nextID] = parts["file"]
		w.WriteHeader(http.StatusOK)
	})
	for _, id := range ids {
		id := id
		p := "/edge_configurations/" + strconv.Itoa(id)
		m.On("GET", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			c, ok := f.configs[id]
			if !ok {
				RespondString(http.StatusNotFound, "", "not found")(w, r)
				return
			}
			RespondJSON(http.StatusOK, c)(w, r)
		})
		m.On("PUT", p, func(w http.ResponseWriter, r *http.Request) {
			parts := parseEdgeConfigurationForm(r)
			var payload map[string]interface{}
			_ = json.Unmarshal([]byte(parts["edgeConfiguration"]), &payload)
			f.mu.Lock()
			defer f.mu.Unlock()
			f.lastForm = parts
			c := f.configs[id]
			c["type"] = typeNumber(payload["type"])
			c["edgeGroupIDs"] = payload["edgeGroupIDs"]
			f.files[id] = parts["file"]
			w.WriteHeader(http.StatusOK)
		})
		m.On("DELETE", p, func(w http.ResponseWriter, _ *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			delete(f.configs, id)
			w.WriteHeader(http.StatusNoContent)
		})
	}
	return f
}

func edgeConfigurationHCL(fp, typ, groups string) string {
	return `resource "portainer_edge_configurations" "c" {
  name           = "cfg"
  type           = "` + typ + `"
  category       = "configuration"
  edge_group_ids = ` + groups + `
  file_path      = "` + fp + `"
}`
}

func writeEdgeConfigFile(t *testing.T, content string) string {
	t.Helper()
	fp := filepath.Join(t.TempDir(), "cfg.txt")
	if err := os.WriteFile(fp, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	return fp
}

// TestEdgeConfigurationsResource_Lifecycle: create (the ID resolved from the
// listing, a same-name configuration that existed before left alone), a
// re-upload when the file is rewritten in place, import and destroy.
func TestEdgeConfigurationsResource_Lifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeConfigurations(mock, 7)
	// A same-name configuration made outside Terraform (issue #115).
	fake.configs[3] = map[string]interface{}{"id": 3, "name": "cfg", "type": 1, "created": 99}
	fp := writeEdgeConfigFile(t, "payload")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + edgeConfigurationHCL(fp, "general", "[1, 2]"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_edge_configurations.c", "id", "7"),
					resource.TestCheckResourceAttr("portainer_edge_configurations.c", "base_dir", ""),
					resource.TestCheckResourceAttr("portainer_edge_configurations.c", "file_sha256", "239f59ed55e737c77147cf55ad0c1b030b6d7ee748a7426952f9b852d5a935e5"),
				),
			},
			{
				PreConfig: func() {
					if err := os.WriteFile(fp, []byte("payload v2"), 0o600); err != nil {
						t.Fatal(err)
					}
				},
				Config: mockProviderConfig(mock) + edgeConfigurationHCL(fp, "general", "[1, 2]"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_edge_configurations.c", plancheck.ResourceActionUpdate)},
				},
			},
			{
				ResourceName:            "portainer_edge_configurations.c",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"file_path", "file_sha256"},
			},
		},
	})

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if fake.files[7] != "payload v2" {
		t.Errorf("expected the rewritten file to be uploaded, got %q", fake.files[7])
	}
	if _, ok := fake.configs[3]; !ok {
		t.Error("the pre-existing same-name configuration must be left alone")
	}
}

// TestEdgeConfigurationsResource_UpdateFormFieldsRegression119 pins the
// multipart shape of an update: one "edgeConfiguration" JSON field with
// camelCase keys and a lowercase "file" part, nothing else.
func TestEdgeConfigurationsResource_UpdateFormFieldsRegression119(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeConfigurations(mock, 7)
	fp := writeEdgeConfigFile(t, "hello-edge-config")

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + edgeConfigurationHCL(fp, "general", "[1]")},
			{Config: mockProviderConfig(mock) + edgeConfigurationHCL(fp, "general", "[1, 2]")},
		},
	})

	fake.mu.Lock()
	defer fake.mu.Unlock()
	if len(fake.lastForm) != 2 || fake.lastForm["file"] != "hello-edge-config" {
		t.Errorf("expected only edgeConfiguration and file parts, got %v", fake.lastForm)
	}
	var payload map[string]interface{}
	if err := json.Unmarshal([]byte(fake.lastForm["edgeConfiguration"]), &payload); err != nil {
		t.Fatal(err)
	}
	if payload["type"] != "general" || payload["edgeGroupIDs"] == nil || len(payload) != 2 {
		t.Errorf("unexpected edgeConfiguration payload %v", payload)
	}
	for _, key := range []string{"EdgeGroupIDs", "Type"} {
		if _, ok := payload[key]; ok {
			t.Errorf("capitalized %s must not be sent", key)
		}
	}
}

// TestEdgeConfigurationsResource_NumericTypeKept: "1" and "general" are the
// same type, so the numeric form a configuration uses plans no change.
func TestEdgeConfigurationsResource_NumericTypeKept(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEdgeConfigurations(mock, 7)
	config := mockProviderConfig(mock) + edgeConfigurationHCL(writeEdgeConfigFile(t, "x"), "1", "[1]")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{Config: config, PlanOnly: true},
		},
	})
}

func TestEdgeConfigurationsResource_Errors(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		setup  func(*MockServer)
		config func(*testing.T) string
		want   string
	}{
		"create rejected": {
			setup: func(m *MockServer) {
				m.On("GET", "/edge_configurations", RespondJSON(http.StatusOK, []interface{}{}))
				m.On("POST", "/edge_configurations", RespondString(http.StatusBadRequest, "", "bad config"))
			},
			config: func(t *testing.T) string { return edgeConfigurationHCL(writeEdgeConfigFile(t, "x"), "general", "[1]") },
			want:   "failed to create edge configuration: bad config",
		},
		"missing file": {
			config: func(*testing.T) string { return edgeConfigurationHCL("/nonexistent/cfg.txt", "general", "[1]") },
			want:   "failed to hash file_path",
		},
		"unknown ID": {
			setup: func(m *MockServer) {
				m.On("GET", "/edge_configurations", RespondJSON(http.StatusOK, []interface{}{}))
				m.On("POST", "/edge_configurations", RespondString(http.StatusOK, "", ""))
			},
			config: func(t *testing.T) string { return edgeConfigurationHCL(writeEdgeConfigFile(t, "x"), "general", "[1]") },
			want:   "edge configuration created but could not determine its ID",
		},
		"invalid category": {
			config: func(*testing.T) string {
				return strings.Replace(edgeConfigurationHCL("x", "general", "[1]"), `"configuration"`, `"other"`, 1)
			},
			want: "value must be one of",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			if tc.setup != nil {
				tc.setup(mock)
			}
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{{
					Config:      mockProviderConfig(mock) + tc.config(t),
					ExpectError: edgeErrRe(tc.want),
				}},
			})
		})
	}
}

// TestEdgeConfigurationsResource_IDFromResponse: a Portainer that does return
// the new configuration needs no listing to find it.
func TestEdgeConfigurationsResource_IDFromResponse(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	created := map[string]interface{}{"id": 21, "name": "cfg", "type": 1, "category": "configuration", "edgeGroupIDs": []int{1}}
	mock.On("GET", "/edge_configurations", RespondJSON(http.StatusOK, []interface{}{}))
	mock.On("POST", "/edge_configurations", RespondJSON(http.StatusOK, created))
	mock.On("GET", "/edge_configurations/21", RespondJSON(http.StatusOK, created))
	mock.On("DELETE", "/edge_configurations/21", RespondString(http.StatusNoContent, "", ""))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + edgeConfigurationHCL(writeEdgeConfigFile(t, "x"), "general", "[1]"),
			Check:  resource.TestCheckResourceAttr("portainer_edge_configurations.c", "id", "21"),
		}},
	})
	listings := 0
	for _, r := range mock.Requests() {
		if r.Method == http.MethodGet && r.Path == "/edge_configurations" {
			listings++
		}
	}
	if listings != 1 {
		t.Errorf("expected only the pre-create listing, got %d", listings)
	}
}

func TestEdgeConfigurations_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEdgeConfigurations(mock, 7, 8)
	fp := writeEdgeConfigFile(t, "payload")
	config := mockProviderConfig(mock) + edgeConfigurationHCL(fp, "general", "[1, 2]") + `
resource "portainer_edge_configurations" "bare" {
  name           = "bare"
  type           = "1"
  edge_group_ids = []
  file_path      = "` + fp + `"
}`
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				ConfigPlanChecks:         resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()}},
			},
		},
	})
}

func TestAccEdgeConfigurationsResource(t *testing.T) {
	t.Skip("edge configurations are Business Edition only (PORTAINER_ACC_BE=1) and need an edge group to target; not covered by the acceptance setup")
}

func TestEdgeConfigTypeEquivalent(t *testing.T) {
	t.Parallel()
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
			result := edgeConfigTypeEquivalent(tt.old, tt.new)
			if result != tt.expected {
				t.Errorf("edgeConfigTypeEquivalent(%q, %q) = %v, want %v", tt.old, tt.new, result, tt.expected)
			}
		})
	}
}

func TestEdgeConfigTypeToStringMap(t *testing.T) {
	t.Parallel()
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

func TestSha256File(t *testing.T) {
	t.Parallel()
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

func TestResolveCreatedEdgeConfigID(t *testing.T) {
	t.Parallel()
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
