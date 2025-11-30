# 🧩 **Resource Documentation: `portainer_docker_volume`**

# portainer_docker_volume
The `portainer_docker_volume` resource allows you to create and manage Docker volumes via the Portainer API.

## Example Usage

### Create a Docker Volume
```hcl
resource "portainer_docker_volume" "example" {
  endpoint_id = 1
  name        = "my-test-volume"
  driver      = "local"
}
```

### Advanced: Create a Volume with driver configuration and labels
```hcl
resource "portainer_docker_volume" "example" {
  endpoint_id = 1
  name        = "my-test-volume"
  driver      = "local"

  driver_opts = {
    device = "tmpfs"
    o      = "size=100m,uid=1000"
    type   = "tmpfs"
  }

  labels = {
    env     = "test"
    managed = "terraform"
  }
}
```

### Advanced: Create a Volume with ClusterVolumeSpec
```hcl
resource "portainer_docker_volume" "clustered" {
  endpoint_id = 1
  name        = "shared-data"
  driver      = "custom"

  cluster_volume_spec {
    group = "analytics"

    access_mode {
      scope   = "single"
      sharing = "none"
      mount_volume = {
        path = "/mnt/data"
      }
    }

    secrets = [
      {
        key    = "vol-cred"
        secret = "secret-ref"
      }
    ]

    accessibility_requirements {
      requisite = [
        {
          property1 = "ssd"
          property2 = "zone-a"
        }
      ]
      preferred = [
        {
          property1 = "nvme"
          property2 = "zone-b"
        }
      ]
    }

    capacity_range {
      required_bytes = 1000000000
      limit_bytes    = 5000000000
    }

    availability = "active"
  }
}
```

## Lifecycle & Behavior
Creating a volume sends a POST request to the Docker API via Portainer.

Deleting a volume removes it via the corresponding DELETE call.
- You can recreate the volume by changing its name or any ForceNew parameter and running:
```hcl
terraform apply
```

- To destroy the volume:
```hcl
terraform destroy
```

## Arguments Reference
| Name                  | Type          | Required    | Description                                                            |
| --------------------- | ------------- | ----------- | ---------------------------------------------------------------------- |
| `endpoint_id`         | `int`         | ✅ yes       | ID of the Portainer environment (endpoint)                             |
| `name`                | `string`      | ✅ yes       | Name of the Docker volume                                              |
| `driver`              | `string`      | ✅ yes       | Volume driver to use (e.g., `local`, `custom`)                         |
| `driver_opts`         | `map(string)` | 🚫 optional | Driver-specific options (e.g., `device`, `type`, `o`)                  |
| `labels`              | `map(string)` | 🚫 optional | Key-value metadata applied to the volume                               |
| `cluster_volume_spec` | `block`       | 🚫 optional | Cluster-level volume configuration for scheduling, secrets, and access |


### `cluster_volume_spec` block
| Attribute                    | Type           | Required    | Description                          |
| ---------------------------- | -------------- | ----------- | ------------------------------------ |
| `group`                      | `string`       | 🚫 optional | Logical volume group name            |
| `access_mode`                | `block`        | 🚫 optional | Access mode configuration            |
| `secrets`                    | `list(object)` | 🚫 optional | List of secrets to be attached       |
| `accessibility_requirements` | `block`        | 🚫 optional | Node preference/restriction settings |
| `capacity_range`             | `block`        | 🚫 optional | Volume capacity constraints          |
| `availability`               | `string`       | 🚫 optional | Availability state (`active`, etc.)  |


### `access_mode` block
| Attribute      | Type          | Description                           |
| -------------- | ------------- | ------------------------------------- |
| `scope`        | `string`      | e.g., `single`, `multi`               |
| `sharing`      | `string`      | e.g., `none`, `readonly`, `readwrite` |
| `mount_volume` | `map(string)` | Arbitrary key-value mount options     |


### `secrets` block
| Attribute | Type     | Description                   |
| --------- | -------- | ----------------------------- |
| `key`     | `string` | Key used within the volume    |
| `secret`  | `string` | Name of the referenced secret |


### `accessibility_requirements` block
| Attribute   | Type                | Description                       |
| ----------- | ------------------- | --------------------------------- |
| `requisite` | `list(map(string))` | List of required node properties  |
| `preferred` | `list(map(string))` | List of preferred node properties |


### `capacity_range` block
| Attribute        | Type | Description                       |
| ---------------- | ---- | --------------------------------- |
| `required_bytes` | int  | Minimum storage required in bytes |
| `limit_bytes`    | int  | Maximum storage allowed in bytes  |


## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` | Unique identifier of the volume |
| `resource_control_id` | ID of the automatically generated Portainer ResourceControl for this volume |
