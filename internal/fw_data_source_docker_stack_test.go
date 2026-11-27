package internal

import (
	"fmt"
	"net/http"
	"regexp"
	"testing"

	"github.com/hashicorp/terraform-plugin-testing/helper/resource"
	"github.com/hashicorp/terraform-plugin-testing/terraform"
)

// Tests of the Docker and stack data sources.

func dkDataSourceTest(t *testing.T, f *dkFake, config string, check resource.TestCheckFunc) {
	t.Helper()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps:                    []resource.TestStep{{Config: f.provider() + config, Check: check}},
	})
}

func dkDataSourceError(t *testing.T, f *dkFake, config, want string) {
	t.Helper()
	resource.UnitTest(t, resource.TestCase{
		ProtoV5ProviderFactories: testProtoV5Factories,
		Steps:                    []resource.TestStep{{Config: f.provider() + config, ExpectError: regexp.MustCompile(want)}},
	})
}

func TestStackDataSource(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	f.json("GET", `/stacks`, http.StatusOK, []map[string]interface{}{
		{"Id": 1, "Name": "web", "EndpointId": 2, "Type": 2},
		{"Id": 4, "Name": "web", "EndpointId": 1, "Type": 1, "SwarmId": "sw1"},
	})
	dkDataSourceTest(t, f, `data "portainer_stack" "s" {
  name        = "web"
  endpoint_id = 1
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_stack.s", "id", "4"),
		resource.TestCheckResourceAttr("data.portainer_stack.s", "type", "1"),
		resource.TestCheckResourceAttr("data.portainer_stack.s", "swarm_id", "sw1"),
	))
	dkDataSourceError(t, f, `data "portainer_stack" "s" {
  name        = "api"
  endpoint_id = 1
}`, `stack api not found in endpoint 1`)
}

func TestDockerLookupDataSources(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	f.json("GET", `/endpoints/1/docker/networks`, http.StatusOK, []map[string]string{{"Id": "n1", "Name": "backend", "Driver": "overlay", "Scope": "swarm"}})
	f.json("GET", `/endpoints/1/docker/volumes`, http.StatusOK, map[string]interface{}{"Volumes": []map[string]string{{"Name": "data", "Driver": "local", "Mountpoint": "/var/lib/docker/volumes/data"}}})
	f.json("GET", `/endpoints/1/docker/configs`, http.StatusOK, []map[string]interface{}{{"ID": "c1", "Spec": map[string]string{"Name": "app"}}})
	f.json("GET", `/endpoints/1/docker/secrets`, http.StatusOK, []map[string]interface{}{{"ID": "s1", "Spec": map[string]string{"Name": "token"}}})
	f.json("GET", `/endpoints/1/docker/images/json`, http.StatusOK, []map[string]interface{}{{"Id": "sha256:1", "RepoTags": []string{"nginx:latest"}}})
	f.json("GET", `/endpoints/1/docker/nodes`, http.StatusOK, []map[string]interface{}{{"ID": "node1", "Description": map[string]string{"Hostname": "host-a"}, "Spec": map[string]string{"Role": "manager"}, "Status": map[string]string{"State": "ready"}}})
	dkDataSourceTest(t, f, `
data "portainer_docker_network" "n" {
  endpoint_id = 1
  name        = "backend"
}
data "portainer_docker_volume" "v" {
  endpoint_id = 1
  name        = "data"
}
data "portainer_docker_config" "c" {
  endpoint_id = 1
  name        = "app"
}
data "portainer_docker_secret" "s" {
  endpoint_id = 1
  name        = "token"
}
data "portainer_docker_image" "i" {
  endpoint_id = 1
  name        = "nginx"
}
data "portainer_docker_node" "node" {
  endpoint_id = 1
  hostname    = "host-a"
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_docker_network.n", "id", "n1"),
		resource.TestCheckResourceAttr("data.portainer_docker_network.n", "driver", "overlay"),
		resource.TestCheckResourceAttr("data.portainer_docker_network.n", "scope", "swarm"),
		resource.TestCheckResourceAttr("data.portainer_docker_volume.v", "id", "data"),
		resource.TestCheckResourceAttr("data.portainer_docker_volume.v", "mount_point", "/var/lib/docker/volumes/data"),
		resource.TestCheckResourceAttr("data.portainer_docker_config.c", "id", "c1"),
		resource.TestCheckResourceAttr("data.portainer_docker_secret.s", "id", "s1"),
		resource.TestCheckResourceAttr("data.portainer_docker_image.i", "id", "sha256:1"),
		resource.TestCheckResourceAttr("data.portainer_docker_node.node", "id", "node1"),
		resource.TestCheckResourceAttr("data.portainer_docker_node.node", "role", "manager"),
		resource.TestCheckResourceAttr("data.portainer_docker_node.node", "status", "ready"),
	))
	for config, want := range map[string]string{
		`data "portainer_docker_network" "n" {
  endpoint_id = 1
  name        = "x"
}`: `docker network x not found in endpoint 1`,
		`data "portainer_docker_volume" "v" {
  endpoint_id = 1
  name        = "x"
}`: `docker volume x not found in endpoint 1`,
		`data "portainer_docker_config" "c" {
  endpoint_id = 1
  name        = "x"
}`: `docker config x not found in endpoint 1`,
		`data "portainer_docker_secret" "s" {
  endpoint_id = 1
  name        = "x"
}`: `docker secret x not found in endpoint 1`,
		`data "portainer_docker_image" "i" {
  endpoint_id = 1
  name        = "x"
}`: `docker image x not found in endpoint 1`,
		`data "portainer_docker_node" "n" {
  endpoint_id = 1
  hostname    = "x"
}`: `docker node with hostname x not found in endpoint 1`,
	} {
		dkDataSourceError(t, f, config, want)
	}
}

