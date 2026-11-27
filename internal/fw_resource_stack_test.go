package internal

import (
	"context"
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
	"time"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

func init() {
	// The deployment waits poll Portainer; tests do not wait in real time.
	stackSettlePollInterval = time.Millisecond
}

// fakeStack is one stack of fakeStacks.
type fakeStack struct {
	ID, EndpointID, Type, Status, RC int
	Name, SwarmID, Namespace         string
	Content, Webhook                 string
	Env                              []map[string]string
	Git                              map[string]interface{}
	AutoUpdate                       map[string]interface{}
	Option                           map[string]interface{}
	Helm                             map[string]interface{}
	// deployingPolls is how many reads still report status 3 (Deploying);
	// failWith turns the deployment into status 4 with that message.
	deployingPolls int
	failWith       string
}

func (s *fakeStack) view() map[string]interface{} {
	status := s.Status
	var history []map[string]interface{}
	if s.deployingPolls > 0 {
		s.deployingPolls--
		status = stackStatusDeploying
	} else if s.failWith != "" {
		status = stackStatusError
		history = []map[string]interface{}{{"Status": 3, "Time": time.Now().Unix()}, {"Status": 4, "Time": time.Now().Unix(), "Message": s.failWith}}
	}
	m := map[string]interface{}{
		"Id": s.ID, "Name": s.Name, "Status": status, "Type": s.Type, "EndpointId": s.EndpointID,
		"SwarmId": s.SwarmID, "namespace": s.Namespace, "Env": s.Env,
		"Portainer": map[string]interface{}{"ResourceControl": map[string]interface{}{"Id": s.RC}},
	}
	if s.RC != 0 {
		m["ResourceControl"] = map[string]interface{}{"Id": s.RC, "Type": 6, "ResourceId": strconv.Itoa(s.ID)}
	}
	if history != nil {
		m["DeploymentStatus"] = history
	}
	if s.Git != nil {
		m["gitConfig"] = s.Git
	}
	if s.AutoUpdate != nil {
		m["AutoUpdate"] = s.AutoUpdate
	}
	if s.Webhook != "" {
		m["webhook"] = s.Webhook
	}
	if s.Option != nil {
		m["Option"] = s.Option
	}
	if s.Helm != nil {
		m["helmConfig"] = s.Helm
	}
	return m
}

// fakeStacks is a stateful stand-in for Portainer's stack API.
type fakeStacks struct {
	mu      sync.Mutex
	nextID  int
	stacks  map[int]*fakeStack
	swarmID string
	rcs     map[int]map[string]interface{}
	// afterMutation, when set, adjusts a stack after a create or update
	// (e.g. to make it deploy asynchronously).
	afterMutation func(*fakeStack)
}

func newFakeStacks(f *dkFake) *fakeStacks {
	fs := &fakeStacks{stacks: map[int]*fakeStack{}, swarmID: "swarm-1", rcs: map[int]map[string]interface{}{}}
	fs.register(f)
	return fs
}

func dkEnv(v interface{}) []map[string]string {
	list, _ := v.([]interface{})
	out := make([]map[string]string, 0, len(list))
	for _, e := range list {
		m, _ := e.(map[string]interface{})
		n, _ := m["name"].(string)
		val, _ := m["value"].(string)
		out = append(out, map[string]string{"name": n, "value": val})
	}
	return out
}

func (fs *fakeStacks) withStack(w http.ResponseWriter, m []string, fn func(s *fakeStack)) {
	id, _ := strconv.Atoi(m[1])
	fs.mu.Lock()
	s, ok := fs.stacks[id]
	if !ok {
		fs.mu.Unlock()
		dkWriteJSON(w, http.StatusNotFound, map[string]string{"message": "stack not found"})
		return
	}
	fn(s)
	fs.mu.Unlock()
}

func (fs *fakeStacks) mutated(s *fakeStack) {
	if fs.afterMutation != nil {
		fs.afterMutation(s)
	}
}

func (fs *fakeStacks) register(f *dkFake) {
	f.json("GET", `/endpoints/\d+/docker/swarm`, http.StatusOK, map[string]string{"ID": "swarm-1"})
	f.on("GET", `/stacks`, func(w http.ResponseWriter, _ *http.Request, _ []byte, _ []string) {
		fs.mu.Lock()
		defer fs.mu.Unlock()
		list := []map[string]interface{}{}
		for id := 1; id <= fs.nextID; id++ {
			if s, ok := fs.stacks[id]; ok {
				list = append(list, map[string]interface{}{"Id": s.ID, "Name": s.Name, "EndpointId": s.EndpointID, "Type": s.Type, "SwarmId": s.SwarmID, "Env": s.Env})
			}
		}
		dkWriteJSON(w, http.StatusOK, list)
	})
	create := func(stackType int) func(w http.ResponseWriter, r *http.Request, body []byte, m []string) {
		return func(w http.ResponseWriter, r *http.Request, body []byte, m []string) {
			var p map[string]interface{}
			_ = json.Unmarshal(body, &p)
			fs.mu.Lock()
			defer fs.mu.Unlock()
			fs.nextID++
			endpointID, _ := strconv.Atoi(r.URL.Query().Get("endpointId"))
			name, _ := p["name"].(string)
			if name == "" {
				name, _ = p["stackName"].(string)
			}
			s := &fakeStack{ID: fs.nextID, Name: name, EndpointID: endpointID, Type: stackType, Status: 1, Env: dkEnv(p["env"])}
			s.Content, _ = p["stackFileContent"].(string)
			s.SwarmID, _ = p["swarmID"].(string)
			s.Namespace, _ = p["namespace"].(string)
			if m[1] == "repository" {
				file, _ := p["composeFile"].(string)
				if file == "" {
					file, _ = p["manifestFile"].(string)
				}
				s.Git = map[string]interface{}{"URL": p["repositoryURL"], "ReferenceName": p["repositoryReferenceName"], "ConfigFilePath": file, "AdditionalFiles": p["additionalFiles"], "tlsskipVerify": p["tlsskipVerify"]}
				if au, ok := p["autoUpdate"].(map[string]interface{}); ok {
					s.AutoUpdate = map[string]interface{}{"Interval": au["interval"], "Webhook": au["webhook"], "ForcePullImage": au["forcePullImage"]}
				}
				if chart, ok := p["helmChartPath"].(string); ok && chart != "" {
					s.Helm = map[string]interface{}{"chartPath": chart, "atomic": p["atomic"] == true}
				}
			}
			if stackType == 3 {
				opt := map[string]interface{}{}
				if v, ok := p["prune"]; ok {
					opt["KubernetesPrune"] = v
				}
				if v, ok := p["forceApply"]; ok {
					opt["KubernetesForceApply"] = v
				}
				s.Option = opt
			}
			fs.stacks[s.ID] = s
			fs.mutated(s)
			dkWriteJSON(w, http.StatusOK, map[string]interface{}{"Id": s.ID})
		}
	}
	f.on("POST", `/stacks/create/standalone/(string|repository)`, create(2))
	f.on("POST", `/stacks/create/swarm/(string|repository)`, create(1))
	f.on("POST", `/stacks/create/kubernetes/(string|repository|url)`, create(3))
	f.on("GET", `/stacks/(\d+)`, func(w http.ResponseWriter, _ *http.Request, _ []byte, m []string) {
		fs.withStack(w, m, func(s *fakeStack) { dkWriteJSON(w, http.StatusOK, s.view()) })
	})
	f.on("GET", `/stacks/(\d+)/file`, func(w http.ResponseWriter, _ *http.Request, _ []byte, m []string) {
		fs.withStack(w, m, func(s *fakeStack) {
			dkWriteJSON(w, http.StatusOK, map[string]string{"StackFileContent": s.Content})
		})
	})
	f.on("PUT", `/stacks/(\d+)`, func(w http.ResponseWriter, _ *http.Request, body []byte, m []string) {
		var p map[string]interface{}
		_ = json.Unmarshal(body, &p)
		fs.withStack(w, m, func(s *fakeStack) {
			s.Content, _ = p["stackFileContent"].(string)
			s.Env = dkEnv(p["env"])
			if wh, ok := p["webhook"].(string); ok {
				s.Webhook = wh
			}
			s.Status = 1
			fs.mutated(s)
			dkWriteJSON(w, http.StatusOK, map[string]interface{}{"Id": s.ID})
		})
	})
	f.on("POST", `/stacks/(\d+)/git`, func(w http.ResponseWriter, _ *http.Request, body []byte, m []string) {
		var p map[string]interface{}
		_ = json.Unmarshal(body, &p)
		fs.withStack(w, m, func(s *fakeStack) {
			if s.Git == nil {
				s.Git = map[string]interface{}{}
			}
			s.Git["ReferenceName"] = p["repositoryReferenceName"]
			if cfp, ok := p["configFilePath"]; ok {
				s.Git["ConfigFilePath"] = cfp
			}
			if au, ok := p["AutoUpdate"].(map[string]interface{}); ok {
				s.AutoUpdate = map[string]interface{}{"Interval": au["Interval"], "Webhook": au["Webhook"], "ForcePullImage": au["ForcePullImage"]}
			} else {
				s.AutoUpdate = nil
			}
			if s.Option == nil {
				s.Option = map[string]interface{}{}
			}
			if v, ok := p["kubernetesPrune"]; ok {
				s.Option["KubernetesPrune"] = v
			}
			if v, ok := p["kubernetesForceApply"]; ok {
				s.Option["KubernetesForceApply"] = v
			}
			if v, ok := p["atomic"]; ok && s.Helm != nil {
				s.Helm["atomic"] = v
			}
			dkWriteJSON(w, http.StatusOK, map[string]interface{}{"Id": s.ID, "GitConfig": s.Git})
		})
	})
	f.on("PUT", `/stacks/(\d+)/git/redeploy`, func(w http.ResponseWriter, _ *http.Request, body []byte, m []string) {
		var p map[string]interface{}
		_ = json.Unmarshal(body, &p)
		fs.withStack(w, m, func(s *fakeStack) {
			s.Env = dkEnv(p["env"])
			s.Status = 1
			fs.mutated(s)
			dkWriteJSON(w, http.StatusOK, map[string]interface{}{"Id": s.ID})
		})
	})
	f.on("POST", `/stacks/(\d+)/(stop|start)`, func(w http.ResponseWriter, r *http.Request, _ []byte, m []string) {
		fs.withStack(w, m, func(s *fakeStack) {
			s.Status = 2
			if strings.HasSuffix(r.URL.Path, "/start") {
				s.Status = 1
			}
			dkWriteJSON(w, http.StatusOK, map[string]interface{}{"Id": s.ID})
		})
	})
	f.on("DELETE", `/stacks/(\d+)`, func(w http.ResponseWriter, _ *http.Request, _ []byte, m []string) {
		id, _ := strconv.Atoi(m[1])
		fs.mu.Lock()
		delete(fs.stacks, id)
		fs.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
	f.on("GET", `/resource_controls/(\d+)`, func(w http.ResponseWriter, _ *http.Request, _ []byte, m []string) {
		id, _ := strconv.Atoi(m[1])
		fs.mu.Lock()
		defer fs.mu.Unlock()
		rc, ok := fs.rcs[id]
		if !ok {
			rc = map[string]interface{}{"Public": false, "AdministratorsOnly": true}
		}
		dkWriteJSON(w, http.StatusOK, rc)
	})
	f.on("PUT", `/resource_controls/(\d+)`, func(w http.ResponseWriter, _ *http.Request, body []byte, m []string) {
		id, _ := strconv.Atoi(m[1])
		var p map[string]interface{}
		_ = json.Unmarshal(body, &p)
		rc := map[string]interface{}{"Public": p["public"], "AdministratorsOnly": p["administratorsOnly"]}
		userIDs, _ := p["users"].([]interface{})
		teamIDs, _ := p["teams"].([]interface{})
		users := make([]map[string]interface{}, 0, len(userIDs))
		teams := make([]map[string]interface{}, 0, len(teamIDs))
		for _, u := range userIDs {
			users = append(users, map[string]interface{}{"UserId": u})
		}
		for _, t := range teamIDs {
			teams = append(teams, map[string]interface{}{"TeamId": t})
		}
		rc["UserAccesses"], rc["TeamAccesses"] = users, teams
		fs.mu.Lock()
		fs.rcs[id] = rc
		fs.mu.Unlock()
		dkWriteJSON(w, http.StatusOK, rc)
	})
}

func (fs *fakeStacks) get(id int) *fakeStack {
	fs.mu.Lock()
	defer fs.mu.Unlock()
	return fs.stacks[id]
}

const stackStringConfig = `
resource "portainer_stack" "s" {
  name               = "web"
  deployment_type    = "standalone"
  method             = "string"
  endpoint_id        = 1
  stack_file_content = %q
  active             = %v
%s
}
`

// TestStackResource_StandaloneStringLifecycle: create sends the stack and
// finalizes it with a PUT, an empty plan follows, a content change updates in
// place, active = false stops the stack after the redeploy, the stack
// imports, and destroy deletes it.
func TestStackResource_StandaloneStringLifecycle(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	fs := newFakeStacks(f)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: f.provider() + fmt.Sprintf(stackStringConfig, "services: {}", true, `
  env {
    name  = "A"
    value = "1"
  }
  registries = [3]`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_stack.s", "id", "1"),
					resource.TestCheckResourceAttr("portainer_stack.s", "active", "true"),
					resource.TestCheckResourceAttr("portainer_stack.s", "swarm_id", ""),
					resource.TestCheckNoResourceAttr("portainer_stack.s", "namespace"),
					resource.TestCheckResourceAttr("portainer_stack.s", "webhook_url", ""),
					func(*terraform.State) error {
						post := dkBody(t, f.last(t, "POST", "/stacks/create/standalone/string"))
						if post["name"] != "web" || post["stackFileContent"] != "services: {}" || post["fromAppTemplate"] != false {
							return fmt.Errorf("unexpected create payload %v", post)
						}
						if q := f.last(t, "POST", "/stacks/create/standalone/string").Query; q != "endpointId=1" {
							return fmt.Errorf("create query %q", q)
						}
						put := dkBody(t, f.last(t, "PUT", "/stacks/1"))
						if put["prune"] != false || put["pullImage"] != false || fmt.Sprint(put["registries"]) != "[3]" {
							return fmt.Errorf("unexpected finalize payload %v", put)
						}
						if _, ok := put["webhook"]; ok {
							return fmt.Errorf("finalize sent a webhook without stack_webhook: %v", put)
						}
						if _, ok := put["repullImageAndRedeploy"]; ok {
							return fmt.Errorf("repullImageAndRedeploy must be left out without pull_image: %v", put)
						}
						return nil
					},
				),
			},
			{
				Config: f.provider() + fmt.Sprintf(stackStringConfig, "services: {web: {}}", false, ""),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_stack.s", plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_stack.s", "active", "false"),
					resource.TestCheckResourceAttr("portainer_stack.s", "env.#", "0"),
					func(*terraform.State) error {
						put := dkBody(t, f.last(t, "PUT", "/stacks/1"))
						if put["stackFileContent"] != "services: {web: {}}" {
							return fmt.Errorf("update sent %v", put)
						}
						if len(f.requests("POST", "/stacks/1/stop")) != 1 {
							return fmt.Errorf("active = false did not stop the stack")
						}
						return nil
					},
				),
			},
			{
				ResourceName:            "portainer_stack.s",
				ImportState:             true,
				ImportStateId:           "1:1:standalone:string",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"registries", "file_path_in_repository"},
			},
		},
	})
	if fs.get(1) != nil {
		t.Error("destroy did not delete the stack")
	}
}

