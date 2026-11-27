package internal

import (
	"encoding/json"
	"fmt"
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

// fakeEdgeStacks is a stateful stand-in for Portainer's edge stack API: the
// four create methods, the three update endpoints, read and delete. It stores
// what Portainer would and answers GET /edge_stacks/{id} the way Portainer does.
type fakeEdgeStacks struct {
	mu     sync.Mutex
	nextID int
	stacks map[int]map[string]interface{}
	// bodies keeps every JSON body (or multipart fields) by "METHOD path".
	bodies map[string][]map[string]interface{}
}

func newFakeEdgeStacks(m *MockServer, ids ...int) *fakeEdgeStacks {
	f := &fakeEdgeStacks{stacks: map[int]map[string]interface{}{}, bodies: map[string][]map[string]interface{}{}}
	m.On("GET", "/edge_stacks", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		list := make([]map[string]interface{}, 0, len(f.stacks))
		for id, s := range f.stacks {
			list = append(list, map[string]interface{}{"Id": id, "Name": s["Name"], "DeploymentType": s["DeploymentType"]})
		}
		RespondJSON(http.StatusOK, list)(w, r)
	})
	for _, method := range []string{"string", "repository", "helmRepo", "file"} {
		method := method
		m.On("POST", "/edge_stacks/create/"+method, func(w http.ResponseWriter, r *http.Request) {
			body := f.decode(r)
			f.mu.Lock()
			defer f.mu.Unlock()
			f.bodies["POST "+method] = append(f.bodies["POST "+method], body)
			if r.URL.Query().Get("dryrun") == "true" {
				RespondJSON(http.StatusOK, map[string]interface{}{"Id": 0})(w, r)
				return
			}
			f.nextID++
			f.stacks[f.nextID] = f.fromCreate(method, body)
			RespondJSON(http.StatusOK, map[string]interface{}{"Id": f.nextID})(w, r)
		})
	}
	for _, id := range ids {
		id := id
		p := "/edge_stacks/" + strconv.Itoa(id)
		m.On("GET", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			s, ok := f.stacks[id]
			if !ok {
				RespondString(http.StatusNotFound, "", "not found")(w, r)
				return
			}
			RespondJSON(http.StatusOK, s)(w, r)
		})
		for _, suffix := range []string{"", "/git", "/helmRepo"} {
			suffix := suffix
			m.On("PUT", p+suffix, func(w http.ResponseWriter, r *http.Request) {
				body := f.decode(r)
				f.mu.Lock()
				defer f.mu.Unlock()
				f.bodies["PUT "+suffix] = append(f.bodies["PUT "+suffix], body)
				f.applyUpdate(f.stacks[id], suffix, body)
				RespondJSON(http.StatusOK, map[string]interface{}{})(w, r)
			})
		}
		m.On("DELETE", p, func(w http.ResponseWriter, _ *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			delete(f.stacks, id)
			w.WriteHeader(http.StatusNoContent)
		})
	}
	return f
}

// last returns the last body sent to "METHOD suffix", e.g. "POST string" or
// "PUT /git".
func (f *fakeEdgeStacks) last(key string) map[string]interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	if b := f.bodies[key]; len(b) > 0 {
		return b[len(b)-1]
	}
	return nil
}

func (f *fakeEdgeStacks) decode(r *http.Request) map[string]interface{} {
	body := map[string]interface{}{}
	mediaType, params, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if strings.HasPrefix(mediaType, "multipart/") {
		mr := multipart.NewReader(r.Body, params["boundary"])
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			data, _ := io.ReadAll(part)
			body[part.FormName()] = string(data)
		}
		return body
	}
	_ = json.NewDecoder(r.Body).Decode(&body)
	return body
}

func edgeStackKubernetesOptions(opts interface{}) map[string]interface{} {
	o, ok := opts.(map[string]interface{})
	if !ok {
		return nil
	}
	k := map[string]interface{}{}
	if v, ok := o["KubernetesPrune"]; ok {
		k["Prune"] = v
	}
	if v, ok := o["KubernetesForceApply"]; ok {
		k["ForceApply"] = v
	}
	return k
}

// edgeStackStagger is the stagger configuration of a create or update body,
// under the key the endpoint uses (multipart sends it JSON-encoded).
func edgeStackStagger(body map[string]interface{}) interface{} {
	for _, key := range []string{"staggerConfig", "StaggerConfig"} {
		switch v := body[key].(type) {
		case map[string]interface{}:
			return v
		case string:
			var o map[string]interface{}
			_ = json.Unmarshal([]byte(v), &o)
			return o
		}
	}
	return nil
}

