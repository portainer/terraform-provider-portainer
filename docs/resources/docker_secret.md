# 🔐 **Resource Documentation: `portainer_docker_secret`**

# portainer_docker_secret
The `portainer_docker_secret` resource allows you to manage Docker secrets within a specific environment (endpoint) in Portainer.
Secrets are created with base64-encoded content and can include labels, driver-specific options, and templating configuration.

## Example Usage
### Standard Secret
- [Example on GitHub](https://github.com/portainer/terraform-provider-portainer/tree/main/examples/docker_secret)

```hcl
resource "portainer_docker_secret" "example_secret" {
  endpoint_id = 1
  name        = "app-key.crt"
  data        = base64encode("THIS IS NOT A REAL CERTIFICATE\n")

  labels = {                                               # optional
    "com.example.some-label" = "some-value"
  }

  driver = {                                               # optional
    name    = "secret-driver"
    option1 = "value1"
    option2 = "value2"
  }

  templating = {                                           # optional
    name     = "some-driver"
    OptionA  = "value for driver-specific option A"
  }
}
```
### Ephemeral / Write-only Secret
```hcl
resource "portainer_docker_secret" "example_secret_wo" {
  endpoint_id      = 1
  name             = "api-token"
  data_wo          = base64encode("SOME API TOKEN")
  data_wo_version  = 1                                     # increment this to force secret rotation

  labels = {                                               # optional
    "env"     = "prod"
    "purpose" = "api-auth"
  }

  templating = {                                           # optional
    name     = "custom-driver"
    OptionA  = "dynamic-secret"
  }
}
```

## Lifecycle & Behavior
Docker secrets are **immutable**. Updating them (changing `data, labels`, etc.) will **force recreation**.

Terraform will automatically destroy and re-create secrets on change.

Use `terraform destroy` to remove the secret.

## Arguments Reference
| Name              | Type        | Required    | Description                                                                                      |
| ----------------- | ----------- | ----------- | ------------------------------------------------------------------------------------------------ |
| `endpoint_id`     | int         | ✅ yes       | ID of the environment (endpoint) in Portainer                                                    |
| `name`            | string      | ✅ yes       | Name of the Docker secret                                                                        |
| `data`            | string      | 🚫 optional | Base64-encoded string containing the secret content (stored in Terraform state)                  |
| `data_wo`         | string      | 🚫 optional | Write-only Base64-encoded secret data (not stored in Terraform state; supports ephemeral values) |
| `data_wo_version` | int         | 🚫 optional | Version flag for write-only secret; changing this value triggers secret rotation (ForceNew)      |
| `labels`          | map(string) | 🚫 optional | Map of labels to associate with the secret                                                       |
| `driver`          | map(string) | 🚫 optional | Secret driver configuration (e.g., `name`, `Options`)                                            |
| `templating`      | map(string) | 🚫 optional | Templating configuration for the secret                                                          |
> ⚠️ Note: **The `data` must be a valid base64-encoded string. Use Terraform's `base64encode()` function if needed.**

## Attributes Reference

| Attribute             | Description                                                                 |
| --------------------- | --------------------------------------------------------------------------- |
| `id`                  | ID of the created Docker secret (from Portainer)                            |
| `resource_control_id` | ID of the automatically generated Portainer ResourceControl for this secret |
