# ⚙️ **Resource Documentation: `portainer_endpoint_settings`**

# portainer_endpoint_settings
The `portainer_endpoint_settings` resource allows you to configure per-endpoint security and GPU management settings in Portainer.
## Example Usage
```hcl
resource "portainer_endpoint_settings" "example" {
  endpoint_id               = 3
  enable_gpu_management     = false

  security_settings {
    allow_bind_mounts            = true
    allow_container_capabilities = true
    allow_device_mapping         = true
    allow_host_namespace         = true
    allow_privileged_mode        = false
    allow_stack_management       = true
    allow_sysctl_setting         = true
    allow_security_opt           = false
    allow_volume_browser         = true
    enable_host_management       = true
  }

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
terraform apply
```

## Arguments Reference

### Main Attributes

| Name                           | Type   | Required | Description                                                        |
|--------------------------------|--------|----------|--------------------------------------------------------------------|
| `endpoint_id`                  | number | ✅ yes   | ID of the environment (endpoint) to configure                      |
| `enable_gpu_management`        | bool   | 🚫 optional | Enable GPU selection for deployments (`default: false`)            |
| `enable_image_notification`    | bool   | 🚫 optional | Enable image update notifications (`default: false`)               |

### `gpus` Block

| Name     | Type   | Required | Description                      |
|----------|--------|----------|----------------------------------|
| `name`   | string | ✅ yes   | GPU name (e.g. `"nvidia"`)       |
| `value`  | string | ✅ yes   | GPU identifier (e.g. `"gpu0"`)   |

### `change_window` Block

| Name        | Type   | Required | Description                          |
|-------------|--------|----------|--------------------------------------|
| `enabled`   | bool   | 🚫 optional | Whether the change window is enabled |
| `start_time`| string | 🚫 optional | Start time in `HH:MM` format         |
| `end_time`  | string | 🚫 optional | End time in `HH:MM` format           |

### `deployment_options` Block

| Name                    | Type   | Required | Description                                       |
|-------------------------|--------|----------|---------------------------------------------------|
| `hide_add_with_form`    | bool   | 🚫 optional | Hide the “Deploy via form” stack UI option        |
| `hide_file_upload`      | bool   | 🚫 optional | Hide “Upload docker-compose file” UI              |
| `hide_web_editor`       | bool   | 🚫 optional | Hide the “Web editor” stack deployment UI         |
| `override_global_options`| bool  | 🚫 optional | Override global deployment options                |

### `security_settings` Block

| Name                         | Type   | Required | Description                                                  |
|------------------------------|--------|----------|--------------------------------------------------------------|
| `allow_bind_mounts`          | bool   | 🚫 optional | Allow bind mounts for regular users                          |
| `allow_container_capabilities`| bool  | 🚫 optional | Allow setting container capabilities                         |
| `allow_device_mapping`       | bool   | 🚫 optional | Allow device mapping                                         |
| `allow_host_namespace`       | bool   | 🚫 optional | Allow use of host namespaces                                 |
| `allow_privileged_mode`      | bool   | 🚫 optional | Allow privileged containers                                  |
| `allow_stack_management`     | bool   | 🚫 optional | Allow stack management                                       |
| `allow_sysctl_setting`       | bool   | 🚫 optional | Allow sysctl settings                                        |
| `allow_security_opt`         | bool   | 🚫 optional | Allow security options (`--security-opt`) on containers      |
| `allow_volume_browser`       | bool   | 🚫 optional | Allow volume browser                                         |
| `enable_host_management`     | bool   | 🚫 optional | Enable host management in the UI                             |

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` |ID of the resource (same as `endpoint_id`) |

## Import

Environment settings can be imported using the numeric environment ID:

```shell
terraform import portainer_endpoint_settings.example 3
```

The import fills in `enable_gpu_management`, `gpus` and `security_settings` from Portainer. Business Edition settings (`change_window`, `deployment_options`, `enable_image_notification`) are not returned by the API and stay as configured.
