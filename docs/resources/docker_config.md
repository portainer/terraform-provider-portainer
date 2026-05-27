# 🧾 **Resource Documentation: `portainer_docker_config`**

## `portainer_docker_config`

The `portainer_docker_config` resource allows you to manage Docker configs within a specific environment (endpoint) in Portainer.
Configs are immutable, and any change will cause them to be re-created.

---

## 📘 Example Usage

```hcl
resource "portainer_docker_config" "example_config" {
  endpoint_id = 1
  name        = "server.conf"
  data        = base64encode("THIS IS NOT A REAL CERTIFICATE\n")

  labels = {
    property1 = "string"
    property2 = "string"
    foo       = "bar"
  }

  templating = {
    name     = "some-driver"
    OptionA  = "value for driver-specific option A"
    OptionB  = "value for driver-specific option B"
  }
}
```

## Lifecycle & Behavior
Updating them (changing `data, labels`, etc.) will **force recreation**.

Terraform will automatically destroy and re-create config on change.

Use `terraform destroy` to remove the config.

## Arguments Reference
| Name        | Type         | Required     | Description                                                       |
|-------------|--------------|--------------|-------------------------------------------------------------------|
| endpoint_id | int          | ✅ yes       | ID of the environment (endpoint) in Portainer                     |
| name        | string       | ✅ yes       | Name of the Docker config                                         |
| data        | string       | ✅ yes       | Base64-encoded string containing the config content               |
| labels      | map(string)  | 🚫 optional  | Map of labels to associate with the config                        |
| templating  | map(string)  | 🚫 optional  | Templating configuration (e.g., `name`, `Options`)                |

> ⚠️ Note: **The `data` must be a valid base64-encoded string. Use Terraform's `base64encode()` function if needed.**

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` | ID of the created Docker config (from Portainer) |
| `resource_control_id` | ID of the automatically generated Portainer ResourceControl for this config |

## Import

Docker configs can be imported using a composite ID in the form `<endpoint_id>-<config_id>`, where `<endpoint_id>` is the numeric ID of the Portainer environment and `<config_id>` is the Docker Swarm config ID (a string):

```shell
terraform import portainer_docker_config.example 1-abc123
```