func (f *fakeEdgeStacks) fromCreate(method string, body map[string]interface{}) map[string]interface{} {
	// Business Edition records a default stagger configuration for every stack.
	s := map[string]interface{}{"EdgeGroups": []int{}, "Registries": []int{}, "EnvVars": []interface{}{}, "StaggerConfig": map[string]interface{}{"StaggerOption": 1}}
	if st := edgeStackStagger(body); st != nil {
		s["StaggerConfig"] = st
	}
	switch method {
	case "helmRepo":
		s["Name"], s["EdgeGroups"], s["DeploymentType"] = body["Name"], body["EdgeGroups"], 1
		s["HelmConfig"] = body["HelmConfig"]
		return s
	case "file":
		var groups, registries interface{}
		_ = json.Unmarshal([]byte(body["EdgeGroups"].(string)), &groups)
		_ = json.Unmarshal([]byte(body["Registries"].(string)), &registries)
		dt, _ := strconv.Atoi(body["DeploymentType"].(string))
		s["Name"], s["EdgeGroups"], s["Registries"], s["DeploymentType"] = body["Name"], groups, registries, dt
		s["UseManifestNamespaces"] = body["UseManifestNamespaces"] == "true"
		s["PrePullImage"] = body["PrePullImage"] == "true"
		s["RetryDeploy"] = body["RetryDeploy"] == "true"
		if opts, ok := body["DeployerOptions"].(string); ok {
			var o interface{}
			_ = json.Unmarshal([]byte(opts), &o)
			s["KubernetesOptions"] = edgeStackKubernetesOptions(o)
		}
		return s
	}
	s["Name"], s["EdgeGroups"], s["DeploymentType"] = body["name"], body["edgeGroups"], body["deploymentType"]
	s["UseManifestNamespaces"], s["Registries"] = body["useManifestNamespaces"], body["registries"]
	if v, ok := body["envVars"]; ok {
		s["EnvVars"] = v
	}
	if k := edgeStackKubernetesOptions(body["deployerOptions"]); k != nil {
		s["KubernetesOptions"] = k
	}
	if method == "repository" {
		git := map[string]interface{}{
			"URL": body["repositoryURL"], "ReferenceName": body["repositoryReferenceName"], "ConfigFilePath": body["filePathInRepository"],
		}
		if body["repositoryAuthentication"] == true {
			git["Authentication"] = map[string]interface{}{"Username": body["repositoryUsername"], "GitCredentialID": body["repositoryGitCredentialID"]}
		}
		s["GitConfig"] = git
		if body["supportRelativePath"] == true {
			s["SupportRelativePath"], s["FilesystemPath"] = true, body["filesystemPath"]
			s["AlwaysCloneGitRepoForRelativePath"] = body["AlwaysCloneGitRepoForRelativePath"]
		}
		if body["supportPerDeviceConfigs"] == true {
			s["SupportPerDeviceConfigs"], s["PerDeviceConfigsPath"] = true, body["perDeviceConfigsPath"]
			s["PerDeviceConfigsMatchType"], s["PerDeviceConfigsGroupMatchType"] = body["perDeviceConfigsMatchType"], body["perDeviceConfigsGroupMatchType"]
		}
		f.applyAutoUpdate(s, body)
	}
	return s
}

func (f *fakeEdgeStacks) applyAutoUpdate(s, body map[string]interface{}) {
	au, ok := body["autoUpdate"].(map[string]interface{})
	if !ok {
		return
	}
	s["AutoUpdate"] = map[string]interface{}{
		"Interval": au["interval"], "Webhook": au["webhook"], "ForcePullImage": au["forcePullImage"], "ForceUpdate": au["forceUpdate"],
	}
}

func (f *fakeEdgeStacks) applyUpdate(s map[string]interface{}, suffix string, body map[string]interface{}) {
	if st := edgeStackStagger(body); st != nil {
		s["StaggerConfig"] = st
	}
	switch suffix {
	case "/helmRepo":
		s["EdgeGroups"], s["HelmConfig"] = body["EdgeGroups"], body["HelmConfig"]
	case "/git":
		s["EdgeGroups"], s["Registries"], s["EnvVars"] = body["groupIds"], body["registries"], body["envVars"]
		s["PrePullImage"], s["RePullImage"], s["RetryDeploy"] = body["prePullImage"], body["rePullImage"], body["retryDeploy"]
		if git, ok := s["GitConfig"].(map[string]interface{}); ok {
			git["ReferenceName"] = body["refName"]
		}
		// Only RelativePath is read by the git update; the top-level copy is not.
		if rel, ok := body["relativePath"].(map[string]interface{}); ok {
			s["AlwaysCloneGitRepoForRelativePath"] = rel["AlwaysCloneGitRepoForRelativePath"]
		}
		f.applyAutoUpdate(s, body)
		if k := edgeStackKubernetesOptions(body["deployerOptions"]); k != nil {
			s["KubernetesOptions"] = k
		}
	default:
		s["Name"], s["EdgeGroups"], s["Registries"], s["EnvVars"] = body["name"], body["edgeGroups"], body["registries"], body["envVars"]
		s["UseManifestNamespaces"], s["PrePullImage"], s["RePullImage"] = body["useManifestNamespaces"], body["prePullImage"], body["rePullImage"]
		if k := edgeStackKubernetesOptions(body["deployerOptions"]); k != nil {
			s["KubernetesOptions"] = k
		}
	}
}

func edgeStackStep(config string, checks ...resource.TestCheckFunc) resource.TestStep {
	return resource.TestStep{Config: config, Check: resource.ComposeTestCheckFunc(checks...)}
}

