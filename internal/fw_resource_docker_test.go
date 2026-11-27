package internal

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/plancheck"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// dkObjects is a stateful stand-in for one kind of Docker object behind
// Portainer's Docker proxy: objects keyed by ID, as JSON documents.
type dkObjects struct {
	mu      sync.Mutex
	objects map[string]map[string]interface{}
	next    int
}

func newDkObjects() *dkObjects { return &dkObjects{objects: map[string]map[string]interface{}{}} }

func (o *dkObjects) put(id string, obj map[string]interface{}) {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.objects[id] = obj
}

func (o *dkObjects) get(id string) map[string]interface{} {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.objects[id]
}

func (o *dkObjects) remove(id string) {
	o.mu.Lock()
	defer o.mu.Unlock()
	delete(o.objects, id)
}

func (o *dkObjects) list() []map[string]interface{} {
	o.mu.Lock()
	defer o.mu.Unlock()
	out := make([]map[string]interface{}, 0, len(o.objects))
	for _, v := range o.objects {
		out = append(out, v)
	}
	return out
}

func (o *dkObjects) newID(prefix string) string {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.next++
	return fmt.Sprintf("%s%d", prefix, o.next)
}

// registerGetDelete serves GET and DELETE of single objects under base.
func (o *dkObjects) registerGetDelete(f *dkFake, base string) {
	f.on("GET", base+`/([^/]+)`, func(w http.ResponseWriter, _ *http.Request, _ []byte, m []string) {
		if obj := o.get(m[1]); obj != nil {
			dkWriteJSON(w, http.StatusOK, obj)
			return
		}
		dkWriteJSON(w, http.StatusNotFound, map[string]string{"message": "no such object"})
	})
	f.on("DELETE", base+`/([^/]+)`, func(w http.ResponseWriter, _ *http.Request, _ []byte, m []string) {
		o.remove(m[1])
		w.WriteHeader(http.StatusNoContent)
	})
}

func dkDecode(body []byte) map[string]interface{} {
	var p map[string]interface{}
	_ = json.Unmarshal(body, &p)
	return p
}

// ---------------------------------------------------------------- networks

func dkNetworkFake(f *dkFake) *dkObjects {
	nets := newDkObjects()
	f.on("POST", `/endpoints/1/docker/networks/create`, func(w http.ResponseWriter, _ *http.Request, body []byte, _ []string) {
		p := dkDecode(body)
		id := nets.newID("net")
		ipam, _ := p["IPAM"].(map[string]interface{})
		var cfg []map[string]interface{}
		if list, ok := ipam["Config"].([]interface{}); ok {
			for _, c := range list {
				cm := c.(map[string]interface{})
				cfg = append(cfg, map[string]interface{}{"Subnet": cm["Subnet"], "Gateway": cm["Gateway"]})
			}
		}
		nets.put(id, map[string]interface{}{
			"Id": id, "Name": p["Name"], "Driver": p["Driver"], "Scope": p["Scope"], "Internal": p["Internal"] == true,
			"Attachable": p["Attachable"] == true, "Labels": p["Labels"], "Options": p["Options"],
			"IPAM":      map[string]interface{}{"Driver": ipam["Driver"], "Config": cfg},
			"Portainer": map[string]interface{}{"ResourceControl": map[string]interface{}{"Id": 9}},
		})
		dkWriteJSON(w, http.StatusCreated, map[string]interface{}{"Id": id, "Portainer": map[string]interface{}{"ResourceControl": map[string]interface{}{"Id": 9}}})
	})
	f.on("GET", `/endpoints/1/docker/networks`, func(w http.ResponseWriter, _ *http.Request, _ []byte, _ []string) {
		dkWriteJSON(w, http.StatusOK, nets.list())
	})
	nets.registerGetDelete(f, `/endpoints/1/docker/networks`)
	return nets
}

const dkNetworkConfig = `
resource "portainer_docker_network" "n" {
  endpoint_id = 1
  name        = "backend"
%s
}
`

