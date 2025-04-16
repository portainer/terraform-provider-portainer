# ⚙️ **Resource Documentation: `portainer_endpoint_settings`**

# portainer_endpoint_settings
The `portainer_endpoint_settings` resource allows you to configure per-endpoint security and GPU management settings in Portainer.
## Example Usage
```hcl
resource "portainer_endpoint_settings" "example" {
  endpoint_id                    = var.endpoint_id
  allow_bind_mounts             = true
  allow_container_capabilities  = true
  allow_device_mapping          = true
  allow_host_namespace          = true
  allow_privileged_mode         = false
  allow_stack_management        = true
  allow_sysctl_setting          = true
  allow_volume_browser          = true
  enable_gpu_management         = false
  enable_host_management        = true

  dynamic "gpus" {
    for_each = var.gpus
    content {
      name  = gpus.value.name
      value = gpus.value.value
    }
  }
}
```

## Lifecycle & Behavior
Settings of Endpoints in Portainer are modify if any of the arguments change by run:
```hcl
trraform apply
```

## Arguments Reference
### Main Attributes
| Name                         | Type   | Required | Description                                          |
|------------------------------|--------|----------|------------------------------------------------------|
| `endpoint_id`                | number | ✅ yes   | ID of the environment (endpoint) to configure        |
| `allow_bind_mounts`          | bool   | 🚫 no    | Allow bind mounts for regular users                  |
| `allow_container_capabilities` | bool | 🚫 no    | Allow setting container capabilities for regular users |
| `allow_device_mapping`       | bool   | 🚫 no    | Allow device mapping                                 |
| `allow_host_namespace`       | bool   | 🚫 no    | Allow use of host namespaces                         |
| `allow_privileged_mode`      | bool   | 🚫 no    | Allow privileged containers                          |
| `allow_stack_management`     | bool   | 🚫 no    | Allow regular users to manage stacks                 |
| `allow_sysctl_setting`       | bool   | 🚫 no    | Allow sysctl settings                                |
| `allow_volume_browser`       | bool   | 🚫 no    | Allow volume browser in UI                           |
| `enable_gpu_management`      | bool   | 🚫 no    | Enable GPU selection for deployments                 |
| `enable_host_management`     | bool   | 🚫 no    | Enable host management features in Portainer UI      |

### `gpus` Block
| Name   | Type   | Required | Description                     |
|--------|--------|----------|---------------------------------|
| `name` | string | ✅ yes   | GPU name (e.g. "nvidia")        |
| `value`| string | ✅ yes   | GPU value (e.g. "gpu0")         |

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` |ID of the resource (same as `endpoint_id`) |
