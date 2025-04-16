# 🔐 **Resource Documentation: `portainer_docker_secret`**

# portainer_docker_secret
The `portainer_docker_secret` resource allows you to manage Docker secrets within a specific environment (endpoint) in Portainer.
Secrets are created with base64-encoded content and can include labels, driver-specific options, and templating configuration.

## Example Usage
```hcl
resource "portainer_docker_secret" "example_secret" {
  endpoint_id = 1
  name        = "app-key.crt"
  data        = base64encode("THIS IS NOT A REAL CERTIFICATE\n")

  labels = {
    "com.example.some-label" = "some-value"
  }

  templating = {
    name     = "some-driver"
    OptionA  = "value for driver-specific option A"
  }
}
```

## Lifecycle & Behavior
Docker secrets are **immutable**. Updating them (changing `data, labels`, etc.) will **force recreation**.

Terraform will automatically destroy and re-create secrets on change.

Use `terraform destroy` to remove the secret.

## Arguments Reference
| Name        | Type         | Required     | Description                                                       |
|-------------|--------------|--------------|-------------------------------------------------------------------|
| endpoint_id | int          | ✅ yes       | ID of the environment (endpoint) in Portainer                     |
| name        | string       | ✅ yes       | Name of the Docker secret                                         |
| data        | string       | ✅ yes       | Base64-encoded string containing the secret content               |
| labels      | map(string)  | 🚫 optional  | Map of labels to associate with the secret                        |
| driver      | map(string)  | 🚫 optional  | Secret driver configuration (e.g., `name`, `Options`)             |
| templating  | map(string)  | 🚫 optional  | Templating configuration for the secret                           |
> ⚠️ Note: **The `data` must be a valid base64-encoded string. Use Terraform's `base64encode()` function if needed.**

## Attributes Reference

| Name | Description              |
|------|--------------------------|
| `id` | ID of the created Docker secret (from Portainer) |
