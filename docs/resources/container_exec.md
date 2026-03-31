# 🧠 **Resource Documentation: `portainer_container_exec`**

# portainer_container_exec
The `portainer_container_exec` resource allows you to remotely execute a command inside a running container managed by Portainer.
> You can target a container in a **standalone** or **swarm** environment.

## Example Usage

### Run command in standalone container
```hcl
resource "portainer_container_exec" "standalone" {
  endpoint_id   = 1
  service_name  = "my-nginx-container"
  command       = "nginx -v"
  user          = "root"
}
```

### Run command in swarm service container
```hcl
resource "portainer_container_exec" "swarm_exec" {
  endpoint_id   = 2
  service_name  = "my-service-name"
  command       = "ls -la /etc"
  user          = "root"
  mode          = "swarm"
}
```
---

## Lifecycle & Behavior
This resource is stateless – it runs once when terraform apply is called.
> 💡 Pro Tip: You can output the result like this:
```hcl
output "exec_output" {
  value = portainer_container_exec.standalone.output
}
```
---

## Timeouts

This resource supports the following [timeouts](https://developer.hashicorp.com/terraform/language/resources/syntax#operation-timeouts) configuration options:

| Operation | Default | Description                                    |
|-----------|---------|------------------------------------------------|
| `create`  | 5m      | Time to wait for command execution to complete |

### Example
```hcl
resource "portainer_container_exec" "standalone" {
  endpoint_id   = 1
  service_name  = "my-nginx-container"
  command       = "nginx -v"
  user          = "root"

  timeouts {
    create = "10m"
  }
}
```

---

## Arguments Reference
| Name          | Type   | Required    | Description                                                               |
|---------------|--------|-------------|---------------------------------------------------------------------------|
| `endpoint_id` | int    | ✅ yes      | ID of the Portainer environment                                           |
| `service_name`| string | ✅ yes      | Name of the container (for standalone) or service (for swarm)             |
| `command`     | string | ✅ yes      | Command to execute inside the container                                   |
| `user`        | string | 🚫 optional | User to run the command as (default: `"root:root"`)                       |
| `wait`        | int    | 🚫 optional | Seconds to wait before executing the command (default: `0`)               |
| `mode`        | string | 🚫 optional | Deployment type: `"standalone"` (default) or `"swarm"`                    |

---

## Attributes Reference

| Name | Description                     |
|------|---------------------------------|
| `id` | ID of the execution instance    |
| `output` | Output (stdout/stderr) from the executed command |
