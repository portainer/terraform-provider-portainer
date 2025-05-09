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

| Name                           | Type   | Required | Description                                                        |
|--------------------------------|--------|----------|--------------------------------------------------------------------|
| `endpoint_id`                  | number | ✅ yes   | ID of the environment (endpoint) to configure                      |
| `allow_bind_mounts`            | bool   | 🚫 no    | Allow bind mounts for regular users                                |
| `allow_container_capabilities` | bool   | 🚫 no    | Allow setting container capabilities for regular users             |
| `allow_device_mapping`         | bool   | 🚫 no    | Allow device mapping                                               |
| `allow_host_namespace`         | bool   | 🚫 no    | Allow use of host namespaces                                       |
| `allow_privileged_mode`        | bool   | 🚫 no    | Allow privileged containers                                        |
| `allow_stack_management`       | bool   | 🚫 no    | Allow regular users to manage stacks                               |
| `allow_sysctl_setting`         | bool   | 🚫 no    | Allow sysctl settings                                              |
| `allow_volume_browser`         | bool   | 🚫 no    | Allow volume browser in UI                                         |
| `enable_gpu_management`        | bool   | 🚫 no    | Enable GPU selection for deployments                               |
| `enable_host_management`       | bool   | 🚫 no    | Enable host management features in Portainer UI                    |
| `enable_image_notification`    | bool   | 🚫 no    | Enable image update notifications                                  |

### `gpus` Block

| Name     | Type   | Required | Description                      |
|----------|--------|----------|----------------------------------|
| `name`   | string | ✅ yes   | GPU name (e.g. `"nvidia"`)       |
| `value`  | string | ✅ yes   | GPU identifier (e.g. `"gpu0"`)   |

### `change_window` Block

| Name        | Type   | Required | Description                          |
|-------------|--------|----------|--------------------------------------|
| `enabled`   | bool   | 🚫 no    | Whether the change window is enabled |
| `start_time`| string | 🚫 no    | Start time in `HH:MM` format         |
| `end_time`  | string | 🚫 no    | End time in `HH:MM` format           |

### `deployment_options` Block

| Name                    | Type   | Required | Description                                       |
|-------------------------|--------|----------|---------------------------------------------------|
| `hide_add_with_form`    | bool   | 🚫 no    | Hide the “Deploy via form” stack UI option        |
| `hide_file_upload`      | bool   | 🚫 no    | Hide “Upload docker-compose file” UI              |
| `hide_web_editor`       | bool   | 🚫 no    | Hide the “Web editor” stack deployment UI         |
| `override_global_options`| bool  | 🚫 no    | Override global deployment options                |

### `security_settings` Block

| Name                         | Type   | Required | Description                                                  |
|------------------------------|--------|----------|--------------------------------------------------------------|
| `allow_bind_mounts`          | bool   | 🚫 no    | Allow bind mounts for regular users                          |
| `allow_container_capabilities`| bool  | 🚫 no    | Allow setting container capabilities                         |
| `allow_device_mapping`       | bool   | 🚫 no    | Allow device mapping                                         |
| `allow_host_namespace`       | bool   | 🚫 no    | Allow use of host namespaces                                 |
| `allow_privileged_mode`      | bool   | 🚫 no    | Allow privileged containers                                  |
| `allow_stack_management`     | bool   | 🚫 no    | Allow stack management                                       |
| `allow_sysctl_setting`       | bool   | 🚫 no    | Allow sysctl settings                                        |
| `allow_volume_browser`       | bool   | 🚫 no    | Allow volume browser                                         |
| `enable_host_management`     | bool   | 🚫 no    | Enable host management in the UI                             |

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` |ID of the resource (same as `endpoint_id`) |