// TestDockerNetworkResource_Lifecycle: create payload, computed resource
// control, an empty plan, import and destroy.
func TestDockerNetworkResource_Lifecycle(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	nets := dkNetworkFake(f)
	config := f.provider() + fmt.Sprintf(dkNetworkConfig, `
  driver   = "overlay"
  internal = true
  labels   = { team = "a" }
  ipam_config {
    subnet  = "10.1.0.0/24"
    gateway = "10.1.0.1"
  }`)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_docker_network.n", "id", "net1"),
					resource.TestCheckResourceAttr("portainer_docker_network.n", "resource_control_id", "9"),
					resource.TestCheckResourceAttr("portainer_docker_network.n", "scope", "local"),
					resource.TestCheckNoResourceAttr("portainer_docker_network.n", "attachable"),
					func(*terraform.State) error {
						p := dkBody(t, f.last(t, "POST", "/endpoints/1/docker/networks/create"))
						if p["Driver"] != "overlay" || p["Internal"] != true || p["Scope"] != "local" {
							return fmt.Errorf("unexpected payload %v", p)
						}
						if _, ok := p["Attachable"]; ok {
							return fmt.Errorf("unset attachable was sent: %v", p)
						}
						ipam := p["IPAM"].(map[string]interface{})
						if ipam["Driver"] != "default" || fmt.Sprint(ipam["Config"]) != "[map[AuxiliaryAddresses:map[] Gateway:10.1.0.1 Subnet:10.1.0.0/24]]" {
							return fmt.Errorf("unexpected IPAM %v", ipam)
						}
						return nil
					},
				),
			},
			{
				ResourceName:            "portainer_docker_network.n",
				ImportState:             true,
				ImportStateId:           "1:net1",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"ipam_config", "attachable", "ingress", "config_only", "enable_ipv4", "enable_ipv6"},
			},
		},
	})
	if nets.get("net1") != nil {
		t.Error("destroy did not delete the network")
	}
}

// TestDockerNetworkResource_FallbackLookup: a network whose ID is gone is
// found again by name; one gone entirely is planned for creation.
func TestDockerNetworkResource_FallbackLookup(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	nets := dkNetworkFake(f)
	config := f.provider() + fmt.Sprintf(dkNetworkConfig, "")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{Config: config},
			{
				PreConfig: func() {
					obj := nets.get("net1")
					nets.remove("net1")
					obj["Id"] = "net9"
					nets.put("net9", obj)
				},
				Config: config,
				Check:  resource.TestCheckResourceAttr("portainer_docker_network.n", "id", "net9"),
			},
			{
				PreConfig:          func() { nets.remove("net9") },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestDockerNetworkResource_ImportErrors(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	dkNetworkFake(f)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config:        f.provider() + fmt.Sprintf(dkNetworkConfig, ""),
			ResourceName:  "portainer_docker_network.n",
			ImportState:   true,
			ImportStateId: "net1",
			ExpectError:   regexp.MustCompile(`expected <endpoint_id>:<network_id>`),
		}},
	})
}

func TestDockerNetworkResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	dkNetworkFake(f)
	config := f.provider() + fmt.Sprintf(dkNetworkConfig, "") + `
resource "portainer_docker_network" "full" {
  endpoint_id = 1
  name        = "full"
  driver      = "bridge"
  internal    = true
  options     = { "com.docker.network.bridge.name" = "br0" }
  labels      = { team = "a" }
  ipam_config {
    subnet = "10.2.0.0/24"
  }
}`
	dkUpgradeTest(t, config)
}