// TestStackResource_SwarmFetchesSwarmID: a swarm stack without swarm_id gets
// the swarm of the environment.
func TestStackResource_SwarmFetchesSwarmID(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	newFakeStacks(f)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: f.provider() + `
resource "portainer_stack" "s" {
  name               = "svc"
  deployment_type    = "swarm"
  method             = "string"
  endpoint_id        = 2
  stack_file_content = "services: {}"
}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_stack.s", "swarm_id", "swarm-1"),
				func(*terraform.State) error {
					if p := dkBody(t, f.last(t, "POST", "/stacks/create/swarm/string")); p["swarmID"] != "swarm-1" {
						return fmt.Errorf("swarmID not sent: %v", p)
					}
					return nil
				},
			),
		}},
	})
}

// TestStackResource_RepositoryLifecycle: a Git stack is created with the
// repository settings and a webhook, updated through the git settings and a
// redeploy, and imported without a method.
func TestStackResource_RepositoryLifecycle(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	newFakeStacks(f)

	config := func(ref string) string {
		return f.provider() + fmt.Sprintf(`
resource "portainer_stack" "s" {
  name                      = "git"
  deployment_type           = "standalone"
  method                    = "repository"
  endpoint_id               = 1
  repository_url            = "https://example.com/repo.git"
  repository_reference_name = %q
  file_path_in_repository   = "docker-compose.yml"
  stack_webhook             = true
  update_interval           = "5m"
  env {
    name  = "A"
    value = "1"
  }
}`, ref)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config("refs/heads/main"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttrSet("portainer_stack.s", "webhook_id"),
					resource.TestMatchResourceAttr("portainer_stack.s", "webhook_url", regexp.MustCompile(`/api/stacks/webhooks/`)),
					func(*terraform.State) error {
						p := dkBody(t, f.last(t, "POST", "/stacks/create/standalone/repository"))
						if p["composeFile"] != "docker-compose.yml" || p["repositoryURL"] != "https://example.com/repo.git" || p["repositoryReferenceName"] != "refs/heads/main" {
							return fmt.Errorf("unexpected create payload %v", p)
						}
						au, _ := p["autoUpdate"].(map[string]interface{})
						if au["interval"] != "5m" || au["webhook"] == "" {
							return fmt.Errorf("unexpected autoUpdate %v", au)
						}
						if len(f.requests("PUT", "/stacks/1")) != 0 {
							return fmt.Errorf("a repository stack must not be finalized with PUT /stacks/1")
						}
						return nil
					},
				),
			},
			{
				Config: config("refs/heads/dev"),
				Check: func(*terraform.State) error {
					git := dkBody(t, f.last(t, "POST", "/stacks/1/git"))
					if git["repositoryReferenceName"] != "refs/heads/dev" || git["configFilePath"] != "docker-compose.yml" {
						return fmt.Errorf("unexpected git settings %v", git)
					}
					au, _ := git["AutoUpdate"].(map[string]interface{})
					if au["Interval"] != "5m" || au["Webhook"] == "" {
						return fmt.Errorf("unexpected AutoUpdate %v", au)
					}
					redeploy := dkBody(t, f.last(t, "PUT", "/stacks/1/git/redeploy"))
					if redeploy["stackName"] != "git" || fmt.Sprint(redeploy["env"]) != "[map[name:A value:1]]" {
						return fmt.Errorf("unexpected redeploy %v", redeploy)
					}
					return nil
				},
			},
			{
				ResourceName:      "portainer_stack.s",
				ImportState:       true,
				ImportStateId:     "1-1-standalone",
				ImportStateVerify: true,
			},
		},
	})
}

// TestStackResource_RepullImageAndRedeploy: pull_image also sends
// RepullImageAndRedeploy, which replaces the deprecated pullImage, on the
// stack update and on the git redeploy.
func TestStackResource_RepullImageAndRedeploy(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	newFakeStacks(f)

	config := func(content string) string {
		return f.provider() + fmt.Sprintf(stackStringConfig, content, true, "  pull_image = true") + fmt.Sprintf(`
resource "portainer_stack" "git" {
  name                      = "git"
  deployment_type           = "standalone"
  method                    = "repository"
  endpoint_id               = 1
  repository_url            = "https://example.com/repo.git"
  repository_reference_name = "refs/heads/%s"
  file_path_in_repository   = "docker-compose.yml"
  pull_image                = true
  update_interval           = "5m"
  depends_on                = [portainer_stack.s]
}`, content)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config("main"),
				Check: func(*terraform.State) error {
					put := dkBody(t, f.last(t, "PUT", "/stacks/1"))
					if put["pullImage"] != true || put["repullImageAndRedeploy"] != true {
						return fmt.Errorf("unexpected finalize payload %v", put)
					}
					return nil
				},
			},
			{
				Config: config("dev"),
				Check: func(*terraform.State) error {
					put := dkBody(t, f.last(t, "PUT", "/stacks/1"))
					if put["stackFileContent"] != "dev" || put["repullImageAndRedeploy"] != true {
						return fmt.Errorf("unexpected update payload %v", put)
					}
					redeploy := dkBody(t, f.last(t, "PUT", "/stacks/2/git/redeploy"))
					if redeploy["pullImage"] != true || redeploy["repullImageAndRedeploy"] != true {
						return fmt.Errorf("unexpected redeploy payload %v", redeploy)
					}
					return nil
				},
			},
		},
	})
}

// TestStackResource_WriteOnlyCredentialsAndEnv: write-only repository
// credentials and env_wo values are sent and never stored.
//
// Read stores the repository URL Portainer reports, so a configuration that
// only sets repository_url_wo plans to replace the stack afterwards; v3.0.0
// did the same, and that is kept.
func TestStackResource_WriteOnlyCredentialsAndEnv(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	newFakeStacks(f)

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: f.provider() + `
resource "portainer_stack" "s" {
  name                   = "git"
  deployment_type        = "swarm"
  method                 = "repository"
  endpoint_id            = 1
  swarm_id               = "swarm-1"
  repository_url_wo      = "https://example.com/secret.git"
  repository_username_wo = "bot"
  repository_password_wo = "s3cret"
  repository_wo_version  = 1
  env {
    name  = "PLAIN"
    value = "p"
  }
  env_wo {
    name  = "TOKEN"
    value = "hidden"
  }
  env_wo_version = 1
}`,
			ExpectNonEmptyPlan: true,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_stack.s", "env.#", "1"),
				resource.TestCheckResourceAttr("portainer_stack.s", "env_wo.#", "1"),
				resource.TestCheckResourceAttr("portainer_stack.s", "env_wo.0.name", "TOKEN"),
				resource.TestCheckNoResourceAttr("portainer_stack.s", "env_wo.0.value"),
				resource.TestCheckNoResourceAttr("portainer_stack.s", "repository_password_wo"),
				func(*terraform.State) error {
					p := dkBody(t, f.last(t, "POST", "/stacks/create/swarm/repository"))
					if p["repositoryURL"] != "https://example.com/secret.git" || p["repositoryUsername"] != "bot" || p["repositoryPassword"] != "s3cret" {
						return fmt.Errorf("write-only credentials not sent: %v", p)
					}
					if fmt.Sprint(p["env"]) != "[map[name:PLAIN value:p] map[name:TOKEN value:hidden]]" {
						return fmt.Errorf("env_wo not sent after env: %v", p["env"])
					}
					return nil
				},
			),
		}},
	})
}

// TestStackResource_EnvValidation: a variable in both env and env_wo, or twice
// in env_wo, is rejected at plan.
func TestStackResource_EnvValidation(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	newFakeStacks(f)
	base := f.provider() + `
resource "portainer_stack" "s" {
  name               = "web"
  deployment_type    = "standalone"
  method             = "string"
  endpoint_id        = 1
  stack_file_content = "services: {}"
  %s
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config:      fmt.Sprintf(base, "env {\n name = \"A\"\n value = \"1\"\n}\nenv_wo {\n name = \"A\"\n value = \"2\"\n}"),
				ExpectError: regexp.MustCompile(`variable "A" is declared in both env and env_wo`),
			},
			{
				Config:      fmt.Sprintf(base, "env_wo {\n name = \"B\"\n value = \"1\"\n}\nenv_wo {\n name = \"B\"\n value = \"2\"\n}"),
				ExpectError: regexp.MustCompile(`env_wo: variable "B" is declared more than once`),
			},
			{
				Config:      fmt.Sprintf(base, `kubernetes_prune = true`),
				ExpectError: regexp.MustCompile(`kubernetes_prune only applies to deployment_type = "kubernetes"`),
			},
			{
				Config:      fmt.Sprintf(base, `ownership = "everyone"`),
				ExpectError: regexp.MustCompile(`"ownership" must be one of 'public', 'private'`),
			},
		},
	})
}

