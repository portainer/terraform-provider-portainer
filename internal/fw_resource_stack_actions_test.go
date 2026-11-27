package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Tests of the one-shot stack and container resources: portainer_deploy,
// portainer_container_exec, portainer_stack_associate, portainer_stack_webhook,
// portainer_stack_migrate and portainer_stack_delete_by_name.

// ------------------------------------------------------------------ deploy

func dkDeployFake(f *dkFake, swarm bool) {
	if swarm {
		f.json("GET", `/endpoints/1/docker/swarm`, http.StatusOK, map[string]string{"ID": "sw1"})
	} else {
		f.json("GET", `/endpoints/1/docker/swarm`, http.StatusServiceUnavailable, map[string]string{"message": "not a swarm"})
	}
	f.json("GET", `/stacks`, http.StatusOK, []map[string]interface{}{{"Id": 4, "Name": "app", "Env": []map[string]string{{"name": "TAG", "value": "1"}, {"name": "X", "value": "y"}}}})
	f.json("GET", `/stacks/4/file`, http.StatusOK, map[string]string{"StackFileContent": "services: {}"})
	f.json("GET", `/stacks/4`, http.StatusOK, map[string]interface{}{"Id": 4, "Status": 1})
	f.json("PUT", `/stacks/4`, http.StatusOK, map[string]interface{}{"Id": 4})
	f.json("GET", `/endpoints/1/docker/services`, http.StatusOK, []map[string]interface{}{
		{"Spec": map[string]interface{}{"Name": "app_web", "TaskTemplate": map[string]interface{}{"ContainerSpec": map[string]interface{}{"Image": "repo/web:1@sha256:abc"}}, "Labels": map[string]interface{}{}}, "Version": map[string]interface{}{"Index": 12}},
		{"Spec": map[string]interface{}{"Name": "app_db", "TaskTemplate": map[string]interface{}{"ContainerSpec": map[string]interface{}{"Image": "repo/db:2"}}}, "Version": map[string]interface{}{"Index": 3}},
	})
	f.json("POST", `/endpoints/1/docker/services/app_web/update`, http.StatusOK, map[string]interface{}{"Warnings": nil})
}

const dkDeployConfig = `
resource "portainer_deploy" "d" {
  endpoint_id   = 1
  stack_name    = "app"
  stack_env_var = "TAG"
  revision      = "2"
  services_list = "web"
}`

// TestDeployResource_Swarm updates the image of the listed services and the
// revision variable of the stack.
func TestDeployResource_Swarm(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	dkDeployFake(f, true)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: f.provider() + dkDeployConfig,
			Check: resource.ComposeTestCheckFunc(
				resource.TestMatchResourceAttr("portainer_deploy.d", "id", regexp.MustCompile(`^deploy-\d+$`)),
				resource.TestMatchResourceAttr("portainer_deploy.d", "output", regexp.MustCompile(`Service "app_web" updated to "repo/web:2"`)),
				func(*terraform.State) error {
					svc := f.last(t, "POST", "/endpoints/1/docker/services/app_web/update")
					if svc.Query != "version=12" || !strings.Contains(string(svc.Body), `"Image":"repo/web:2"`) {
						return fmt.Errorf("unexpected service update %s %s", svc.Query, svc.Body)
					}
					if len(f.requests("POST", "/endpoints/1/docker/services/app_db/update")) != 0 {
						return fmt.Errorf("a service not in services_list was updated")
					}
					put := dkBody(t, f.last(t, "PUT", "/stacks/4"))
					if fmt.Sprint(put["Env"]) != "[map[name:TAG value:2] map[name:X value:y]]" || put["Prune"] != true {
						return fmt.Errorf("unexpected stack update %v", put)
					}
					return nil
				},
			),
		}},
	})
}

// TestDeployResource_Standalone only updates the stack variable.
func TestDeployResource_Standalone(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	dkDeployFake(f, false)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: f.provider() + dkDeployConfig,
				Check: func(*terraform.State) error {
					put := dkBody(t, f.last(t, "PUT", "/stacks/4"))
					if put["pullImage"] != true || put["prune"] != true || fmt.Sprint(put["env"]) != "[map[name:TAG value:2] map[name:X value:y]]" {
						return fmt.Errorf("unexpected stack update %v", put)
					}
					return nil
				},
			},
			{
				Config:      f.provider() + strings.Replace(dkDeployConfig, `"web"`, `" "`, 1),
				ExpectError: regexp.MustCompile(`services_list must not be empty`),
			},
		},
	})
}

func TestDeployResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	dkDeployFake(f, false)
	dkUpgradeTest(t, f.provider()+dkDeployConfig)
}

// --------------------------------------------------------- container exec

