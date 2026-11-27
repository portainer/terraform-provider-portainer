package internal

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/knownvalue"
	"github.com/hashicorp/terraform-plugin-testing/querycheck"
	"github.com/hashicorp/terraform-plugin-testing/querycheck/queryfilter"
	"github.com/hashicorp/terraform-plugin-testing/statecheck"
	"github.com/hashicorp/terraform-plugin-testing/tfjsonpath"
	"github.com/hashicorp/terraform-plugin-testing/tfversion"
)

// dkIdentityCase is a bag-backed resource with a composite identity and its
// list resource, run against the dkFake stand-ins.
type dkIdentityCase struct {
	typ   string
	setup func(f *dkFake)
	// config is the resource block, named "r".
	config string
	// identity is checked on the created object and on the filtered result.
	identity map[string]knownvalue.Check
	// display is the display name of the created object in a query.
	display string
	// values are checked on the listed resource (include_resource).
	values map[string]knownvalue.Check
	// all is the list configuration listing everything ("" for none),
	// filter one that matches the object, miss one that matches nothing.
	all, filter, miss string
	// importChanges: the configuration sets values Portainer never returns.
	importChanges bool
}

func dkListConfig(typ, name, config string, includeResource bool) string {
	block := ""
	if config != "" {
		block = fmt.Sprintf("\n  config {\n    %s\n  }\n", config)
	}
	return fmt.Sprintf("\nlist %q %q {\n  provider         = portainer\n  include_resource = %v\n%s}\n", typ, name, includeResource, block)
}

// testDkIdentity: the identity is stored on create and the resource imports
// by identity.
func testDkIdentity(t *testing.T, c dkIdentityCase) {
	t.Helper()
	f := newDkFake(t)
	c.setup(f)
	addr := c.typ + ".r"
	config := f.provider() + c.config
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_12_0), skipOnOpenTofuIdentity},
		Steps: []resource.TestStep{
			{
				Config:            config,
				ConfigStateChecks: []statecheck.StateCheck{statecheck.ExpectIdentity(addr, c.identity)},
			},
			{
				Config:             config,
				ResourceName:       addr,
				ImportState:        true,
				ImportStateKind:    resource.ImportBlockWithResourceIdentity,
				ExpectNonEmptyPlan: c.importChanges,
			},
		},
	})
}

// testDkList: `terraform query` lists the created object with its identity
// and, with include_resource, the state an import by identity reads; the
// filters narrow the list.
func testDkList(t *testing.T, c dkIdentityCase) {
	t.Helper()
	f := newDkFake(t)
	c.setup(f)
	byName := queryfilter.ByDisplayName(knownvalue.StringExact(c.display))
	values := make([]querycheck.KnownValueCheck, 0, len(c.values))
	for k, v := range c.values {
		values = append(values, querycheck.KnownValueCheck{Path: tfjsonpath.New(k), KnownValue: v})
	}
	query := dkListConfig(c.typ, "all", c.all, true) + dkListConfig(c.typ, "match", c.filter, false) + dkListConfig(c.typ, "miss", c.miss, false)
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_14_0), skipOnOpenTofuList},
		Steps: []resource.TestStep{
			{Config: f.provider() + c.config},
			{
				Query:  true,
				Config: query,
				QueryResultChecks: []querycheck.QueryResultCheck{
					querycheck.ExpectLengthAtLeast(c.typ+".all", 1),
					querycheck.ExpectResourceDisplayName(c.typ+".all", byName, knownvalue.StringExact(c.display)),
					querycheck.ExpectResourceKnownValues(c.typ+".all", byName, values),
					querycheck.ExpectLength(c.typ+".match", 1),
					querycheck.ExpectIdentity(c.typ+".match", c.identity),
					querycheck.ExpectLength(c.typ+".miss", 0),
				},
			},
		},
	})
}

func dkEndpointIdentityCheck(key string, value string) map[string]knownvalue.Check {
	return map[string]knownvalue.Check{"endpoint_id": knownvalue.Int64Exact(1), key: knownvalue.StringExact(value)}
}

var dkNetworkIdentityCase = dkIdentityCase{
	typ:   "portainer_docker_network",
	setup: func(f *dkFake) { dkNetworkFake(f) },
	config: `resource "portainer_docker_network" "r" {
  endpoint_id = 1
  name        = "backend"
  labels      = { team = "a" }
}`,
	identity: dkEndpointIdentityCheck("id", "net1"),
	display:  "backend",
	values: map[string]knownvalue.Check{
		"id": knownvalue.StringExact("net1"), "endpoint_id": knownvalue.Int64Exact(1),
		"name": knownvalue.StringExact("backend"), "labels": knownvalue.MapExact(map[string]knownvalue.Check{"team": knownvalue.StringExact("a")}),
	},
	all: `endpoint_id = 1`,
	filter: `endpoint_id = 1
    name        = "backend"`,
	miss: `endpoint_id = 1
    name        = "frontend"`,
}