// TestEdgeStackResource_StringLifecycle: a Kubernetes stack from inline
// content, its update and import.
func TestEdgeStackResource_StringLifecycle(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeStacks(mock, 1)
	hcl := func(content string) string {
		return mockProviderConfig(mock) + `resource "portainer_edge_stack" "s" {
  name               = "web"
  deployment_type    = 1
  edge_groups        = [1, 2]
  registries         = [3]
  stack_file_content = "` + content + `"
  kubernetes_prune   = false
  environment        = { FOO = "bar" }
}`
	}

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			edgeStackStep(hcl("v1"),
				resource.TestCheckResourceAttr("portainer_edge_stack.s", "id", "1"),
				resource.TestCheckResourceAttr("portainer_edge_stack.s", "kubernetes_prune", "false"),
				resource.TestCheckNoResourceAttr("portainer_edge_stack.s", "kubernetes_force_apply"),
				resource.TestCheckResourceAttr("portainer_edge_stack.s", "webhook_id", ""),
				resource.TestCheckResourceAttr("portainer_edge_stack.s", "environment.FOO", "bar"),
			),
			{
				Config: hcl("v2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_edge_stack.s", plancheck.ResourceActionUpdate)},
				},
			},
			// Like the SDKv2 importer, an import reads back only what the API
			// reports; the git defaults of a non-git stack stay unset.
			{
				ResourceName:            "portainer_edge_stack.s",
				ImportState:             true,
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"stack_file_content", "timeouts", "file_path_in_repository", "git_repository_authentication", "relative_path", "repository_reference_name", "update_interval"},
			},
		},
	})

	create := fake.last("POST string")
	if create["name"] != "web" || create["stackFileContent"] != "v1" || create["deploymentType"] != float64(1) {
		t.Errorf("unexpected create payload %v", create)
	}
	if opts, _ := create["deployerOptions"].(map[string]interface{}); opts["KubernetesPrune"] != false || len(opts) != 1 {
		t.Errorf("deployerOptions: expected only KubernetesPrune=false, got %v", create["deployerOptions"])
	}
	if env, _ := create["envVars"].([]interface{}); len(env) != 1 {
		t.Errorf("envVars: got %v", create["envVars"])
	}
	update := fake.last("PUT ")
	if update["stackFileContent"] != "v2" || update["updateVersion"] != true || update["rePullImage"] != false {
		t.Errorf("unexpected update payload %v", update)
	}
	if opts, _ := update["deployerOptions"].(map[string]interface{}); opts["KubernetesPrune"] != false {
		t.Errorf("update deployerOptions: got %v", update["deployerOptions"])
	}
}

// TestEdgeStackResource_KubernetesOptions covers the apply policies: left out
// they are not sent, on a Compose stack they are rejected at plan time.
func TestEdgeStackResource_KubernetesOptions(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeStacks(mock, 1)
	fake.mu.Lock()
	fake.nextID = 0
	fake.mu.Unlock()

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `resource "portainer_edge_stack" "s" {
  name               = "compose"
  deployment_type    = 0
  edge_groups        = [1]
  stack_file_content = "services: {}"
  kubernetes_force_apply = true
}`,
				ExpectError: edgeErrRe("kubernetes_force_apply only applies to deployment_type = 1 (Kubernetes)"),
			},
			edgeStackStep(mockProviderConfig(mock)+`resource "portainer_edge_stack" "s" {
  name               = "k8s"
  deployment_type    = 1
  edge_groups        = [1]
  stack_file_content = "kind: List"
}`, resource.TestCheckNoResourceAttr("portainer_edge_stack.s", "kubernetes_prune")),
		},
	})
	if _, ok := fake.last("POST string")["deployerOptions"]; ok {
		t.Error("deployerOptions must be left out when no policy is configured")
	}
}

// TestEdgeStackResource_FileMultipart covers the multipart create and the
// update of a file-based stack, which sends no content.
func TestEdgeStackResource_FileMultipart(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeStacks(mock, 1)
	fp := filepath.Join(t.TempDir(), "docker-compose.yml")
	if err := os.WriteFile(fp, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	hcl := func(pull bool) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`resource "portainer_edge_stack" "s" {
  name            = "file"
  deployment_type = 1
  edge_groups     = [1]
  stack_file_path = %q
  pre_pull_image  = true
  pull_image      = %t
  kubernetes_force_apply = true
}`, fp, pull)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			edgeStackStep(hcl(false), resource.TestCheckResourceAttr("portainer_edge_stack.s", "kubernetes_force_apply", "true")),
			edgeStackStep(hcl(true), resource.TestCheckResourceAttr("portainer_edge_stack.s", "pull_image", "true")),
		},
	})

	form := fake.last("POST file")
	if form["Name"] != "file" || form["EdgeGroups"] != "[1]" || form["Registries"] != "[]" || form["PrePullImage"] != "true" || form["file"] != "services: {}\n" {
		t.Errorf("unexpected multipart fields %v", form)
	}
	if form["DeployerOptions"] != `{"KubernetesForceApply":true}` {
		t.Errorf("DeployerOptions: got %v", form["DeployerOptions"])
	}
	update := fake.last("PUT ")
	if _, ok := update["stackFileContent"]; ok || update["rePullImage"] != true {
		t.Errorf("a file-based update sends no content: %v", update)
	}
}

func TestEdgeStackResource_FileDryRun(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEdgeStacks(mock)
	fp := filepath.Join(t.TempDir(), "docker-compose.yml")
	if err := os.WriteFile(fp, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: mockProviderConfig(mock) + fmt.Sprintf(`resource "portainer_edge_stack" "s" {
  name            = "dry"
  deployment_type = 0
  edge_groups     = [1]
  stack_file_path = %q
  dryrun          = true
}`, fp),
			ExpectError: edgeErrRe("only simulates the deployment"),
		}},
	})
	if req := mock.FindRequest("POST", "/edge_stacks/create/file"); req == nil || req.Query != "dryrun=true" {
		t.Errorf("expected POST /edge_stacks/create/file?dryrun=true, got %+v", req)
	}
}

