# 🧩 **Resource Documentation: `portainer_docker_node`**

# portainer_docker_node
The `portainer_docker_node` resource allows you to update, inspect, and delete Docker Swarm nodes via the Portainer API.

You can change attributes such as availability, role, name or labels, or even remove the node entirely from the cluster using this resource.

## Example Usage
```hcl
resource "portainer_docker_node" "example" {
  endpoint_id  = 1
  node_id      = "wna048ajhbc1n1t5ispvf6mvg"
  version      = 4869
  name         = "node-name"
  availability = "active"
  role         = "manager"

  labels = {
    foo = "barrerun"
  }
}

```

## Lifecycle & Behavior
- You can update node's role, availability, name, or labels by running:
```hcl
terraform apply
```

- To destroy the node:
```hcl
terraform destroy
```

## Arguments Reference
| Name         | Type        | Required     | Description                                                          |
|--------------|-------------|--------------|----------------------------------------------------------------------|
| `endpoint_id`| number      | ✅ yes       | ID of the Portainer environment (endpoint)                          |
| `node_id`    | string      | ✅ yes       | ID of the Docker Swarm node to update                               |
| `version`    | number      | ✅ yes       | Version of the swarm node, required for updates and deletion        |
| `name`       | string      | 🚫 optional  | Custom name to assign to the node                                   |
| `availability`| string     | 🚫 optional  | Node availability (`active`, `pause`, or `drain`)                   |
| `role`       | string      | 🚫 optional  | Node role in the cluster (`manager` or `worker`)                    |
| `labels`     | map(string) | 🚫 optional  | Key-value metadata labels to attach to the node                     |
                     |

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` | A combination of endpoint ID and node ID (`{endpoint}-{node}`) |

## Import

Swarm nodes can be imported as `<endpoint_id>:<node_id>`. The format from before v3.0.0, `<endpoint_id>-<node_id>`, is still accepted.

```shell
terraform import portainer_docker_node.example 1:ktnbjxoalbkvbvedmg1urrz8h
```