// dkUpgradeTest applies config with v3.0.0 (SDKv2), then plans it with this
// build: the plan has to be empty.
func dkUpgradeTest(t *testing.T, config string) {
	t.Helper()
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

// ----------------------------------------------------------------- volumes

func dkVolumeFake(f *dkFake) *dkObjects {
	vols := newDkObjects()
	f.on("POST", `/endpoints/1/docker/volumes/create`, func(w http.ResponseWriter, _ *http.Request, body []byte, _ []string) {
		p := dkDecode(body)
		driver, _ := p["Driver"].(string)
		if driver == "" {
			driver = "local"
		}
		obj := map[string]interface{}{"Name": p["Name"], "Driver": driver, "Labels": p["Labels"], "Options": p["DriverOpts"]}
		vols.put(p["Name"].(string), obj)
		dkWriteJSON(w, http.StatusCreated, obj)
	})
	vols.registerGetDelete(f, `/endpoints/1/docker/volumes`)
	return vols
}

func TestDockerVolumeResource_Lifecycle(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	vols := dkVolumeFake(f)
	config := f.provider() + `
resource "portainer_docker_volume" "v" {
  endpoint_id = 1
  name        = "data"
  labels      = { app = "db" }
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_docker_volume.v", "id", "1:data"),
					resource.TestCheckResourceAttr("portainer_docker_volume.v", "driver", "local"),
					resource.TestCheckNoResourceAttr("portainer_docker_volume.v", "driver_opts"),
					func(*terraform.State) error {
						p := dkBody(t, f.last(t, "POST", "/endpoints/1/docker/volumes/create"))
						if _, ok := p["Driver"]; ok {
							return fmt.Errorf("an unset driver was sent: %v", p)
						}
						return nil
					},
				),
			},
			{ResourceName: "portainer_docker_volume.v", ImportState: true, ImportStateId: "1-data", ImportStateVerify: true},
			{
				PreConfig:          func() { vols.remove("data") },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestDockerVolumeResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	dkVolumeFake(f)
	dkUpgradeTest(t, f.provider()+`
resource "portainer_docker_volume" "minimal" {
  endpoint_id = 1
  name        = "minimal"
}
resource "portainer_docker_volume" "full" {
  endpoint_id = 1
  name        = "full"
  driver      = "local"
  driver_opts = { type = "tmpfs" }
  labels      = { app = "db" }
}`)
}

// ----------------------------------------------------------------- secrets

func dkSwarmObjectFake(f *dkFake, kind string) *dkObjects {
	objs := newDkObjects()
	base := "/endpoints/1/docker/" + kind
	f.on("GET", base, func(w http.ResponseWriter, _ *http.Request, _ []byte, _ []string) {
		dkWriteJSON(w, http.StatusOK, objs.list())
	})
	f.on("POST", base+"/create", func(w http.ResponseWriter, _ *http.Request, body []byte, _ []string) {
		p := dkDecode(body)
		id := objs.newID(kind[:3])
		spec := map[string]interface{}{"Name": p["Name"], "Labels": p["Labels"]}
		if v, ok := p["Driver"]; ok {
			spec["Driver"] = v
		}
		if v, ok := p["Templating"]; ok {
			spec["Templating"] = v
		}
		objs.put(id, map[string]interface{}{"ID": id, "Spec": spec, "Version": map[string]interface{}{"Index": 11}})
		dkWriteJSON(w, http.StatusCreated, map[string]interface{}{"ID": id, "Portainer": map[string]interface{}{"ResourceControl": map[string]interface{}{"Id": 5}}})
	})
	f.on("POST", base+`/([^/]+)/update`, func(w http.ResponseWriter, _ *http.Request, body []byte, m []string) {
		obj := objs.get(m[1])
		if obj == nil {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		p := dkDecode(body)
		spec := obj["Spec"].(map[string]interface{})
		spec["Labels"] = p["Labels"]
		if n, ok := p["Name"]; ok {
			spec["Name"] = n
		}
		w.WriteHeader(http.StatusOK)
	})
	objs.registerGetDelete(f, base)
	return objs
}

func TestDockerSecretResource_Lifecycle(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	secrets := dkSwarmObjectFake(f, "secrets")
	config := func(labels string) string {
		return f.provider() + fmt.Sprintf(`
resource "portainer_docker_secret" "s" {
  endpoint_id = 1
  name        = "token"
  data        = "c2VjcmV0"
  labels      = %s
}`, labels)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config(`{ a = "1" }`),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_docker_secret.s", "id", "sec1"),
					resource.TestCheckResourceAttr("portainer_docker_secret.s", "resource_control_id", "5"),
					func(*terraform.State) error {
						if p := dkBody(t, f.last(t, "POST", "/endpoints/1/docker/secrets/create")); p["Data"] != "c2VjcmV0" || p["Name"] != "token" {
							return fmt.Errorf("unexpected create payload %v", p)
						}
						return nil
					},
				),
			},
			{
				Config: config(`{ a = "2" }`),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_docker_secret.s", plancheck.ResourceActionUpdate)},
				},
				Check: func(*terraform.State) error {
					upd := f.last(t, "POST", "/endpoints/1/docker/secrets/sec1/update")
					if upd.Query != "version=11" || fmt.Sprint(dkBody(t, upd)["Labels"]) != "map[a:2]" {
						return fmt.Errorf("unexpected update %s %s", upd.Query, upd.Body)
					}
					return nil
				},
			},
			{
				ResourceName:            "portainer_docker_secret.s",
				ImportState:             true,
				ImportStateId:           "1:sec1",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"data"},
			},
		},
	})
	if secrets.get("sec1") != nil {
		t.Error("destroy did not delete the secret")
	}
}

// TestDockerSecretResource_WriteOnlyData: data_wo is sent at create and
// never stored.
func TestDockerSecretResource_WriteOnlyData(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	dkSwarmObjectFake(f, "secrets")
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: f.provider() + `
resource "portainer_docker_secret" "s" {
  endpoint_id     = 1
  name            = "token"
  data_wo         = "d3Jp"
  data_wo_version = 1
}`,
			Check: resource.ComposeTestCheckFunc(
				resource.TestCheckNoResourceAttr("portainer_docker_secret.s", "data_wo"),
				resource.TestCheckNoResourceAttr("portainer_docker_secret.s", "data"),
				func(*terraform.State) error {
					if p := dkBody(t, f.last(t, "POST", "/endpoints/1/docker/secrets/create")); p["Data"] != "d3Jp" {
						return fmt.Errorf("data_wo not sent: %v", p)
					}
					return nil
				},
			),
		}},
	})
}

func TestDockerSecretResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	dkSwarmObjectFake(f, "secrets")
	dkUpgradeTest(t, f.provider()+`
resource "portainer_docker_secret" "minimal" {
  endpoint_id = 1
  name        = "minimal"
  data        = "c2VjcmV0"
}
resource "portainer_docker_secret" "labels" {
  endpoint_id = 1
  name        = "labels"
  data        = "c2VjcmV0"
  labels      = { a = "1" }
}`)
}

// ----------------------------------------------------------------- configs

func TestDockerConfigResource_Lifecycle(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	configs := dkSwarmObjectFake(f, "configs")
	config := func(name string) string {
		return f.provider() + fmt.Sprintf(`
resource "portainer_docker_config" "c" {
  endpoint_id = 1
  name        = %q
  data        = "Y29uZmln"
}`, name)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config("app"),
				Check:  resource.TestCheckResourceAttr("portainer_docker_config.c", "id", "con1"),
			},
			{
				Config: config("app2"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectResourceAction("portainer_docker_config.c", plancheck.ResourceActionUpdate)},
				},
				Check: func(*terraform.State) error {
					p := dkBody(t, f.last(t, "POST", "/endpoints/1/docker/configs/con1/update"))
					if p["Name"] != "app2" || p["Data"] != "Y29uZmln" {
						return fmt.Errorf("unexpected update %v", p)
					}
					return nil
				},
			},
			{
				ResourceName:            "portainer_docker_config.c",
				ImportState:             true,
				ImportStateId:           "1:con1",
				ImportStateVerify:       true,
				ImportStateVerifyIgnore: []string{"data"},
			},
		},
	})
	if configs.get("con1") != nil {
		t.Error("destroy did not delete the config")
	}
}

func TestDockerConfigResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	dkSwarmObjectFake(f, "configs")
	dkUpgradeTest(t, f.provider()+`
resource "portainer_docker_config" "c" {
  endpoint_id = 1
  name        = "app"
  data        = "Y29uZmln"
}`)
}

// ------------------------------------------------------------------ images

func dkImageFake(f *dkFake) *dkObjects {
	images := newDkObjects()
	f.on("POST", `/endpoints/1/docker/images/create`, func(w http.ResponseWriter, r *http.Request, _ []byte, _ []string) {
		images.put(r.URL.Query().Get("fromImage"), map[string]interface{}{"Id": "sha256:1"})
		_, _ = w.Write([]byte(`{"status":"Pulled"}`))
	})
	f.on("GET", `/endpoints/1/docker/images/(.+)/json`, func(w http.ResponseWriter, _ *http.Request, _ []byte, m []string) {
		if obj := images.get(m[1]); obj != nil {
			dkWriteJSON(w, http.StatusOK, obj)
			return
		}
		dkWriteJSON(w, http.StatusNotFound, map[string]string{"message": "no such image"})
	})
	f.on("DELETE", `/endpoints/1/docker/images/(.+)`, func(w http.ResponseWriter, _ *http.Request, _ []byte, m []string) {
		images.remove(m[1])
		dkWriteJSON(w, http.StatusOK, []interface{}{})
	})
	return images
}

func TestDockerImageResource_Lifecycle(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	images := dkImageFake(f)
	config := f.provider() + `
resource "portainer_docker_image" "i" {
  endpoint_id              = 1
  image                    = "registry.example.com/app:1"
  registry_auth_wo         = "bot:pw"
  registry_auth_wo_version = 1
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_docker_image.i", "id", "1-registry.example.com/app:1"),
					resource.TestCheckNoResourceAttr("portainer_docker_image.i", "registry_auth"),
					func(*terraform.State) error {
						req := f.last(t, "POST", "/endpoints/1/docker/images/create")
						raw, _ := base64.StdEncoding.DecodeString(req.Headers.Get("X-Registry-Auth"))
						var auth dockerImageAuth
						_ = json.Unmarshal(raw, &auth)
						if auth.Username != "bot" || auth.Password != "pw" || auth.ServerAddress != "registry.example.com" {
							return fmt.Errorf("unexpected registry auth %s", raw)
						}
						return nil
					},
				),
			},
			{
				PreConfig:          func() { images.remove("registry.example.com/app:1") },
				Config:             config,
				PlanOnly:           true,
				ExpectNonEmptyPlan: true,
			},
		},
	})
}

func TestDockerImageResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	dkImageFake(f)
	dkUpgradeTest(t, f.provider()+`
resource "portainer_docker_image" "i" {
  endpoint_id = 1
  image       = "nginx:latest"
}
resource "portainer_docker_image" "auth" {
  endpoint_id   = 1
  image         = "registry.example.com/app:1"
  registry_auth = "bot:pw"
}`)
}

// ------------------------------------------------------------------- nodes

func dkNodeFake(f *dkFake) *dkObjects {
	nodes := newDkObjects()
	nodes.put("n1", map[string]interface{}{"ID": "n1", "Version": map[string]interface{}{"Index": 7}, "Spec": map[string]interface{}{"Role": "worker", "Availability": "active"}})
	f.on("POST", `/endpoints/1/docker/nodes/n1/update`, func(w http.ResponseWriter, _ *http.Request, body []byte, _ []string) {
		p := dkDecode(body)
		spec := map[string]interface{}{"Role": "worker", "Availability": "active"}
		for k, v := range p {
			spec[k] = v
		}
		nodes.put("n1", map[string]interface{}{"ID": "n1", "Version": map[string]interface{}{"Index": 7}, "Spec": spec})
		w.WriteHeader(http.StatusOK)
	})
	nodes.registerGetDelete(f, `/endpoints/1/docker/nodes`)
	return nodes
}

