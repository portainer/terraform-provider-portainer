# Resource Documentation: `portainer_d2k_environment`

# portainer_d2k_environment

> **Requires Portainer Business Edition 3.0 or later.** On Community Edition or an older server the create fails with an error that says so.

Installs d2k in a namespace of a Kubernetes environment and registers the D2K environment that goes with it: a Docker (or Docker Swarm) environment whose Docker API is served by d2k, which translates Docker API calls into Kubernetes resources. Docker resources such as `portainer_stack`, `portainer_docker_network` or `portainer_docker_volume` then work against the D2K environment like against any other Docker environment.

D2K replaces the removed `portainer_compose_convert` (Kompose) resource: rather than converting Compose files into manifests, the Docker workloads run on the cluster directly. To turn an existing stack into Kubernetes manifests or a Helm chart instead, use the `portainer_stack_conversion` data source.

Portainer installs d2k instances in batches; this resource manages exactly one instance, in the namespace `d2k-<name>`. Use `for_each` for several.

## Requirements

- The parent environment must be a **Kubernetes environment connected through an Edge Agent 3.0.0 or later in tunnel (standard) mode**. Kubernetes environments connected via a Portainer Agent, a local connection, or an Edge Agent in **Async** mode cannot run d2k.
- A D2K environment does not support container kill, pause and resume, and its image list is read-only (the kubelet pulls images on the cluster's nodes), so image pulls, builds and removals are not available.

> ⚠️ **Deleting the resource does not uninstall d2k.** Destroying it removes the D2K environment from Portainer only: the d2k install in the `d2k-<name>` namespace and every workload it manages are left running in the cluster. Remove the namespace yourself (for example with `portainer_kubernetes_namespace` or `kubectl delete namespace d2k-<name>`) if you want them gone. If the parent Kubernetes environment is deleted, the D2K environment is marked down, and its d2k install is still left in the cluster.

## Example Usage

```hcl
resource "portainer_d2k_environment" "team_a" {
  parent_environment_id = portainer_environment.k8s_edge.id
  name                  = "team-a"
  environment_name      = "Team A (D2K)"
  swarm_mode            = false
  group_id              = portainer_endpoint_group.teams.id
  tag_ids               = [portainer_tag.d2k.id]
}

resource "portainer_docker_network" "team_a" {
  endpoint_id = portainer_d2k_environment.team_a.environment_id
  name        = "team-a-net"
}
```

Several instances on the same cluster:

```hcl
resource "portainer_d2k_environment" "teams" {
  for_each              = toset(["team-a", "team-b"])
  parent_environment_id = var.kubernetes_environment_id
  name                  = each.key
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `parent_environment_id` | number | ✅ yes | Kubernetes environment d2k is installed in (Edge Agent 3.0.0+, tunnel mode). Changing it forces a new resource. |
| `name` | string | ✅ yes | Name of the d2k install. It runs in the namespace `d2k-<name>`. Changing it forces a new resource. |
| `environment_name` | string | ❌ no | Name of the D2K environment in Portainer. Defaults to `name`. Updated in place. |
| `swarm_mode` | bool | ❌ no | Emulate a Docker Swarm manager (services, configs, secrets, Swarm stacks) rather than a standalone Docker engine. Defaults to `false`. Fixed at install time, so changing it forces a new resource. |
| `group_id` | number | ❌ no | Endpoint group of the D2K environment. Defaults to `1` (Unassigned). Updated in place. |
| `tag_ids` | list(number) | ❌ no | Tags of the D2K environment. Updated in place. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `id` | string | ID of the D2K environment. |
| `environment_id` | number | ID of the D2K environment as a number, for the `endpoint_id` of Docker resources. |
| `namespace` | string | Namespace of the parent environment the d2k install runs in (`d2k-<name>`). |

An instance that Portainer fails to install fails the apply with the reason Portainer reported. When Portainer created the environment despite the error, it is kept in state and marked tainted, so the next apply replaces it.

## Import

```bash
terraform import portainer_d2k_environment.team_a 12
```

The import ID is the ID of the D2K environment. `name` is read back from the namespace.