// TestEdgeStackResource_RepositoryWebhook: a git stack with a GitOps webhook,
// a relative path and credentials; an update keeps the webhook.
func TestEdgeStackResource_RepositoryWebhook(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeStacks(mock, 1)
	hcl := func(ref string) string {
		return mockProviderConfig(mock) + `resource "portainer_edge_stack" "s" {
  name                          = "git"
  deployment_type               = 0
  edge_groups                   = [1]
  repository_url                = "https://github.com/acme/edge.git"
  repository_reference_name     = "` + ref + `"
  file_path_in_repository       = "edge/compose.yml"
  git_repository_authentication = true
  repository_username           = "bot"
  repository_password           = "token"
  stack_webhook                 = true
  update_interval               = "5m"
  relative_path                 = "/opt/edge"
  always_clone                  = true
}`
	}
	var webhookID string
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			edgeStackStep(hcl("refs/heads/main"),
				resource.TestCheckResourceAttrWith("portainer_edge_stack.s", "webhook_id", func(v string) error {
					if len(v) != 36 {
						return fmt.Errorf("expected a UUID, got %q", v)
					}
					webhookID = v
					return nil
				}),
				resource.TestCheckResourceAttrWith("portainer_edge_stack.s", "webhook_url", func(v string) error {
					if !strings.HasSuffix(v, "/api/edge_stacks/webhooks/"+webhookID) {
						return fmt.Errorf("webhook_url %q does not end with the webhook", v)
					}
					return nil
				}),
			),
			{
				Config: hcl("refs/heads/release"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_edge_stack.s", plancheck.ResourceActionUpdate)},
				},
				Check: resource.TestCheckResourceAttrWith("portainer_edge_stack.s", "webhook_id", func(v string) error {
					if v != webhookID {
						return fmt.Errorf("the webhook changed on update: %q -> %q", webhookID, v)
					}
					return nil
				}),
			},
			{Config: hcl("refs/heads/release"), PlanOnly: true},
		},
	})

	create := fake.last("POST repository")
	if create["repositoryPassword"] != "token" || create["supportRelativePath"] != true || create["filesystemPath"] != "/opt/edge" || create["AlwaysCloneGitRepoForRelativePath"] != true {
		t.Errorf("unexpected create payload %v", create)
	}
	git := fake.last("PUT /git")
	if git["refName"] != "refs/heads/release" || git["updateVersion"] != true {
		t.Errorf("unexpected git update payload %v", git)
	}
	auth, _ := git["authentication"].(map[string]interface{})
	if auth["username"] != "bot" || auth["password"] != "token" {
		t.Errorf("authentication: got %v", git["authentication"])
	}
	au, _ := git["autoUpdate"].(map[string]interface{})
	if au["webhook"] != webhookID || au["interval"] != "5m" {
		t.Errorf("autoUpdate must keep the webhook: %v", git["autoUpdate"])
	}
}

// TestEdgeStackResource_IntervalWithoutWebhook: GitOps polling without a
// webhook leaves the webhook outputs empty.
func TestEdgeStackResource_IntervalWithoutWebhook(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeStacks(mock, 1)
	hcl := func(webhook bool) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`resource "portainer_edge_stack" "s" {
  name            = "poll"
  deployment_type = 0
  edge_groups     = [1]
  repository_url  = "https://github.com/acme/edge.git"
  update_interval = "10m"
  stack_webhook   = %t
}`, webhook)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			edgeStackStep(hcl(false),
				resource.TestCheckResourceAttr("portainer_edge_stack.s", "stack_webhook", "false"),
				resource.TestCheckResourceAttr("portainer_edge_stack.s", "webhook_id", ""),
				resource.TestCheckResourceAttr("portainer_edge_stack.s", "update_interval", "10m"),
			),
			// Turning the webhook on later makes one.
			edgeStackStep(hcl(true), resource.TestCheckResourceAttrWith("portainer_edge_stack.s", "webhook_id", func(v string) error {
				if len(v) != 36 {
					return fmt.Errorf("expected a UUID, got %q", v)
				}
				return nil
			})),
		},
	})
	au, _ := fake.bodies["POST repository"][0]["autoUpdate"].(map[string]interface{})
	if au["webhook"] != "" || au["interval"] != "10m" {
		t.Errorf("autoUpdate: got %v", au)
	}
}

func TestEdgeStackResource_FromSource(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeStacks(mock, 1)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{edgeStackStep(mockProviderConfig(mock) + `resource "portainer_edge_stack" "s" {
  name                    = "src"
  deployment_type         = 0
  edge_groups             = [4]
  source_id               = 6
  file_path_in_repository = "edge/compose.yml"
}`)},
	})
	if got := fake.last("POST repository")["sourceID"]; got != float64(6) {
		t.Errorf("payload.sourceID: expected 6, got %v", got)
	}
}

// TestEdgeStackResource_WriteOnlyRepositoryPassword: the write-only twin is
// sent and never stored.
func TestEdgeStackResource_WriteOnlyRepositoryPassword(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeStacks(mock, 1)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{edgeStackStep(mockProviderConfig(mock)+`resource "portainer_edge_stack" "s" {
  name                           = "e"
  deployment_type                = 0
  edge_groups                    = [2]
  repository_url                 = "https://git.example.com/edge.git"
  git_repository_authentication  = true
  repository_username            = "bot"
  repository_password_wo         = "git-token-2"
  repository_password_wo_version = 1
}`, resource.TestCheckNoResourceAttr("portainer_edge_stack.s", "repository_password"),
			resource.TestCheckNoResourceAttr("portainer_edge_stack.s", "repository_password_wo"))},
	})
	if got := fake.last("POST repository")["repositoryPassword"]; got != "git-token-2" {
		t.Errorf("repositoryPassword: expected the write-only value, got %v", got)
	}
}