func TestDockerNodeResource_Lifecycle(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	dkNodeFake(f)
	config := func(availability string) string {
		return f.provider() + fmt.Sprintf(`
resource "portainer_docker_node" "n" {
  endpoint_id  = 1
  node_id      = "n1"
  version      = 7
  role         = "worker"
  availability = %q
  labels       = { zone = "a" }
}`, availability)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config("active"),
				Check:  resource.TestCheckResourceAttr("portainer_docker_node.n", "id", "1:n1"),
			},
			{
				Config: config("drain"),
				Check: func(*terraform.State) error {
					req := f.last(t, "POST", "/endpoints/1/docker/nodes/n1/update")
					if req.Query != "version=7" || dkBody(t, req)["Availability"] != "drain" {
						return fmt.Errorf("unexpected update %s %s", req.Query, req.Body)
					}
					return nil
				},
			},
			{ResourceName: "portainer_docker_node.n", ImportState: true, ImportStateId: "1-n1", ImportStateVerify: true},
		},
	})
}

func TestDockerNodeResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	dkNodeFake(f)
	dkUpgradeTest(t, f.provider()+`
resource "portainer_docker_node" "n" {
  endpoint_id  = 1
  node_id      = "n1"
  version      = 7
  role         = "worker"
  availability = "active"
}`)
}

// ----------------------------------------------------------------- plugins

func dkPluginFake(f *dkFake) *dkObjects {
	plugins := newDkObjects()
	f.on("POST", `/endpoints/1/docker/plugins/pull`, func(w http.ResponseWriter, r *http.Request, _ []byte, _ []string) {
		name := r.URL.Query().Get("name")
		if name == "" {
			name = r.URL.Query().Get("remote")
		}
		plugins.put(name, map[string]interface{}{"Enabled": false, "PluginReference": "docker.io/" + r.URL.Query().Get("remote") + ":latest"})
		_, _ = w.Write([]byte(`{"status":"Downloading"}` + "\n" + `{"status":"Complete"}`))
	})
	f.on("POST", `/endpoints/1/docker/plugins/(.+)/enable`, func(w http.ResponseWriter, _ *http.Request, _ []byte, m []string) {
		if obj := plugins.get(m[1]); obj != nil {
			obj["Enabled"] = true
		}
		w.WriteHeader(http.StatusOK)
	})
	f.on("GET", `/endpoints/1/docker/plugins/(.+)/json`, func(w http.ResponseWriter, _ *http.Request, _ []byte, m []string) {
		if obj := plugins.get(m[1]); obj != nil {
			dkWriteJSON(w, http.StatusOK, obj)
			return
		}
		dkWriteJSON(w, http.StatusNotFound, map[string]string{"message": "plugin not found"})
	})
	f.json("GET", `/endpoints/1/docker/plugins/privileges`, http.StatusOK, []map[string]interface{}{{"Name": "network", "Value": []string{"host"}}})
	f.on("DELETE", `/endpoints/1/docker/plugins/(.+)`, func(w http.ResponseWriter, _ *http.Request, _ []byte, m []string) {
		plugins.remove(m[1])
		w.WriteHeader(http.StatusOK)
	})
	return plugins
}