func dkExecFake(f *dkFake) {
	f.json("GET", `/endpoints/1/docker/containers/json`, http.StatusOK, []map[string]interface{}{{"Id": "c1"}})
	f.json("POST", `/endpoints/1/docker/containers/c1/exec`, http.StatusCreated, map[string]string{"Id": "e1"})
	f.on("POST", `/endpoints/1/docker/exec/e1/start`, func(w http.ResponseWriter, _ *http.Request, _ []byte, _ []string) {
		_, _ = w.Write([]byte("hello\n"))
	})
	f.json("GET", `/endpoints/1/docker/tasks`, http.StatusOK, []map[string]interface{}{{"NodeID": "n1", "Status": map[string]interface{}{"ContainerStatus": map[string]interface{}{"ContainerID": "c1"}}}})
	f.json("GET", `/endpoints/1/docker/nodes/n1`, http.StatusOK, map[string]interface{}{"Description": map[string]interface{}{"Hostname": "node-1"}})
}

func TestContainerExecResource_Standalone(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	dkExecFake(f)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: f.provider() + `
resource "portainer_container_exec" "e" {
  endpoint_id  = 1
  service_name = "web"
  command      = "echo hello"
}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckResourceAttr("portainer_container_exec.e", "id", "e1"),
				resource.TestCheckResourceAttr("portainer_container_exec.e", "output", "hello\n"),
				resource.TestCheckResourceAttr("portainer_container_exec.e", "user", "root:root"),
				func(*terraform.State) error {
					p := dkBody(t, f.last(t, "POST", "/endpoints/1/docker/containers/c1/exec"))
					if p["User"] != "root:root" || fmt.Sprint(p["Cmd"]) != "[echo hello]" {
						return fmt.Errorf("unexpected exec %v", p)
					}
					return nil
				},
			),
		}},
	})
}

func TestContainerExecResource_Swarm(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	dkExecFake(f)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: f.provider() + `
resource "portainer_container_exec" "e" {
  endpoint_id  = 1
  service_name = "app_web"
  command      = "ls"
  mode         = "swarm"
}`,
			Check: func(*terraform.State) error {
				if h := f.last(t, "POST", "/endpoints/1/docker/exec/e1/start").Headers.Get("X-PortainerAgent-Target"); h != "node-1" {
					return fmt.Errorf("exec not routed to the node: %q", h)
				}
				return nil
			},
		}},
	})
}

// TestContainerExecResource_UnscheduledTask: a task without a container is
// an error, not a crash.
func TestContainerExecResource_UnscheduledTask(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	dkExecFake(f)
	f.json("GET", `/endpoints/1/docker/tasks`, http.StatusOK, []map[string]interface{}{{"Status": map[string]interface{}{}}})
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: f.provider() + `
resource "portainer_container_exec" "e" {
  endpoint_id  = 1
  service_name = "app_web"
  command      = "ls"
  mode         = "swarm"
}`,
			ExpectError: regexp.MustCompile(`has no container yet`),
		}},
	})
}

func TestContainerExecResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	dkExecFake(f)
	dkUpgradeTest(t, f.provider()+`
resource "portainer_container_exec" "e" {
  endpoint_id  = 1
  service_name = "web"
  command      = "echo hello"
}`)
}

// --------------------------------------------------------- stack associate

func TestStackAssociateResource_Lifecycle(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	endpoint := 2
	f.json("PUT", `/stacks/9/associate`, http.StatusOK, map[string]interface{}{"Id": 9})
	f.on("GET", `/stacks/9`, func(w http.ResponseWriter, _ *http.Request, _ []byte, _ []string) {
		dkWriteJSON(w, http.StatusOK, map[string]interface{}{"Id": 9, "EndpointId": endpoint})
	})
	config := f.provider() + `
resource "portainer_stack_associate" "a" {
  stack_id    = 9
  endpoint_id = 2
  swarm_id    = "sw1"
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_stack_associate.a", "id", "9"),
					func(*terraform.State) error {
						if q := f.last(t, "PUT", "/stacks/9/associate").Query; q != "endpointId=2&orphanedRunning=false&swarmId=sw1" {
							return fmt.Errorf("unexpected query %q", q)
						}
						return nil
					},
				),
			},
			{
				// The stack moved to another environment: re-associate.
				PreConfig:          func() { endpoint = 3 },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestStackAssociateResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	f.json("PUT", `/stacks/9/associate`, http.StatusOK, map[string]interface{}{"Id": 9})
	f.json("GET", `/stacks/9`, http.StatusOK, map[string]interface{}{"Id": 9, "EndpointId": 2})
	dkUpgradeTest(t, f.provider()+`
resource "portainer_stack_associate" "a" {
  stack_id    = 9
  endpoint_id = 2
  swarm_id    = "sw1"
}`)
}

// ----------------------------------------------------------- stack webhook

