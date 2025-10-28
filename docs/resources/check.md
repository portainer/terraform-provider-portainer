# ✅ **Resource Documentation: `portainer_check`**

# portainer_check

The `portainer_check` resource validates that one or more containers (in standalone mode) or services (in swarm mode) are running with the **expected image revision (tag)** and **desired runtime state** in a Portainer-managed environment.

> You can use it in both **Docker Standalone** and **Docker Swarm** deployments.
> It’s especially useful for CI/CD pipelines to verify that a deployment or update has completed successfully before proceeding to the next step.

---

## 🚀 Example Usage
- [Example on GitHub](https://github.com/portainer/terraform-provider-portainer/tree/main/examples/deployment)

### ✅ Check service status in Docker Swarm

```hcl
resource "portainer_check" "swarm_check" {
  endpoint_id     = 1
  stack_name      = "my-swarm-stack"
  services_list   = "web,api"
  revision        = "1.29"
  desired_state   = "running"
  max_retries     = 3
  wait            = 10
  wait_between_checks = 5
}
```

### ✅ Check container status in Docker Standalone

```hcl
resource "portainer_check" "standalone_check" {
  endpoint_id     = 1
  stack_name      = "nginx"
  services_list   = "web"
  revision        = "1.29"
  desired_state   = "running"
  wait            = 10
  max_retries     = 3
  wait_between_checks = 5
}
```

---

## ⚙️ Lifecycle & Behavior

This resource is **stateless** — it performs runtime verification during `terraform apply` (or `tofu apply` for OpenTofu) and does **not** persist state in Portainer.

When executed:

1. It waits for the optional `wait` period before starting.
2. It determines whether the target environment is **Swarm** or **Standalone**.
3. It checks the matching services or containers for:

   * Correct **image tag** (`revision`)
   * Correct **state** (e.g., `running`)
4. If all targets match → ✅ success.
   Otherwise → ❌ fails after `max_retries`.

> 💡 **Pro Tip:** Combine `portainer_check` after a `portainer_deploy` or `portainer_container_exec` to ensure deployment integrity.

---

## 📥 Arguments Reference

| Name                  | Type   | Required                          | Description                                                                           |
| --------------------- | ------ | --------------------------------- | ------------------------------------------------------------------------------------- |
| `endpoint_id`         | int    | ✅ yes                             | ID of the Portainer environment (endpoint) where the stack or containers are located. |
| `stack_name`          | string | ✅ yes                             | Name of the stack to which the containers or services belong.                         |
| `revision`            | string | ✅ yes                             | Expected image tag (e.g., `"1.29"`) that should be currently running.                 |
| `services_list`       | string | ✅ yes                             | Comma-separated list of service names (without stack prefix). Example: `"web,api"`.   |
| `desired_state`       | string | 🚫 optional (default `"running"`) | Desired container/service state. Usually `"running"`.                                 |
| `wait`                | int    | 🚫 optional (default `30`)        | Seconds to wait before performing the first check (useful after deploy).              |
| `wait_between_checks` | int    | 🚫 optional (default `30`)        | Delay (in seconds) between each retry attempt.                                        |
| `max_retries`         | int    | 🚫 optional (default `3`)         | Number of retry attempts before failing the check.                                    |

---

## 📤 Attributes Reference

| Name     | Description                                                                                                     |
| -------- | --------------------------------------------------------------------------------------------------------------- |
| `id`     | Auto-generated ID of the check execution (stateless).                                                           |
| `output` | The complete textual output of the verification process, including matched containers, retries, and debug info. |

---

## 🧩 Example with Outputs

```hcl
output "check_result" {
  value = portainer_check.standalone_check.output
}
```

This will show you a detailed report like:

```
Docker Standalone detected — using container check logic.
DEBUG: checking container="nginx-web-1" (image="nginx:1.29", state="running")
Container "nginx-web-1" OK — revision "1.29", state "running"
```

---

## 🧠 Summary

| Feature     | Description                                                          |
| ----------- | -------------------------------------------------------------------- |
| Mode        | Works in **Standalone** and **Swarm** environments                   |
| Purpose     | Ensures containers/services run with the correct image tag and state |
| Behavior    | Stateless verification (no Portainer state change)                   |
| Typical Use | Post-deployment validation in CI/CD pipelines                        |
