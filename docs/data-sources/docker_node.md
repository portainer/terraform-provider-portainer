# 🖥️ **Data Source Documentation: `portainer_docker_node`**

# portainer_docker_node
The `portainer_docker_node` data source allows you to look up an existing Docker node (host) within a Portainer Swarm environment.

## Example Usage

### Look up a Swarm node by hostname

```hcl
data "portainer_docker_node" "worker1" {
  endpoint_id = 1
  hostname    = "swarm-worker-01"
}

output "node_role" {
  value = data.portainer_docker_node.worker1.role
}
```

## Arguments Reference

| Name          | Type    | Required | Description                              |
|---------------|---------|----------|------------------------------------------|
| `endpoint_id` | integer | ✅ yes   | ID of the environment (Swarm cluster).   |
| `hostname`    | string  | ✅ yes   | Hostname of the Docker node.            |

## Attributes Reference

| Name     | Type   | Description                      |
|----------|--------|----------------------------------|
| `id`     | string | ID of the Docker node.           |
| `role`   | string | Role (manager/worker).           |
| `status` | string | Status (ready/down).             |
