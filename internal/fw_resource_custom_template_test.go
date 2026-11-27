package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// fakeCustomTemplates is a stateful stand-in for the custom template API.
type fakeCustomTemplates struct {
	mu        sync.Mutex
	nextID    int64
	templates map[int64]map[string]interface{}
}

func newFakeCustomTemplates(m *MockServer) *fakeCustomTemplates {
	f := &fakeCustomTemplates{nextID: 90, templates: map[int64]map[string]interface{}{}}
	m.On("GET", "/custom_templates", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		out := make([]map[string]interface{}, 0, len(f.templates))
		for _, t := range f.templates {
			out = append(out, f.view(t))
		}
		RespondJSON(http.StatusOK, out)(w, r)
	})
	create := func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		f.nextID++
		body["Id"] = f.nextID
		f.templates[f.nextID] = body
		f.route(m, f.nextID)
		RespondJSON(http.StatusOK, f.view(body))(w, r)
	}
	m.On("POST", "/custom_templates/create/string", create)
	m.On("POST", "/custom_templates/create/repository", create)
	return f
}

// view is the template as GET returns it. Business Edition reports edge
// settings with a default stagger configuration for a template that sent none.
func (f *fakeCustomTemplates) view(t map[string]interface{}) map[string]interface{} {
	edge := t["edgeSettings"]
	if edge == nil {
		edge = map[string]interface{}{"StaggerConfig": map[string]interface{}{"StaggerOption": 1}}
	}
	return map[string]interface{}{
		"Id": t["Id"], "Title": t["title"], "Description": t["description"], "Note": t["note"],
		"Platform": t["platform"], "Type": t["type"], "Logo": t["logo"],
		"edgeTemplate": t["edgeTemplate"], "isComposeFormat": t["isComposeFormat"],
		"EdgeSettings": edge,
	}
}

// adopt adds a template created outside Terraform.
func (f *fakeCustomTemplates) adopt(m *MockServer, id int64, title string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.templates[id] = map[string]interface{}{"Id": id, "title": title, "description": "old", "note": "n", "platform": 1, "type": 2}
	f.route(m, id)
}

func (f *fakeCustomTemplates) route(m *MockServer, id int64) {
	p := "/custom_templates/" + strconv.FormatInt(id, 10)
	m.On("GET", p, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		t, ok := f.templates[id]
		if !ok {
			RespondString(http.StatusNotFound, "application/json", `{"message":"not found"}`)(w, r)
			return
		}
		RespondJSON(http.StatusOK, f.view(t))(w, r)
	})
	m.On("PUT", p, func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		body["Id"] = id
		f.templates[id] = body
		RespondJSON(http.StatusOK, f.view(body))(w, r)
	})
	m.On("PUT", p+"/git_fetch", RespondJSON(http.StatusOK, map[string]interface{}{"FileContent": ""}))
	m.On("DELETE", p, func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		delete(f.templates, id)
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
}

func customTemplateConfig(attrs string) string {
	return fmt.Sprintf(`
resource "portainer_custom_template" "t" {
  title       = "web"
  description = "Web stack"
  note        = "n"
  platform    = 1
  type        = 2
  %s
}`, attrs)
}