// TestEdgeStackResource_Helm: the Helm repository method creates and updates
// through its own endpoints, leaves out the optional fields left unset, and a
// git stack never grows a helm_config block.
func TestEdgeStackResource_Helm(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeStacks(mock, 1, 2)
	hcl := func(version string) string {
		return mockProviderConfig(mock) + `resource "portainer_edge_stack" "s" {
  name            = "chart"
  deployment_type = 1
  edge_groups     = [1]
  helm_config {
    chart_url  = "https://charts.example.com"
    chart_name = "nginx"
    ` + version + `
  }
}
resource "portainer_edge_stack" "git" {
  name            = "git"
  deployment_type = 1
  edge_groups     = [1]
  repository_url  = "https://github.com/acme/edge.git"
}`
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			edgeStackStep(hcl(""),
				resource.TestCheckResourceAttr("portainer_edge_stack.s", "helm_config.0.chart_name", "nginx"),
				resource.TestCheckResourceAttr("portainer_edge_stack.s", "helm_config.0.chart_version", ""),
				resource.TestCheckResourceAttr("portainer_edge_stack.git", "helm_config.#", "0"),
			),
			{
				PreConfig: func() {
					// Portainer fills HelmConfig without a ChartURL for a git stack
					// deploying a chart from the repository.
					fake.mu.Lock()
					for _, s := range fake.stacks {
						if s["Name"] == "git" {
							s["HelmConfig"] = map[string]interface{}{"ChartName": "from-repo"}
						}
					}
					fake.mu.Unlock()
				},
				Config: hcl(`chart_version = "1.2.3"
    atomic        = true`),
				Check: resource.TestCheckResourceAttr("portainer_edge_stack.s", "helm_config.0.chart_version", "1.2.3"),
			},
			{Config: hcl(`chart_version = "1.2.3"
    atomic        = true`), PlanOnly: true},
		},
	})

	var create map[string]interface{}
	fake.mu.Lock()
	create = fake.bodies["POST helmRepo"][0]
	fake.mu.Unlock()
	helm, _ := create["HelmConfig"].(map[string]interface{})
	if helm["ChartURL"] != "https://charts.example.com" || len(helm) != 2 {
		t.Errorf("create HelmConfig must carry only the set fields, got %v", helm)
	}
	update, _ := fake.last("PUT /helmRepo")["HelmConfig"].(map[string]interface{})
	if update["ChartVersion"] != "1.2.3" || update["Atomic"] != true {
		t.Errorf("update HelmConfig: got %v", update)
	}
}

// TestEdgeStackResource_StaggerAndRepull: stagger_config goes to every create
// and update endpoint when set and is read back; pull_image also sends
// RepullImageAndRedeploy; a stack without the block sends none and ignores
// the default Portainer records.
func TestEdgeStackResource_StaggerAndRepull(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeStacks(mock, 1, 2, 3, 4)
	fp := filepath.Join(t.TempDir(), "docker-compose.yml")
	if err := os.WriteFile(fp, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// pull_image only on update: Portainer's create endpoints have no field for it.
	hcl := func(content, delay string) string {
		pull := "false"
		if content != "v1" {
			pull = "true"
		}
		stagger := `
  stagger_config {
    stagger_option             = "parallel"
    parallel_option            = "incremental"
    device_number_start_from   = 2
    device_number_increment_by = 2
    update_delay               = "` + delay + `"
    update_failure_action      = "rollback"
  }`
		return mockProviderConfig(mock) + `resource "portainer_edge_stack" "s" {
  name               = "staggered"
  deployment_type    = 0
  edge_groups        = [1]
  stack_file_content = "` + content + `"
  pull_image         = ` + pull + stagger + `
}
resource "portainer_edge_stack" "plain" {
  name               = "plain"
  deployment_type    = 0
  edge_groups        = [1]
  stack_file_content = "services: {}"
}
resource "portainer_edge_stack" "helm" {
  name            = "helm"
  deployment_type = 1
  edge_groups     = [1]
  helm_config {
    chart_url  = "https://charts.example.com"
    chart_name = "nginx"
  }` + stagger + `
}
resource "portainer_edge_stack" "file" {
  name            = "file"
  deployment_type = 0
  edge_groups     = [1]
  stack_file_path = "` + fp + `"` + stagger + `
}`
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			edgeStackStep(hcl("v1", "5"),
				resource.TestCheckResourceAttr("portainer_edge_stack.s", "stagger_config.0.stagger_option", "parallel"),
				resource.TestCheckResourceAttr("portainer_edge_stack.s", "stagger_config.0.update_failure_action", "rollback"),
				resource.TestCheckNoResourceAttr("portainer_edge_stack.s", "stagger_config.0.device_number"),
				resource.TestCheckNoResourceAttr("portainer_edge_stack.s", "stagger_config.0.timeout"),
				resource.TestCheckResourceAttr("portainer_edge_stack.plain", "stagger_config.#", "0"),
				resource.TestCheckResourceAttr("portainer_edge_stack.file", "stagger_config.0.update_delay", "5"),
			),
			{
				Config: hcl("v2", "10"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_edge_stack.s", plancheck.ResourceActionUpdate)},
				},
				Check: resource.TestCheckResourceAttr("portainer_edge_stack.s", "stagger_config.0.update_delay", "10"),
			},
			{Config: hcl("v2", "10"), PlanOnly: true},
		},
	})

	want := map[string]interface{}{
		"StaggerOption": float64(2), "StaggerParallelOption": float64(2), "DeviceNumberStartFrom": float64(2),
		"DeviceNumberIncrementBy": float64(2), "UpdateDelay": "5", "UpdateFailureAction": float64(3),
	}
	fake.mu.Lock()
	creates := append([]map[string]interface{}{}, fake.bodies["POST string"]...)
	helmCreate, fileCreate := fake.bodies["POST helmRepo"][0], fake.bodies["POST file"][0]
	fake.mu.Unlock()
	for _, c := range creates {
		st, ok := c["staggerConfig"]
		if c["name"] == "plain" && ok {
			t.Errorf("staggerConfig must be left out without the block, got %v", st)
		}
		if c["name"] == "staggered" && fmt.Sprint(st) != fmt.Sprint(want) {
			t.Errorf("create staggerConfig: got %v, want %v", st, want)
		}
	}
	if fmt.Sprint(helmCreate["StaggerConfig"]) != fmt.Sprint(want) {
		t.Errorf("helm create StaggerConfig: got %v", helmCreate["StaggerConfig"])
	}
	if fmt.Sprint(edgeStackStagger(fileCreate)) != fmt.Sprint(want) {
		t.Errorf("file create StaggerConfig: got %v", fileCreate["StaggerConfig"])
	}
	fake.mu.Lock()
	var putS, putPlain map[string]interface{}
	for _, b := range fake.bodies["PUT "] {
		switch b["name"] {
		case "staggered":
			putS = b
		case "plain":
			putPlain = b
		}
	}
	fake.mu.Unlock()
	if st, _ := putS["staggerConfig"].(map[string]interface{}); st["UpdateDelay"] != "10" || putS["repullImageAndRedeploy"] != true || putS["rePullImage"] != true {
		t.Errorf("update payload: got %v", putS)
	}
	if putPlain != nil {
		if _, ok := putPlain["repullImageAndRedeploy"]; ok {
			t.Errorf("repullImageAndRedeploy must be left out when pull_image is off: %v", putPlain)
		}
	}
	if st, _ := fake.last("PUT /helmRepo")["StaggerConfig"].(map[string]interface{}); st["UpdateDelay"] != "10" {
		t.Errorf("helm update StaggerConfig: got %v", fake.last("PUT /helmRepo"))
	}
}