// TestStackResource_KubernetesOptions: the apply policies are sent when
// configured, omitted otherwise, and a change replaces a string stack.
func TestStackResource_KubernetesOptions(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	newFakeStacks(f)
	config := func(extra string) string {
		return f.provider() + fmt.Sprintf(`
resource "portainer_stack" "k" {
  name               = "k"
  deployment_type    = "kubernetes"
  method             = "string"
  endpoint_id        = 1
  namespace          = "default"
  stack_file_content = "kind: List"
  %s
}`, extra)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config(""),
				Check: func(*terraform.State) error {
					p := dkBody(t, f.last(t, "POST", "/stacks/create/kubernetes/string"))
					if _, ok := p["prune"]; ok {
						return fmt.Errorf("prune sent while unset: %v", p)
					}
					if p["namespace"] != "default" || p["stackName"] != "k" {
						return fmt.Errorf("unexpected payload %v", p)
					}
					return nil
				},
			},
			{
				Config: config("kubernetes_prune = false\n  kubernetes_force_apply = true"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_stack.k", plancheck.ResourceActionDestroyBeforeCreate)},
				},
				Check: func(*terraform.State) error {
					p := dkBody(t, f.last(t, "POST", "/stacks/create/kubernetes/string"))
					if p["prune"] != false || p["forceApply"] != true {
						return fmt.Errorf("apply policies not sent: %v", p)
					}
					return nil
				},
			},
		},
	})
}