// TestCustomTemplateResource_FromString: the string payload with variables,
// a change of description updated in place, drift, import and delete.
func TestCustomTemplateResource_FromString(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeCustomTemplates(mock)
	body := `file_content = "services: {}"
  variables = [{ name = "PORT", label = "Port", default_value = "80", description = "port" }]`

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + customTemplateConfig(body),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_custom_template.t", "id", "91"),
					resource.TestCheckResourceAttr("portainer_custom_template.t", "edge_template", "false"),
					resource.TestCheckResourceAttr("portainer_custom_template.t", "repository_reference", "refs/heads/main"),
					resource.TestCheckNoResourceAttr("portainer_custom_template.t", "logo"),
					func(*terraform.State) error {
						p := gsbBody(t, mock, "POST", "/custom_templates/create/string")
						if p["title"] != "web" || p["fileContent"] != "services: {}" || p["type"] != float64(2) {
							return fmt.Errorf("payload mismatch: %v", p)
						}
						vars, _ := p["variables"].([]interface{})
						if len(vars) != 1 || vars[0].(map[string]interface{})["defaultValue"] != "80" {
							return fmt.Errorf("variables: %v", p["variables"])
						}
						return nil
					},
				),
			},
			{
				Config: mockProviderConfig(mock) + strings.Replace(customTemplateConfig(body), "Web stack", "Changed", 1),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_custom_template.t", "id", "91"),
					func(*terraform.State) error {
						p := gsbBody(t, mock, "PUT", "/custom_templates/91")
						if p["description"] != "Changed" || p["fileContent"] != "services: {}" {
							return fmt.Errorf("update payload mismatch: %v", p)
						}
						if gsbCount(mock, "PUT", "/custom_templates/91/git_fetch") != 0 {
							return fmt.Errorf("an inline template must not be fetched from Git")
						}
						return nil
					},
				),
			},
			{
				PreConfig: func() {
					fake.mu.Lock()
					fake.templates[91]["note"] = "changed elsewhere"
					fake.mu.Unlock()
				},
				Config:             mockProviderConfig(mock) + strings.Replace(customTemplateConfig(body), "Web stack", "Changed", 1),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
			{Config: mockProviderConfig(mock) + strings.Replace(customTemplateConfig(body), "Web stack", "Changed", 1)},
			{
				ResourceName:            "portainer_custom_template.t",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"file_content", "variables", "repository_reference", "compose_file_path", "tlsskip_verify", "repository_authentication"},
			},
		},
		CheckDestroy: func(*terraform.State) error {
			if gsbCount(mock, "DELETE", "/custom_templates/91") != 1 {
				return fmt.Errorf("expected DELETE /custom_templates/91")
			}
			return nil
		},
	})
}

// TestCustomTemplateResource_ExistingTitleIsAdopted: a template with the same
// title is updated rather than duplicated.
func TestCustomTemplateResource_ExistingTitleIsAdopted(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeCustomTemplates(mock)
	fake.adopt(mock, 7, "web")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + customTemplateConfig(`file_content = "x"`),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_custom_template.t", "id", "7"),
				func(*terraform.State) error {
					if gsbCount(mock, "POST", "/custom_templates/create/string") != 0 {
						return fmt.Errorf("an existing title must not be created again")
					}
					if p := gsbBody(t, mock, "PUT", "/custom_templates/7"); p["description"] != "Web stack" {
						return fmt.Errorf("update payload mismatch: %v", p)
					}
					return nil
				},
			),
		}},
	})
}

// TestCustomTemplateResource_FromFilePath: the file is read on create and on
// update; file_content stays as configured.
func TestCustomTemplateResource_FromFilePath(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeCustomTemplates(mock)
	p := filepath.Join(t.TempDir(), "compose.yml")
	if err := os.WriteFile(p, []byte("services:\n  web: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := func(desc string) string {
		return mockProviderConfig(mock) + strings.Replace(customTemplateConfig(fmt.Sprintf("file_path = %q", p)), "Web stack", desc, 1)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config("Web stack"),
				Check: func(*terraform.State) error {
					if c := gsbBody(t, mock, "POST", "/custom_templates/create/string")["fileContent"]; c != "services:\n  web: {}\n" {
						return fmt.Errorf("fileContent: got %v", c)
					}
					return nil
				},
			},
			{
				PreConfig: func() { _ = os.WriteFile(p, []byte("services: {}\n"), 0o600) },
				Config:    config("Changed"),
				Check: func(*terraform.State) error {
					if c := gsbBody(t, mock, "PUT", "/custom_templates/91")["fileContent"]; c != "services: {}\n" {
						return fmt.Errorf("fileContent: got %v", c)
					}
					return nil
				},
			},
		},
	})
}