// TestEdgeStackResource_GitRelativePathAndPerDevice: a git stack sends the
// per-device configurations on create, and always_clone, stagger_config and
// RepullImageAndRedeploy through the git update's own fields.
func TestEdgeStackResource_GitRelativePathAndPerDevice(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeStacks(mock, 1)
	hcl := func(alwaysClone bool) string {
		return mockProviderConfig(mock) + fmt.Sprintf(`resource "portainer_edge_stack" "s" {
  name                          = "git"
  deployment_type               = 0
  edge_groups                   = [1]
  repository_url                = "https://github.com/acme/edge.git"
  relative_path                 = "/opt/edge"
  always_clone                  = %t
  pull_image                    = %t
  per_device_configs_path       = "configs"
  per_device_configs_match_type = "dir"
  stagger_config {
    stagger_option = "all_at_once"
  }
}`, alwaysClone, alwaysClone)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: mockProviderConfig(mock) + `resource "portainer_edge_stack" "s" {
  name                          = "git"
  deployment_type               = 0
  edge_groups                   = [1]
  repository_url                = "https://github.com/acme/edge.git"
  per_device_configs_match_type = "dir"
}`,
				ExpectError: edgeErrRe(`per_device_configs_path`),
			},
			edgeStackStep(hcl(false),
				resource.TestCheckResourceAttr("portainer_edge_stack.s", "per_device_configs_path", "configs"),
				resource.TestCheckResourceAttr("portainer_edge_stack.s", "per_device_configs_match_type", "dir"),
				resource.TestCheckNoResourceAttr("portainer_edge_stack.s", "per_device_configs_group_match_type"),
			),
			{
				Config: hcl(true),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_edge_stack.s", plancheck.ResourceActionUpdate)},
				},
				Check: resource.TestCheckResourceAttr("portainer_edge_stack.s", "always_clone", "true"),
			},
			{Config: hcl(true), PlanOnly: true},
		},
	})

	create := fake.last("POST repository")
	if create["supportPerDeviceConfigs"] != true || create["perDeviceConfigsPath"] != "configs" || create["perDeviceConfigsMatchType"] != "dir" {
		t.Errorf("create per-device payload: got %v", create)
	}
	if _, ok := create["perDeviceConfigsGroupMatchType"]; ok {
		t.Errorf("an unset group match type must be left out: %v", create)
	}
	git := fake.last("PUT /git")
	rel, _ := git["relativePath"].(map[string]interface{})
	st, _ := git["staggerConfig"].(map[string]interface{})
	if rel["AlwaysCloneGitRepoForRelativePath"] != true || st["StaggerOption"] != float64(1) || git["repullImageAndRedeploy"] != true {
		t.Errorf("git update payload: got %v", git)
	}
}