func TestStackConversionDataSource(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	f.json("POST", `/stacks/3/convert`, http.StatusOK, map[string]interface{}{"files": map[string]string{"deployment.yaml": "kind: Deployment"}})
	dkDataSourceTest(t, f, `data "portainer_stack_conversion" "c" {
  stack_id      = 3
  target_format = "kubernetes"
  namespace     = "apps"
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_stack_conversion.c", "id", "portainer-stack-conversion-3-kubernetes"),
		resource.TestCheckResourceAttr("data.portainer_stack_conversion.c", "files.deployment.yaml", "kind: Deployment"),
		func(*terraform.State) error {
			if p := dkBody(t, f.last(t, "POST", "/stacks/3/convert")); p["targetFormat"] != "kubernetes" || p["namespace"] != "apps" {
				return fmt.Errorf("unexpected payload %v", p)
			}
			return nil
		},
	))
	dkDataSourceError(t, f, `data "portainer_stack_conversion" "c" {
  stack_id      = 3
  target_format = "yaml"
}`, `expected target_format to be one of \["kubernetes" "helm"\], got yaml`)
}

func TestDockerSnapshotDataSources(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	container := map[string]interface{}{
		"Id": "c1", "Names": []string{"/web"}, "Image": "nginx", "ImageID": "sha256:1", "Command": "nginx",
		"Created": 1700000000, "State": "running", "Status": "Up", "Labels": map[string]string{"a": "b"},
		"Ports": []map[string]interface{}{{"IP": "0.0.0.0", "PrivatePort": 80, "PublicPort": 8080, "Type": "tcp"}},
	}
	f.json("GET", `/docker/1/snapshot`, http.StatusOK, map[string]interface{}{"DockerVersion": "27"})
	f.on("GET", `/docker/1/snapshot/containers`, func(w http.ResponseWriter, r *http.Request, _ []byte, _ []string) {
		if r.URL.Query().Get("edgeStackId") == "5" {
			// The specification's single-container shape.
			dkWriteJSON(w, http.StatusOK, container)
			return
		}
		dkWriteJSON(w, http.StatusOK, []interface{}{container})
	})
	f.json("GET", `/docker/1/snapshot/containers/c1`, http.StatusOK, container)
	dkDataSourceTest(t, f, `
data "portainer_docker_snapshot" "s" {
  endpoint_id = 1
}
data "portainer_docker_snapshot_containers" "all" {
  endpoint_id = 1
}
data "portainer_docker_snapshot_containers" "edge" {
  endpoint_id   = 1
  edge_stack_id = 5
}
data "portainer_docker_snapshot_container" "c" {
  endpoint_id  = 1
  container_id = "c1"
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_docker_snapshot.s", "id", "portainer-docker-snapshot-1"),
		resource.TestCheckResourceAttr("data.portainer_docker_snapshot.s", "snapshot", `{"DockerVersion":"27"}`+"\n"),
		resource.TestCheckResourceAttr("data.portainer_docker_snapshot_containers.all", "containers.#", "1"),
		resource.TestCheckResourceAttr("data.portainer_docker_snapshot_containers.all", "containers.0.ports.0.public_port", "8080"),
		resource.TestCheckResourceAttr("data.portainer_docker_snapshot_containers.all", "containers.0.labels.a", "b"),
		resource.TestCheckResourceAttr("data.portainer_docker_snapshot_containers.edge", "containers.0.id", "c1"),
		resource.TestCheckResourceAttr("data.portainer_docker_snapshot_container.c", "id", "portainer-docker-snapshot-container-1-c1"),
		resource.TestCheckResourceAttr("data.portainer_docker_snapshot_container.c", "names.0", "/web"),
		resource.TestCheckResourceAttr("data.portainer_docker_snapshot_container.c", "created", "1700000000"),
		resource.TestCheckResourceAttrSet("data.portainer_docker_snapshot_container.c", "details"),
	))
	// apiGETRaw reports an error status rather than handing the body on.
	f.json("GET", `/docker/2/snapshot`, http.StatusForbidden, map[string]string{"message": "denied"})
	dkDataSourceError(t, f, `data "portainer_docker_snapshot" "s" {
  endpoint_id = 2
}`, `failed to read the snapshot of environment 2`)
}