// TestStackResource_HelmAtomic: atomic (Business Edition 3.1) is sent on the
// create of a Helm chart stack and on the git settings update when configured,
// changed in place, read back while configured, and never sent when unset.
func TestStackResource_HelmAtomic(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	fs := newFakeStacks(f)
	config := func(extra string) string {
		return f.provider() + fmt.Sprintf(`
resource "portainer_stack" "h" {
  name            = "h"
  deployment_type = "kubernetes"
  method          = "repository"
  endpoint_id     = 1
  namespace       = "default"
  repository_url  = "https://example.com/charts.git"
  helm_chart_path = "charts/app"
  %s
}`, extra)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config(""),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckNoResourceAttr("portainer_stack.h", "atomic"),
					func(*terraform.State) error {
						p := dkBody(t, f.last(t, "POST", "/stacks/create/kubernetes/repository"))
						if _, ok := p["atomic"]; ok {
							return fmt.Errorf("atomic sent while unset: %v", p)
						}
						return nil
					},
				),
			},
			{
				Config: config("atomic = true"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_stack.h", plancheck.ResourceActionUpdate)},
				},
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_stack.h", "atomic", "true"),
					func(*terraform.State) error {
						p := dkBody(t, f.last(t, "POST", "/stacks/1/git"))
						if p["atomic"] != true {
							return fmt.Errorf("atomic not sent on the git settings update: %v", p)
						}
						return nil
					},
				),
			},
			{
				PreConfig: func() {
					fs.mu.Lock()
					fs.stacks[1].Helm["atomic"] = false
					fs.mu.Unlock()
				},
				Config:             config("atomic = true"),
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})

	f2 := newDkFake(t)
	newFakeStacks(f2)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: f2.provider() + `
resource "portainer_stack" "h" {
  name            = "h"
  deployment_type = "kubernetes"
  method          = "repository"
  endpoint_id     = 1
  repository_url  = "https://example.com/charts.git"
  helm_chart_path = "charts/app"
  atomic          = false
}`,
				Check: func(*terraform.State) error {
					p := dkBody(t, f2.last(t, "POST", "/stacks/create/kubernetes/repository"))
					if p["atomic"] != false || p["helmChartPath"] != "charts/app" {
						return fmt.Errorf("atomic not sent on create: %v", p)
					}
					return nil
				},
			},
			{
				Config: f2.provider() + `
resource "portainer_stack" "h" {
  name            = "h"
  deployment_type = "kubernetes"
  method          = "repository"
  endpoint_id     = 1
  repository_url  = "https://example.com/charts.git"
  file_path_in_repository = "app.yaml"
  atomic          = true
}`,
				ExpectError: regexp.MustCompile(`atomic only applies to a Helm chart stack`),
			},
		},
	})
}

