# 🔌 **Resource Documentation: `portainer_docker_plugin`**

## `portainer_docker_plugin`

The `portainer_docker_plugin` resource allows installation and (optionally) enabling of a Docker plugin on a selected Portainer endpoint. This resource supports passing plugin privileges such as `mount`, `device`, `capabilities`, etc., as well as setting an alias (`name`) and supplying a `X-Registry-Auth` header.

---

## 📦 **Example Usage**

### Install and enable `rclone/docker-volume-rclone` plugin:

```hcl
resource "portainer_docker_plugin" "rclone" {
  endpoint_id   = 3
  remote        = "rclone/docker-volume-rclone"
  name          = "rclone"
  enable        = true
  registry_auth = "e30=" # base64-encoded {}

  settings {
    name  = "network"
    value = ["host"]
  }

  settings {
    name  = "mount"
    value = ["/var/lib/docker-plugins/rclone/config"]
  }

  settings {
    name  = "mount"
    value = ["/var/lib/docker-plugins/rclone/cache"]
  }

  settings {
    name  = "device"
    value = ["/dev/fuse"]
  }

  settings {
    name  = "capabilities"
    value = ["CAP_SYS_ADMIN"]
  }
}
```
- [Example on GitHub](https://github.com/portainer/terraform-provider-portainer/tree/main/examples/docker_plugin)

---

## ⚙️ **Arguments Reference**

| Name            | Type         | Required | Description                                                        |
| --------------- | ------------ | -------- | ------------------------------------------------------------------ |
| `endpoint_id`   | int          | ✅ yes    | ID of the Portainer endpoint where the plugin will be installed    |
| `remote`        | string       | ✅ yes    | Plugin image name (e.g., `rclone/docker-volume-rclone`)            |
| `name`          | string       | 🚫 optional | Local alias for the plugin (e.g., `rclone`)                        |
| `enable`        | bool         | 🚫 optional | Whether to enable the plugin after installation (default: `false`) |
| `registry_auth` | string       | 🚫 optional | Value for the `X-Registry-Auth` header (default: `e30=` = `{}`)    |
| `settings`      | list(object) | 🚫 optional | List of objects with name and value defining plugin privileges     |

---

### 🔧 `settings` block

| Name          | Type         | Required | Description                                                     |
| ------------- | ------------ | -------- | --------------------------------------------------------------- |
| `name`        | string       | ✅ yes    | Type of privilege: `network`, `mount`, `device`, `capabilities` |
| `value`       | list(string) | ✅ yes    | List of values for the given privilege                          |
| `description` | string       | 🚫 optional | Optional description (ignored if empty)                         |

---

## 💨 **Lifecycle & Behavior**

* Docker plugins are immutable; `Update` is not supported.
* Changing `remote`, `settings`, `enable`, or `registry_auth` will destroy and re-create the plugin (`ForceNew`).
* If `enable = true`, an additional API request is performed after successful installation:
  `POST /endpoints/{id}/docker/plugins/{name}/enable`

---

## 🛄 **Attributes Reference**

| Name | Description                                               |
| ---- | --------------------------------------------------------- |
| `id` | Plugin name (e.g., `rclone:latest`) used as the unique ID |