func TestStackWebhookResource(t *testing.T) {
	t.Parallel()
	for _, status := range []int{http.StatusOK, http.StatusAccepted, http.StatusNoContent} {
		f := newDkFake(t)
		f.on("POST", `/stacks/webhooks/abc`, func(w http.ResponseWriter, _ *http.Request, _ []byte, _ []string) { w.WriteHeader(status) })
		resource.UnitTest(t, resource.TestCase{
			ProtoV5ProviderFactories: testProtoV5Factories,
			Steps: []resource.TestStep{{
				Config: f.provider() + `resource "portainer_stack_webhook" "w" { webhook_id = "abc" }`,
				Check:  resource.TestCheckResourceAttr("portainer_stack_webhook.w", "id", "abc"),
			}},
		})
	}
	f := newDkFake(t)
	f.json("POST", `/stacks/webhooks/abc`, http.StatusBadRequest, map[string]string{"message": "invalid"})
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:      f.provider() + `resource "portainer_stack_webhook" "w" { webhook_id = "abc" }`,
			ExpectError: regexp.MustCompile(`failed to trigger webhook, status 400`),
		}},
	})
}

func TestStackWebhookResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	f.on("POST", `/stacks/webhooks/abc`, func(w http.ResponseWriter, _ *http.Request, _ []byte, _ []string) {
		w.WriteHeader(http.StatusNoContent)
	})
	dkUpgradeTest(t, f.provider()+`resource "portainer_stack_webhook" "w" { webhook_id = "abc" }`)
}

// ----------------------------------------------------------- stack migrate

func TestStackMigrateResource(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	f.json("POST", `/stacks/5/migrate`, http.StatusOK, map[string]interface{}{"Id": 5})
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: f.provider() + `
resource "portainer_stack_migrate" "m" {
  stack_id           = 5
  target_endpoint_id = 3
}`,
				Check: resource.ComposeTestCheckFunc(
					resource.TestMatchResourceAttr("portainer_stack_migrate.m", "id", regexp.MustCompile(`^5-\d+$`)),
					func(*terraform.State) error {
						req := f.last(t, "POST", "/stacks/5/migrate")
						if req.Query != "" || string(req.Body) != `{"EndpointID":3}`+"\n" && string(req.Body) != `{"EndpointID":3}` {
							return fmt.Errorf("unexpected migrate %q %s", req.Query, req.Body)
						}
						return nil
					},
				),
			},
			{
				Config: f.provider() + `
resource "portainer_stack_migrate" "m" {
  stack_id           = 5
  target_endpoint_id = 3
  endpoint_id        = 1
  stack_name         = "moved"
  swarm_id           = "sw1"
  is_helm            = true
}`,
				Check: func(*terraform.State) error {
					req := f.last(t, "POST", "/stacks/5/migrate")
					p := dkBody(t, req)
					if req.Query != "endpointId=1" || p["Name"] != "moved" || p["SwarmID"] != "sw1" || p["IsHelm"] != true {
						return fmt.Errorf("unexpected migrate %q %v", req.Query, p)
					}
					return nil
				},
			},
		},
	})
}

func TestStackMigrateResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	f.json("POST", `/stacks/5/migrate`, http.StatusOK, map[string]interface{}{"Id": 5})
	dkUpgradeTest(t, f.provider()+`
resource "portainer_stack_migrate" "m" {
  stack_id           = 5
  target_endpoint_id = 3
}`)
}

// ---------------------------------------------------- stack delete by name

func TestStackDeleteByNameResource(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	f.on("DELETE", `/stacks/name/web`, func(w http.ResponseWriter, _ *http.Request, _ []byte, _ []string) {
		w.WriteHeader(http.StatusNoContent)
	})
	f.json("DELETE", `/stacks/name/gone`, http.StatusNotFound, map[string]string{"message": "not found"})
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: f.provider() + `
resource "portainer_stack_delete_by_name" "d" {
  name        = "web"
  namespace   = "default"
  endpoint_id = 1
  external    = true
}
resource "portainer_stack_delete_by_name" "gone" {
  name        = "gone"
  namespace   = "default"
  endpoint_id = 1
}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestMatchResourceAttr("portainer_stack_delete_by_name.d", "id", regexp.MustCompile(`^1/web/deleted/\d+$`)),
				func(*terraform.State) error {
					if q := f.last(t, "DELETE", "/stacks/name/web").Query; q != "endpointId=1&external=true&namespace=default" {
						return fmt.Errorf("unexpected query %q", q)
					}
					return nil
				},
			),
		}},
	})
}

func TestStackDeleteByNameResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	f.on("DELETE", `/stacks/name/web`, func(w http.ResponseWriter, _ *http.Request, _ []byte, _ []string) {
		w.WriteHeader(http.StatusNoContent)
	})
	dkUpgradeTest(t, f.provider()+`
resource "portainer_stack_delete_by_name" "d" {
  name        = "web"
  namespace   = "default"
  endpoint_id = 1
}`)
}