// TestStackResource_ExistingStackIsAdopted: a stack of the same name on the
// environment is updated instead of created twice.
func TestStackResource_ExistingStackIsAdopted(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	fs := newFakeStacks(f)
	fs.nextID = 7
	fs.stacks[7] = &fakeStack{ID: 7, Name: "web", EndpointID: 1, Type: 2, Status: 1, Content: "old"}

	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: f.provider() + fmt.Sprintf(stackStringConfig, "new", true, ""),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_stack.s", "id", "7"),
				func(*terraform.State) error {
					if len(f.requests("POST", "/stacks/create/standalone/string")) != 0 {
						return fmt.Errorf("the existing stack was created again")
					}
					if fs.get(7).Content != "new" {
						return fmt.Errorf("the existing stack was not updated")
					}
					return nil
				},
			),
		}},
	})
}

// TestStackResource_GoneOutsideTerraform: a stack deleted in Portainer is
// planned for creation again.
func TestStackResource_GoneOutsideTerraform(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	fs := newFakeStacks(f)
	config := f.provider() + fmt.Sprintf(stackStringConfig, "services: {}", true, "")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					fs.mu.Lock()
					delete(fs.stacks, 1)
					fs.mu.Unlock()
				},
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

// TestStackResource_WaitsOutAsyncDeployment: Portainer 2.45 deploys
// asynchronously; the finalize call waits for the deployment, and a failed
// deployment fails the apply with Portainer's message.
func TestStackResource_WaitsOutAsyncDeployment(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	fs := newFakeStacks(f)
	fs.afterMutation = func(s *fakeStack) { s.deployingPolls = 3 }
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: f.provider() + fmt.Sprintf(stackStringConfig, "services: {}", true, ""),
			Check:  resource.TestCheckResourceAttr("portainer_stack.s", "active", "true"),
		}},
	})

	f2 := newDkFake(t)
	fs2 := newFakeStacks(f2)
	fs2.afterMutation = func(s *fakeStack) { s.failWith = "image not found" }
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      f2.provider() + fmt.Sprintf(stackStringConfig, "services: {}", true, ""),
			ExpectError: regexp.MustCompile(`deployment failed: image not found`),
		}},
	})
}