// TestCustomTemplateResource_FromRepository: the repository payload with its
// credentials, and the Git fetch on update.
func TestCustomTemplateResource_FromRepository(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeCustomTemplates(mock)
	repo := `repository_url            = "https://git.example.com/stacks.git"
  repository_authentication = true
  repository_username       = "bot"
  repository_password       = "token"
  compose_file_path         = "web/compose.yml"`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + customTemplateConfig(repo),
				Check: func(*terraform.State) error {
					p := gsbBody(t, mock, "POST", "/custom_templates/create/repository")
					for k, want := range map[string]interface{}{
						"repositoryURL": "https://git.example.com/stacks.git", "repositoryAuthentication": true,
						"repositoryUsername": "bot", "repositoryPassword": "token",
						"composeFilePathInRepository": "web/compose.yml", "repositoryReferenceName": "refs/heads/main",
					} {
						if p[k] != want {
							return fmt.Errorf("payload.%s: got %v, want %v", k, p[k], want)
						}
					}
					return nil
				},
			},
			{
				Config: mockProviderConfig(mock) + strings.Replace(customTemplateConfig(repo), "Web stack", "Changed", 1),
				Check: func(*terraform.State) error {
					if gsbCount(mock, "PUT", "/custom_templates/91/git_fetch") != 1 {
						return fmt.Errorf("a Git-based template must be fetched again on update")
					}
					if p := gsbBody(t, mock, "PUT", "/custom_templates/91"); p["repositoryPassword"] != "token" {
						return fmt.Errorf("update payload mismatch: %v", p)
					}
					return nil
				},
			},
		},
	})
}

// TestCustomTemplateResource_WriteOnlyPassword: repository_password_wo is
// sent and stays out of state.
func TestCustomTemplateResource_WriteOnlyPassword(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeCustomTemplates(mock)
	config := mockProviderConfig(mock) + customTemplateConfig(`repository_url                 = "https://git.example.com/stacks.git"
  repository_authentication      = true
  repository_username            = "bot"
  repository_password_wo         = "wo-token"
  repository_password_wo_version = 1`)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("portainer_custom_template.t", "repository_password"),
					func(*terraform.State) error {
						if p := gsbBody(t, mock, "POST", "/custom_templates/create/repository")["repositoryPassword"]; p != "wo-token" {
							return fmt.Errorf("repositoryPassword: expected the write-only value, got %v", p)
						}
						return nil
					},
				),
			},
			{Config: config, PlanOnly: true},
		},
	})
}

// TestCustomTemplateResource_FromSource: a Source supplies the repository, so
// SourceID goes out on create and on every update, without repositoryURL.
func TestCustomTemplateResource_FromSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeCustomTemplates(mock)
	src := `source_id         = 4
  compose_file_path = "deploy/compose.yml"`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + customTemplateConfig(src),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_custom_template.t", "id", "91"),
					func(*terraform.State) error {
						p := gsbBody(t, mock, "POST", "/custom_templates/create/repository")
						if p["SourceID"] != float64(4) {
							return fmt.Errorf("payload.SourceID: expected 4, got %v", p["SourceID"])
						}
						if _, ok := p["repositoryURL"]; ok {
							return fmt.Errorf("payload.repositoryURL: expected it left out for a Source, got %v", p["repositoryURL"])
						}
						if p["composeFilePathInRepository"] != "deploy/compose.yml" {
							return fmt.Errorf("composeFilePathInRepository: got %v", p["composeFilePathInRepository"])
						}
						return nil
					},
				),
			},
			{
				Config: mockProviderConfig(mock) + strings.Replace(customTemplateConfig(src), "Web stack", "Changed", 1),
				Check: func(*terraform.State) error {
					if p := gsbBody(t, mock, "PUT", "/custom_templates/91"); p["SourceID"] != float64(4) {
						return fmt.Errorf("payload.SourceID: expected 4 on every update, got %v", p["SourceID"])
					}
					return nil
				},
			},
		},
	})
}

func TestCustomTemplateResource_Errors(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/custom_templates", RespondJSON(http.StatusOK, []interface{}{}))
	mock.On("POST", "/custom_templates/create/string", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))
	listErr := NewMockServer(t)
	listErr.On("GET", "/custom_templates", RespondString(http.StatusInternalServerError, "application/json", `{"message":"boom"}`))
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: mockProviderConfig(mock) + customTemplateConfig(`file_content = "x"`), ExpectError: regexp.MustCompile(`failed to create custom template from string`)},
			{Config: mockProviderConfig(mock) + customTemplateConfig(``), ExpectError: regexp.MustCompile(`one of file_content, file_path, repository_url, or source_id must be provided`)},
			{Config: mockProviderConfig(mock) + customTemplateConfig(`file_path = "/does/not/exist"`), ExpectError: regexp.MustCompile(`failed to read template file from path`)},
			{Config: mockProviderConfig(listErr) + customTemplateConfig(`file_content = "x"`), ExpectError: regexp.MustCompile(`failed to check for existing custom template`)},
			{Config: mockProviderConfig(mock) + strings.Replace(customTemplateConfig(`file_content = "x"`), "platform    = 1", "platform    = 3", 1), ExpectError: regexp.MustCompile(`between 1 and 2`)},
		},
	})
}