func TestDockerNetworkResource_Identity(t *testing.T) {
	t.Parallel()
	testDkIdentity(t, dkNetworkIdentityCase)
}

func TestDockerNetworkResource_List(t *testing.T) {
	t.Parallel()
	testDkList(t, dkNetworkIdentityCase)
}

// dkVolumeListFake is dkVolumeFake with Docker's volume listing.
func dkVolumeListFake(f *dkFake) {
	vols := dkVolumeFake(f)
	f.on("GET", `/endpoints/1/docker/volumes`, func(w http.ResponseWriter, _ *http.Request, _ []byte, _ []string) {
		dkWriteJSON(w, http.StatusOK, map[string]interface{}{"Volumes": vols.list()})
	})
}

var dkVolumeIdentityCase = dkIdentityCase{
	typ:   "portainer_docker_volume",
	setup: dkVolumeListFake,
	config: `resource "portainer_docker_volume" "r" {
  endpoint_id = 1
  name        = "data"
  labels      = { app = "db" }
}`,
	identity: dkEndpointIdentityCheck("name", "data"),
	display:  "data",
	values: map[string]knownvalue.Check{
		"id": knownvalue.StringExact("1:data"), "name": knownvalue.StringExact("data"), "driver": knownvalue.StringExact("local"),
	},
	all: `endpoint_id = 1`,
	filter: `endpoint_id = 1
    name        = "data"`,
	miss: `endpoint_id = 1
    name        = "logs"`,
}

func TestDockerVolumeResource_Identity(t *testing.T) {
	t.Parallel()
	testDkIdentity(t, dkVolumeIdentityCase)
}

func TestDockerVolumeResource_List(t *testing.T) {
	t.Parallel()
	testDkList(t, dkVolumeIdentityCase)
}

func dkSwarmIdentityCase(kind, typ, id string) dkIdentityCase {
	return dkIdentityCase{
		typ:   typ,
		setup: func(f *dkFake) { dkSwarmObjectFake(f, kind) },
		config: fmt.Sprintf(`resource %q "r" {
  endpoint_id = 1
  name        = "app"
  data        = "c2VjcmV0"
  labels      = { a = "1" }
}`, typ),
		identity: dkEndpointIdentityCheck("id", id),
		display:  "app",
		values: map[string]knownvalue.Check{
			"id": knownvalue.StringExact(id), "name": knownvalue.StringExact("app"), "data": knownvalue.Null(),
			"labels": knownvalue.MapExact(map[string]knownvalue.Check{"a": knownvalue.StringExact("1")}),
		},
		all: `endpoint_id = 1`,
		filter: `endpoint_id = 1
    name        = "app"`,
		miss: `endpoint_id = 1
    name        = "other"`,
		// Docker never returns the data, as with an import by ID.
		importChanges: true,
	}
}

func TestDockerSecretResource_Identity(t *testing.T) {
	t.Parallel()
	testDkIdentity(t, dkSwarmIdentityCase("secrets", "portainer_docker_secret", "sec1"))
}

func TestDockerSecretResource_List(t *testing.T) {
	t.Parallel()
	testDkList(t, dkSwarmIdentityCase("secrets", "portainer_docker_secret", "sec1"))
}

func TestDockerConfigResource_Identity(t *testing.T) {
	t.Parallel()
	testDkIdentity(t, dkSwarmIdentityCase("configs", "portainer_docker_config", "con1"))
}

func TestDockerConfigResource_List(t *testing.T) {
	t.Parallel()
	testDkList(t, dkSwarmIdentityCase("configs", "portainer_docker_config", "con1"))
}

// dkPluginListFake is dkPluginFake with Docker's plugin listing, which names
// a plugin with its tag.
func dkPluginListFake(f *dkFake) {
	plugins := dkPluginFake(f)
	f.on("GET", `/endpoints/1/docker/plugins`, func(w http.ResponseWriter, _ *http.Request, _ []byte, _ []string) {
		plugins.mu.Lock()
		defer plugins.mu.Unlock()
		out := make([]map[string]interface{}, 0, len(plugins.objects))
		for name, p := range plugins.objects {
			out = append(out, map[string]interface{}{"Name": name + ":latest", "Enabled": p["Enabled"]})
		}
		dkWriteJSON(w, http.StatusOK, out)
	})
}

var dkPluginIdentityCase = dkIdentityCase{
	typ:   "portainer_docker_plugin",
	setup: dkPluginListFake,
	config: `resource "portainer_docker_plugin" "r" {
  endpoint_id = 1
  remote      = "rclone/docker-volume-rclone"
  enable      = true
  settings {
    name  = "network"
    value = ["host"]
  }
}`,
	identity: dkEndpointIdentityCheck("name", "rclone/docker-volume-rclone"),
	display:  "rclone/docker-volume-rclone",
	values: map[string]knownvalue.Check{
		"id": knownvalue.StringExact("rclone/docker-volume-rclone"), "remote": knownvalue.StringExact("rclone/docker-volume-rclone"),
		"enable": knownvalue.Bool(true),
	},
	all: `endpoint_id = 1`,
	filter: `endpoint_id = 1
    name        = "rclone/docker-volume-rclone"`,
	miss: `endpoint_id = 1
    name        = "rclone"`,
}