// TestStackResource_ReportsFailedRedeploy: an update whose redeploy fails in
// Portainer fails the apply with Portainer's message.
func TestStackResource_ReportsFailedRedeploy(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	fs := newFakeStacks(f)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: f.provider() + fmt.Sprintf(stackStringConfig, "services: {}", true, "")},
			{
				PreConfig:   func() { fs.afterMutation = func(s *fakeStack) { s.failWith = "boom" } },
				Config:      f.provider() + fmt.Sprintf(stackStringConfig, "services: {web: {}}", true, ""),
				ExpectError: regexp.MustCompile(`redeploying stack 1 failed in Portainer: boom`),
			},
			{
				// A stack left failed by an earlier deployment is recovered by the
				// next update rather than refused.
				PreConfig: func() {
					fs.afterMutation = func(s *fakeStack) { s.failWith = "" }
				},
				Config: f.provider() + fmt.Sprintf(stackStringConfig, "services: {api: {}}", true, ""),
			},
		},
	})
}

// TestStackResource_FileMethod follows the file at stack_file_path: no plan
// while it holds what was deployed (whitespace aside), an update when it
// changes.
func TestStackResource_FileMethod(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	fs := newFakeStacks(f)
	file := filepath.Join(t.TempDir(), "compose.yml")
	if err := os.WriteFile(file, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := f.provider() + fmt.Sprintf(`
resource "portainer_stack" "s" {
  name            = "file"
  deployment_type = "standalone"
  method          = "file"
  endpoint_id     = 1
  stack_file_path = %q
}`, file)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					// Portainer hands the content back without the newline.
					fs.get(1).Content = "services: {}"
				},
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				PreConfig: func() {
					_ = os.WriteFile(file, []byte("services: {web: {}}\n"), 0o600)
				},
				Config: config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_stack.s", plancheck.ResourceActionUpdate)},
				},
				Check: func(*terraform.State) error {
					if fs.get(1).Content != "services: {web: {}}\n" {
						return fmt.Errorf("the changed file was not deployed: %q", fs.get(1).Content)
					}
					return nil
				},
			},
		},
	})
}