func TestDockerPluginResource_Lifecycle(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	plugins := dkPluginFake(f)
	config := func(description string) string {
		return f.provider() + fmt.Sprintf(`
resource "portainer_docker_plugin" "p" {
  endpoint_id = 1
  remote      = "rclone/docker-volume-rclone"
  enable      = true
  settings {
    name        = "network"
    value       = ["host"]
    description = %q
  }
}`, description)
	}
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{
			{
				Config: config("access to the host network"),
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("portainer_docker_plugin.p", "id", "rclone/docker-volume-rclone"),
					resource.TestCheckResourceAttr("portainer_docker_plugin.p", "name", "rclone/docker-volume-rclone"),
					resource.TestCheckResourceAttr("portainer_docker_plugin.p", "enable", "true"),
					func(*terraform.State) error {
						req := f.last(t, "POST", "/endpoints/1/docker/plugins/pull")
						if req.Headers.Get("X-Registry-Auth") != "e30=" || !strings.Contains(string(req.Body), `"Description":"access to the host network"`) {
							return fmt.Errorf("unexpected pull %s %s", req.Headers.Get("X-Registry-Auth"), req.Body)
						}
						if q := f.last(t, "POST", "/endpoints/1/docker/plugins/rclone/docker-volume-rclone/enable").Query; q != "timeout=0" {
							return fmt.Errorf("enable without timeout: %q", q)
						}
						return nil
					},
				),
			},
			{
				// A new description does not replace the plugin.
				Config: config("host networking"),
				ConfigPlanChecks: resource.ConfigPlanChecks{
					PreApply: []plancheck.PlanCheck{plancheck.ExpectEmptyPlan()},
				},
			},
		},
	})
	if plugins.get("rclone/docker-volume-rclone") != nil {
		t.Error("destroy did not remove the plugin")
	}
}

func TestDockerPluginResource_Import(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	plugins := dkPluginFake(f)
	plugins.put("rclone", map[string]interface{}{"Enabled": true, "PluginReference": "docker.io/rclone/docker-volume-rclone:latest"})
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps: []resource.TestStep{{
			Config: f.provider() + `
resource "portainer_docker_plugin" "p" {
  endpoint_id = 1
  remote      = "rclone/docker-volume-rclone"
  name        = "rclone"
  enable      = true
  settings {
    name  = "network"
    value = ["host"]
  }
}`,
			ResourceName:  "portainer_docker_plugin.p",
			ImportState:   true,
			ImportStateId: "1:rclone",
			ImportStateCheck: func(states []*terraform.InstanceState) error {
				attrs := states[0].Attributes
				if attrs["remote"] != "rclone/docker-volume-rclone" || attrs["settings.0.name"] != "network" || attrs["registry_auth"] != "e30=" {
					return fmt.Errorf("unexpected imported state %v", attrs)
				}
				return nil
			},
		}},
	})
}

func TestDockerPluginResource_UpgradeFromV300(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	dkPluginFake(f)
	dkUpgradeTest(t, f.provider()+`
resource "portainer_docker_plugin" "p" {
  endpoint_id = 1
  remote      = "rclone/docker-volume-rclone"
  settings {
    name  = "network"
    value = ["host"]
  }
}`)
}

func TestCheckDockerStream(t *testing.T) {
	t.Parallel()
	if err := checkDockerStream(strings.NewReader(`{"status":"ok"}` + "\n" + `{"error":"denied"}`)); err == nil || err.Error() != "denied" {
		t.Errorf("expected the streamed error, got %v", err)
	}
	if err := checkDockerStream(strings.NewReader(`{"status":"ok"}`)); err != nil {
		t.Errorf("unexpected error %v", err)
	}
	if got := normalizePluginReference("docker.io/library/foo:latest"); got != "foo" {
		t.Errorf("normalizePluginReference = %q", got)
	}
}