// TestEdgeStackResource_ExistingNameIsAdopted: a stack of the same name is
// updated in place rather than created twice.
func TestEdgeStackResource_ExistingNameIsAdopted(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeStacks(mock, 9)
	fake.stacks[9] = map[string]interface{}{"Name": "web", "DeploymentType": 0, "EdgeGroups": []int{1}}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{edgeStackStep(mockProviderConfig(mock)+`resource "portainer_edge_stack" "s" {
  name               = "web"
  deployment_type    = 0
  edge_groups        = [1]
  stack_file_content = "services: {}"
}`, resource.TestCheckResourceAttr("portainer_edge_stack.s", "id", "9"))},
	})
	if fake.last("POST string") != nil {
		t.Error("an existing stack must not be created again")
	}
	if fake.last("PUT ") == nil {
		t.Error("expected the existing stack to be updated")
	}
}

func TestEdgeStackResource_Errors(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		setup  func(*MockServer)
		config string
		want   string
	}{
		"no source": {
			setup: func(m *MockServer) { newFakeEdgeStacks(m) },
			config: `resource "portainer_edge_stack" "s" {
  name            = "x"
  deployment_type = 0
  edge_groups     = [1]
}`,
			want: "one of 'stack_file_content', 'stack_file_path', 'repository_url', 'source_id', or 'helm_config' must be provided",
		},
		"list error": {
			setup: func(m *MockServer) {
				m.On("GET", "/edge_stacks", RespondString(http.StatusInternalServerError, "", "boom"))
			},
			config: `resource "portainer_edge_stack" "s" {
  name               = "x"
  deployment_type    = 0
  edge_groups        = [1]
  stack_file_content = "a"
}`,
			want: "failed to check for existing edge stack",
		},
		"create error": {
			setup: func(m *MockServer) {
				m.On("GET", "/edge_stacks", RespondJSON(http.StatusOK, []interface{}{}))
				m.On("POST", "/edge_stacks/create/string", RespondString(http.StatusBadRequest, "", "invalid compose"))
			},
			config: `resource "portainer_edge_stack" "s" {
  name               = "x"
  deployment_type    = 0
  edge_groups        = [1]
  stack_file_content = "a"
}`,
			want: "failed to create edge stack",
		},
		"file open error": {
			setup: func(m *MockServer) { newFakeEdgeStacks(m) },
			config: `resource "portainer_edge_stack" "s" {
  name            = "x"
  deployment_type = 0
  edge_groups     = [1]
  stack_file_path = "/nonexistent/compose.yml"
}`,
			want: "failed to open stack file",
		},
		"password conflict": {
			config: `resource "portainer_edge_stack" "s" {
  name                           = "x"
  deployment_type                = 0
  edge_groups                    = [1]
  repository_url                 = "https://example.com/r.git"
  repository_password            = "a"
  repository_password_wo         = "b"
  repository_password_wo_version = 1
}`,
			want: "cannot be specified when",
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
					Config:      mockProviderConfig(mock) + tc.config,
					ExpectError: edgeErrRe(tc.want),
				}},
			})
		})
	}
}

func TestEdgeStackResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeStacks(mock, 1, 2)
	config := mockProviderConfig(mock) + `resource "portainer_edge_stack" "s" {
  name               = "web"
  deployment_type    = 0
  edge_groups        = [1]
  stack_file_content = "services: {}"
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fake.mu.Lock()
					delete(fake.stacks, 1)
					fake.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestEdgeStack_UpgradeFromV300: stacks of every method written by v3.0.0
// (SDKv2) plan no change with this build, with the optional attributes set
// and left out.
func TestEdgeStack_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	newFakeEdgeStacks(mock, 1, 2, 3, 4, 5, 6, 7)
	fp := filepath.Join(t.TempDir(), "docker-compose.yml")
	if err := os.WriteFile(fp, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := mockProviderConfig(mock) + `
resource "portainer_edge_stack" "bare" {
  name               = "bare"
  deployment_type    = 0
  edge_groups        = [1]
  stack_file_content = "services: {}"
}
resource "portainer_edge_stack" "k8s" {
  name               = "k8s"
  deployment_type    = 1
  edge_groups        = [1, 2]
  registries         = [3]
  stack_file_content = "kind: List"
  kubernetes_prune   = false
  environment        = { FOO = "bar" }
  timeouts {
    create = "20m"
  }
}
resource "portainer_edge_stack" "file" {
  name            = "file"
  deployment_type = 0
  edge_groups     = [1]
  stack_file_path = "` + fp + `"
}
resource "portainer_edge_stack" "git" {
  name                          = "git"
  deployment_type               = 0
  edge_groups                   = [1]
  repository_url                = "https://github.com/acme/edge.git"
  git_repository_authentication = true
  repository_username           = "bot"
  repository_password           = "token"
  repository_git_credential_id  = 4
  stack_webhook                 = true
  update_interval               = "5m"
  relative_path                 = "/opt/edge"
}
resource "portainer_edge_stack" "gitauth" {
  name                          = "gitauth"
  deployment_type               = 1
  edge_groups                   = [1]
  repository_url                = "https://github.com/acme/edge.git"
  git_repository_authentication = true
  repository_username           = "bot"
  repository_password           = "token"
  kubernetes_force_apply        = true
}
resource "portainer_edge_stack" "gitbare" {
  name            = "gitbare"
  deployment_type = 0
  edge_groups     = [1]
  repository_url  = "https://github.com/acme/edge.git"
}
resource "portainer_edge_stack" "helm" {
  name            = "helm"
  deployment_type = 1
  edge_groups     = [1]
  helm_config {
    chart_url  = "https://charts.example.com"
    chart_name = "nginx"
  }
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

func TestAccEdgeStackResource(t *testing.T) {
	t.Skip("edge stacks deploy to edge environments, which a plain CE acceptance instance does not have")
}

// TestEdgeStackResource_CreatePullImage: Portainer's create endpoints have no
// re-pull field, so pull_image is not sent on create; the configured true is
// kept until the first update records it (RePullImage).
func TestEdgeStackResource_CreatePullImage(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	fake := newFakeEdgeStacks(mock, 1)
	hcl := func(content string) string {
		return mockProviderConfig(mock) + `resource "portainer_edge_stack" "s" {
  name               = "pull"
  deployment_type    = 0
  edge_groups        = [1]
  stack_file_content = "` + content + `"
  pull_image         = true
}`
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			edgeStackStep(hcl("v1"), resource.TestCheckResourceAttr("portainer_edge_stack.s", "pull_image", "true")),
			edgeStackStep(hcl("v2"), resource.TestCheckResourceAttr("portainer_edge_stack.s", "pull_image", "true")),
		},
	})
	create := fake.last("POST string")
	for _, key := range []string{"rePullImage", "repullImageAndRedeploy", "pullImage", "autoUpdate"} {
		if _, ok := create[key]; ok {
			t.Errorf("create must not send %s: %v", key, create)
		}
	}
	if update := fake.last("PUT "); update["rePullImage"] != true || update["repullImageAndRedeploy"] != true {
		t.Errorf("update must send the re-pull flags: %v", update)
	}
}