// TestStackResource_Ownership: restricted ownership updates the stack's
// resource control and reads it back.
func TestStackResource_Ownership(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	fs := newFakeStacks(f)
	fs.afterMutation = func(s *fakeStack) { s.RC = 40 + s.ID }
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: f.provider() + fmt.Sprintf(stackStringConfig, "services: {}", true, `
  ownership        = "restricted"
  authorized_users = [5]`),
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_stack.s", "resource_control_id", "41"),
				resource.TestCheckResourceAttr("portainer_stack.s", "ownership", "restricted"),
				resource.TestCheckResourceAttr("portainer_stack.s", "authorized_users.#", "1"),
				resource.TestCheckNoResourceAttr("portainer_stack.s", "authorized_teams"),
				func(*terraform.State) error {
					p := dkBody(t, f.last(t, "PUT", "/resource_controls/41"))
					if p["public"] != false || p["administratorsOnly"] != false || fmt.Sprint(p["users"]) != "[5]" || fmt.Sprint(p["teams"]) != "[]" {
						return fmt.Errorf("unexpected resource control update %v", p)
					}
					return nil
				},
			),
		}},
	})
}

func TestStackResource_ImportErrors(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	fs := newFakeStacks(f)
	fs.nextID = 3
	fs.stacks[3] = &fakeStack{ID: 3, Name: "plain", EndpointID: 1, Type: 2, Status: 1}
	config := f.provider() + fmt.Sprintf(stackStringConfig, "services: {}", true, "")
	for id, want := range map[string]string{
		"1:3":              `expected <endpoint_id>:<stack_id>:<deployment_type>\[:<method>\]`,
		"x:3:standalone":   `invalid import ID "x:3:standalone"`,
		"1:3:standalone":   `stack 3 was not deployed from Git, so the import ID needs its method: 1:3:standalone:string`,
		"1::standalone:fi": `invalid import ID`,
	} {
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{{
				Config:        config,
				ResourceName:  "portainer_stack.s",
				ImportState:   true,
				ImportStateId: id,
				ExpectError:   regexp.MustCompile(want),
			}},
		})
	}
}