func TestDockerDashboardImagesAndGpusDataSources(t *testing.T) {
	t.Parallel()
	f := newDkFake(t)
	f.json("GET", `/docker/1/dashboard`, http.StatusOK, map[string]interface{}{
		"containers": map[string]int{"total": 3, "running": 2, "stopped": 1, "healthy": 1, "unhealthy": 0},
		"images":     map[string]int{"total": 4, "size": 100}, "volumes": 2, "networks": 5, "services": 0, "stacks": 1,
	})
	f.on("GET", `/docker/1/images`, func(w http.ResponseWriter, r *http.Request, _ []byte, _ []string) {
		used := r.URL.Query().Get("withUsage") == "true"
		dkWriteJSON(w, http.StatusOK, []map[string]interface{}{
			{"id": "sha256:a", "tags": []string{"nginx:1"}, "size": 10, "created": 1, "used": used},
			{"id": "sha256:b", "size": 5, "created": 2},
		})
	})
	f.json("GET", `/docker/1/containers/c1/gpus`, http.StatusOK, map[string]string{"gpus": "all"})
	dkDataSourceTest(t, f, `
data "portainer_docker_dashboard" "d" {
  environment_id = 1
}
data "portainer_docker_images" "plain" {
  environment_id = 1
}
data "portainer_docker_images" "usage" {
  environment_id = 1
  with_usage     = true
}
data "portainer_docker_container_gpus" "g" {
  environment_id = 1
  container_id   = "c1"
}`, resource.ComposeTestCheckFunc(
		resource.TestCheckResourceAttr("data.portainer_docker_dashboard.d", "id", "1/dashboard"),
		resource.TestCheckResourceAttr("data.portainer_docker_dashboard.d", "containers_running", "2"),
		resource.TestCheckResourceAttr("data.portainer_docker_dashboard.d", "images_size", "100"),
		resource.TestCheckResourceAttr("data.portainer_docker_images.plain", "id", "1/images"),
		resource.TestCheckResourceAttr("data.portainer_docker_images.plain", "with_usage", "false"),
		resource.TestCheckResourceAttr("data.portainer_docker_images.plain", "total_size", "15"),
		resource.TestCheckResourceAttr("data.portainer_docker_images.plain", "unused_count", "0"),
		resource.TestCheckResourceAttr("data.portainer_docker_images.plain", "images.1.tags.#", "0"),
		resource.TestCheckResourceAttr("data.portainer_docker_images.usage", "unused_count", "1"),
		resource.TestCheckResourceAttr("data.portainer_docker_container_gpus.g", "id", "1/c1/gpus"),
		resource.TestCheckResourceAttr("data.portainer_docker_container_gpus.g", "gpus", "all"),
	))
}