func TestDockerPluginResource_Identity(t *testing.T) {
	t.Parallel()
	testDkIdentity(t, dkPluginIdentityCase)
}

func TestDockerPluginResource_List(t *testing.T) {
	t.Parallel()
	testDkList(t, dkPluginIdentityCase)
}

// dkNodeListFake is dkNodeFake with the Swarm node listing.
func dkNodeListFake(f *dkFake) {
	nodes := dkNodeFake(f)
	f.on("GET", `/endpoints/1/docker/nodes`, func(w http.ResponseWriter, _ *http.Request, _ []byte, _ []string) {
		list := nodes.list()
		out := make([]map[string]interface{}, 0, len(list))
		for _, n := range list {
			out = append(out, map[string]interface{}{"ID": n["ID"], "Description": map[string]interface{}{"Hostname": "worker-1"}})
		}
		dkWriteJSON(w, http.StatusOK, out)
	})
}

var dkNodeIdentityCase = dkIdentityCase{
	typ:   "portainer_docker_node",
	setup: dkNodeListFake,
	config: `resource "portainer_docker_node" "r" {
  endpoint_id  = 1
  node_id      = "n1"
  version      = 7
  role         = "worker"
  availability = "active"
  labels       = { zone = "a" }
}`,
	identity: dkEndpointIdentityCheck("node_id", "n1"),
	display:  "worker-1",
	values: map[string]knownvalue.Check{
		"id": knownvalue.StringExact("1:n1"), "node_id": knownvalue.StringExact("n1"), "version": knownvalue.Int64Exact(7),
		"role": knownvalue.StringExact("worker"),
	},
	all: `endpoint_id = 1`,
	filter: `endpoint_id = 1
    name        = "worker-1"`,
	miss: `endpoint_id = 1
    name        = "worker-2"`,
}

func TestDockerNodeResource_Identity(t *testing.T) {
	t.Parallel()
	testDkIdentity(t, dkNodeIdentityCase)
}

func TestDockerNodeResource_List(t *testing.T) {
	t.Parallel()
	testDkList(t, dkNodeIdentityCase)
}

// A stack created from a string imports by identity with method "string"
// and the content Portainer stores: the plan after the import is empty.
var dkStackIdentityCase = dkIdentityCase{
	typ:    "portainer_stack",
	setup:  func(f *dkFake) { newFakeStacks(f) },
	config: strings.Replace(fmt.Sprintf(stackStringConfig, "services: {}", true, ""), `"s"`, `"r"`, 1),
	identity: map[string]knownvalue.Check{
		"endpoint_id": knownvalue.Int64Exact(1), "id": knownvalue.StringExact("1"), "deployment_type": knownvalue.StringExact("standalone"),
	},
	display: "web",
	values: map[string]knownvalue.Check{
		"id": knownvalue.StringExact("1"), "name": knownvalue.StringExact("web"), "method": knownvalue.StringExact("string"),
		"deployment_type": knownvalue.StringExact("standalone"), "stack_file_content": knownvalue.StringExact("services: {}"),
	},
	filter: `endpoint_id     = 1
    name            = "web"
    deployment_type = "standalone"`,
	miss: `deployment_type = "swarm"`,
}

func TestStackResource_Identity(t *testing.T) {
	t.Parallel()
	testDkIdentity(t, dkStackIdentityCase)
}

func TestStackResource_List(t *testing.T) {
	t.Parallel()
	testDkList(t, dkStackIdentityCase)
}

// TestStackResource_IdentityRepository: a stack deployed from Git imports by
// identity with method "repository".
func TestStackResource_IdentityRepository(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	newFakeStacks(f)
	config := f.provider() + `
resource "portainer_stack" "r" {
  name                    = "git"
  deployment_type         = "swarm"
  method                  = "repository"
  endpoint_id             = 1
  repository_url          = "https://github.com/example/app"
  file_path_in_repository = "docker-compose.yml"
}`
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		TerraformVersionChecks:   []tfversion.TerraformVersionCheck{tfversion.SkipBelow(tfversion.Version1_12_0), skipOnOpenTofuIdentity},
		Steps: []resource.TestStep{
			{
				Config: config,
				ConfigStateChecks: []statecheck.StateCheck{statecheck.ExpectIdentity("portainer_stack.r", map[string]knownvalue.Check{
					"endpoint_id": knownvalue.Int64Exact(1), "id": knownvalue.StringExact("1"), "deployment_type": knownvalue.StringExact("swarm"),
				})},
			},
			{
				Config:          config,
				ResourceName:    "portainer_stack.r",
				ImportState:     true,
				ImportStateKind: resource.ImportBlockWithResourceIdentity,
			},
		},
	})
}