// TestCustomTemplateResource_EdgeSettings: the edge_settings block is sent on
// create and update, read back with unset attributes left null, and its drift
// shows; a template without the block sends none.
func TestCustomTemplateResource_EdgeSettings(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeCustomTemplates(mock)
	config := func(period int) string {
		return mockProviderConfig(mock) + customTemplateConfig(fmt.Sprintf(`file_content  = "services: {}"
  edge_template = true
  edge_settings {
    retry_deploy                  = true
    retry_period                  = %d
    relative_path                 = "/opt/edge"
    per_device_configs_path       = "configs"
    per_device_configs_match_type = "file"
    stagger_config {
      stagger_option  = "parallel"
      parallel_option = "fixed"
      device_number   = 3
    }
  }`, period)) + `
resource "portainer_custom_template" "plain" {
  title        = "plain"
  description  = "d"
  note         = "n"
  platform     = 1
  type         = 2
  file_content = "services: {}"
}`
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config(30),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_custom_template.t", "edge_settings.0.retry_period", "30"),
					resource.TestCheckNoResourceAttr("portainer_custom_template.t", "edge_settings.0.pre_pull_image"),
					resource.TestCheckResourceAttr("portainer_custom_template.t", "edge_settings.0.stagger_config.0.device_number", "3"),
					resource.TestCheckResourceAttr("portainer_custom_template.plain", "edge_settings.#", "0"),
					func(*terraform.State) error {
						for _, req := range mock.Requests() {
							if req.Method != http.MethodPost {
								continue
							}
							var p map[string]interface{}
							_ = json.Unmarshal(req.Body, &p)
							edge, ok := p["edgeSettings"].(map[string]interface{})
							if p["title"] == "plain" && ok {
								return fmt.Errorf("edgeSettings must be left out without the block: %v", p)
							}
							if p["title"] != "web" {
								continue
							}
							rel, _ := edge["RelativePathSettings"].(map[string]interface{})
							st, _ := edge["StaggerConfig"].(map[string]interface{})
							if edge["RetryDeploy"] != true || edge["RetryPeriod"] != float64(30) || rel["FilesystemPath"] != "/opt/edge" ||
								rel["SupportPerDeviceConfigs"] != true || rel["PerDeviceConfigsMatchType"] != "file" ||
								st["StaggerOption"] != float64(2) || st["StaggerParallelOption"] != float64(1) || st["DeviceNumber"] != float64(3) {
								return fmt.Errorf("create edgeSettings: %v", edge)
							}
						}
						return nil
					},
				),
			},
			{
				Config: config(60),
				Check: func(*terraform.State) error {
					var p map[string]interface{}
					for _, req := range mock.Requests() {
						if req.Method == http.MethodPut && strings.HasPrefix(req.Path, "/custom_templates/") {
							_ = json.Unmarshal(req.Body, &p)
						}
					}
					if edge, _ := p["edgeSettings"].(map[string]interface{}); edge["RetryPeriod"] != float64(60) {
						return fmt.Errorf("update edgeSettings: %v", p)
					}
					return nil
				},
			},
			{Config: config(60), PlanOnly: true},
			{
				PreConfig: func() {
					fake.mu.Lock()
					defer fake.mu.Unlock()
					for _, tmpl := range fake.templates {
						if edge, ok := tmpl["edgeSettings"].(map[string]interface{}); ok {
							edge["RetryPeriod"] = 5
						}
					}
				},
				Config:             config(60),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestCustomTemplateResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	newMock := func(t *testing.T) *MockServer {
		m := NewMockServer(t)
		newFakeCustomTemplates(m)
		return m
	}
	gsbUpgradeTest(t, newMock,
		customTemplateConfig(`file_content = "services: {}"`),
		customTemplateConfig(`file_content = "services: {}"
  logo          = "https://example.com/logo.png"
  edge_template = true
  variables     = [{ name = "PORT", label = "Port" }]`),
		customTemplateConfig(`repository_url            = "https://git.example.com/stacks.git"
  repository_authentication = true
  repository_username       = "bot"
  repository_password       = "token"
  tlsskip_verify            = true`),
		customTemplateConfig(`source_id = 4`),
	)
}