// TestStackResource_DeleteErrors: a permanent swarm error fails at once, and
// a stack already gone is deleted.
func TestStackResource_DeleteErrors(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	newFakeStacks(f)
	f.on("DELETE", `/stacks/1`, func(w http.ResponseWriter, _ *http.Request, _ []byte, _ []string) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"message":"This node is not a swarm manager."}`))
	})
	start := time.Now()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: f.provider() + fmt.Sprintf(stackStringConfig, "services: {}", true, "")},
			{
				Config:      f.provider(),
				ExpectError: regexp.MustCompile(`failed to delete stack: .*not a swarm manager`),
			},
			{
				PreConfig: func() {
					f.json("DELETE", `/stacks/1`, http.StatusNotFound, map[string]string{"message": "not found"})
				},
				Config: f.provider(),
			},
		},
	})
	if time.Since(start) > time.Minute {
		t.Error("the permanent error was retried")
	}
}

// TestWaitForStackSettled_Errors covers the waits directly: an unrelated
// conflict is not a deployment conflict, and a timeout names the stack.
func TestWaitForStackSettled_Errors(t *testing.T) {
	t.Parallel()
	if isStackDeploymentConflict(http.StatusConflict, "name already used") {
		t.Error("a conflict other than the deployment lock was treated as one")
	}
	if !isStackDeploymentConflict(http.StatusConflict, "Stack deployment is already in progress") {
		t.Error("the deployment lock was not recognised")
	}

	f := newDkFake(t)
	fs := newFakeStacks(f)
	fs.stacks[1] = &fakeStack{ID: 1, Status: 1, deployingPolls: 1 << 30}
	client := &APIClient{Endpoint: f.srv.URL, APIKey: "k", HTTPClient: http.Client{}}
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := waitForStackLockReleased(ctx, client, "1"); err == nil || !strings.Contains(err.Error(), "gave up waiting for stack 1") {
		t.Errorf("expected a timeout naming the stack, got %v", err)
	}

	fs.stacks[2] = &fakeStack{ID: 2, Status: 1, failWith: "boom"}
	if err := waitForStackLockReleased(context.Background(), client, "2"); err != nil {
		t.Errorf("a failed earlier deployment must not block a mutation: %v", err)
	}
	if err := waitForStackDeployment(context.Background(), client, "2"); err == nil || !strings.Contains(err.Error(), "boom") {
		t.Errorf("our own failed deployment must be reported, got %v", err)
	}
}

// TestStackResource_UpgradeFromV300: stacks written by v3.0.0 (SDKv2) against
// the same fake plan no change with this build.
func TestStackResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	newFakeStacks(f)
	file := filepath.Join(t.TempDir(), "compose.yml")
	if err := os.WriteFile(file, []byte("services: {}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	config := f.provider() + fmt.Sprintf(`
resource "portainer_stack" "minimal" {
  name               = "minimal"
  deployment_type    = "standalone"
  method             = "string"
  endpoint_id        = 1
  stack_file_content = "services: {}"
}

resource "portainer_stack" "full" {
  name               = "full"
  deployment_type    = "swarm"
  method             = "string"
  endpoint_id        = 1
  stack_file_content = "services: {}"
  prune              = true
  pull_image         = true
  stack_webhook      = true
  registries         = [1]
  env {
    name  = "A"
    value = "1"
  }
}

resource "portainer_stack" "file" {
  name            = "file"
  deployment_type = "standalone"
  method          = "file"
  endpoint_id     = 1
  stack_file_path = %q
}

resource "portainer_stack" "git" {
  name            = "git"
  deployment_type = "standalone"
  method          = "repository"
  endpoint_id     = 1
  repository_url  = "https://example.com/repo.git"
  file_path_in_repository = "docker-compose.yml"
  update_interval = "5m"
}

resource "portainer_stack" "k8s" {
  name               = "k8s"
  deployment_type    = "kubernetes"
  method             = "string"
  endpoint_id        = 1
  namespace          = "default"
  stack_file_content = "kind: List"
}
`, file)
	resource.Test(t, resource.TestCase{
		IsUnitTest: true,
		Steps: []resource.TestStep{
			{ExternalProviders: previousRelease, Config: config},
			{
				ProtoV5ProviderFactories: testProtoV5Factories,
				Config:                   config,
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
}

// TestStackResource_RepositoryDefaultFilePath: a Git stack that leaves
// file_path_in_repository out deploys docker-compose.yml, which Portainer
// records. The next plans keep that path (v3.0.0 planned it back to "" on
// every run), and an update sends it on.
func TestStackResource_RepositoryDefaultFilePath(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	newFakeStacks(f)

	config := func(ref string) string {
		return f.provider() + fmt.Sprintf(`
resource "portainer_stack" "s" {
  name                      = "git"
  deployment_type           = "standalone"
  method                    = "repository"
  endpoint_id               = 1
  repository_url            = "https://example.com/repo.git"
  repository_reference_name = %q
}`, ref)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config("refs/heads/main"),
				Check:  resource.TestCheckResourceAttr("portainer_stack.s", "file_path_in_repository", "docker-compose.yml"),
			},
			{
				Config: config("refs/heads/main"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
			{
				Config: config("refs/heads/dev"),
				Check: func(*terraform.State) error {
					git := dkBody(t, f.last(t, "POST", "/stacks/1/git"))
					if git["configFilePath"] != "docker-compose.yml" {
						return fmt.Errorf("an update must keep the recorded file path, got %v", git["configFilePath"])
					}
					return nil
				},
			},
		},
	})
}