// TestEdgeStackResource_ImportReadsPerDeviceAndStagger: an import reads the
// per-device configurations and a non-default stagger_config, which later
// reads refresh only while configured; Portainer's default stagger
// configuration is not imported, so a configuration without the block
// matches.
func TestEdgeStackResource_ImportReadsPerDeviceAndStagger(t *testing.T) {
	t.Parallel()
	gitStack := func(stagger map[string]interface{}) map[string]interface{} {
		return map[string]interface{}{
			"Name": "git", "DeploymentType": 0, "EdgeGroups": []int{1}, "Registries": []int{}, "EnvVars": []interface{}{},
			"GitConfig":           map[string]interface{}{"URL": "https://github.com/acme/edge.git", "ReferenceName": "refs/heads/main", "ConfigFilePath": "docker-compose.yml"},
			"SupportRelativePath": true, "FilesystemPath": "/opt/edge",
			"SupportPerDeviceConfigs": true, "PerDeviceConfigsPath": "configs", "PerDeviceConfigsMatchType": "dir", "PerDeviceConfigsGroupMatchType": "file",
			"StaggerConfig": stagger,
		}
	}
	const base = `resource "portainer_edge_stack" "s" {
  name                                = "git"
  deployment_type                     = 0
  edge_groups                         = [1]
  repository_url                      = "https://github.com/acme/edge.git"
  relative_path                       = "/opt/edge"
  per_device_configs_path             = "configs"
  per_device_configs_match_type       = "dir"
  per_device_configs_group_match_type = "file"
%s}`
	for name, tc := range map[string]struct {
		stagger map[string]interface{}
		block   string
		checks  []resource.TestCheckFunc
	}{
		"custom stagger": {
			stagger: map[string]interface{}{"StaggerOption": 2, "StaggerParallelOption": 1, "DeviceNumber": 3, "UpdateFailureAction": 2},
			block: `  stagger_config {
    stagger_option        = "parallel"
    parallel_option       = "fixed"
    device_number         = 3
    update_failure_action = "pause"
  }
`,
			checks: []resource.TestCheckFunc{
				resource.TestCheckResourceAttr("portainer_edge_stack.s", "stagger_config.0.stagger_option", "parallel"),
				resource.TestCheckResourceAttr("portainer_edge_stack.s", "stagger_config.0.device_number", "3"),
				resource.TestCheckNoResourceAttr("portainer_edge_stack.s", "stagger_config.0.timeout"),
			},
		},
		"default stagger": {
			stagger: map[string]interface{}{"StaggerOption": 1},
			checks:  []resource.TestCheckFunc{resource.TestCheckResourceAttr("portainer_edge_stack.s", "stagger_config.#", "0")},
		},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			mock := NewMockServer(t)
			fake := newFakeEdgeStacks(mock, 1)
			fake.stacks[1] = gitStack(tc.stagger)
			fake.nextID = 1
			config := mockProviderConfig(mock) + fmt.Sprintf(base, tc.block)
			checks := append([]resource.TestCheckFunc{
				resource.TestCheckResourceAttr("portainer_edge_stack.s", "per_device_configs_path", "configs"),
				resource.TestCheckResourceAttr("portainer_edge_stack.s", "per_device_configs_match_type", "dir"),
				resource.TestCheckResourceAttr("portainer_edge_stack.s", "per_device_configs_group_match_type", "file"),
			}, tc.checks...)
			resource.UnitTest(t, resource.TestCase{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Steps: []resource.TestStep{
					{
						Config:             config,
						ResourceName:       "portainer_edge_stack.s",
						ImportState:        true,
						ImportStateId:      "1",
						ImportStatePersist: true,
					},
					{
						Config: config,
						ConfigPlanChecks: resource.ConfigPlanChecks{PreApply: []plancheck.PlanCheck{
							plancheck.ExpectResourceAction("portainer_edge_stack.s", plancheck.ResourceActionNoop),
						}},
						Check: resource.ComposeTestCheckFunc(checks...),
					},
				},
			})
		})
	}
}
