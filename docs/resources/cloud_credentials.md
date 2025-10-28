# ☁️🔑 **Resource Documentation: `portainer_cloud_credentials`**

# portainer_cloud_credentials
The `portainer_cloud_credentials` resource allows you to provision cloud credentials in Portainer for use with providers like AWS, DigitalOcean, Civo, etc.

> Currently working only for Portainer BE edition

## Example Usage
- [Example on GitHub](https://github.com/portainer/terraform-provider-portainer/tree/main/examples/cloud_credentials)

```hcl
resource "portainer_cloud_credentials" "example" {
  name     = "example-aws-creds"
  cloud_provider = "aws"

  credentials = jsonencode({
    accessKeyId     = "your-access-key"
    secretAccessKey = "your-secret-key"
    region          = "eu-central-1"
  })
}
```
## Lifecycle & Behavior
Cloud credentials are created via the `/cloud/credentials` API.

You can only delete credentials if they are not attached to any endpoint.

If deletion fails due to attached endpoints, an error will be thrown.

Credential values are considered sensitive and write-only.

- To destroy credentials (if unused by Portainer endpoints):
```hcl
terraform destroy
```

- To update cloud credentials, update the relevant fields and re-apply:
```hcl
terraform apply
```

## Arguments Reference
| **Name**      | **Type** | **Required** | **Description**                                                            |
|---------------|----------|--------------|----------------------------------------------------------------------------|
| `name`        | string   | ✅ yes       | Human-readable name for the cloud credentials                             |
| `cloud_provider`| string   | ✅ yes       | Provider name (`aws`, `digitalocean`, `civo`, `gcp`, etc.)                |
| `credentials` | string   | ✅ yes       | JSON-encoded credentials payload (use `jsonencode({ ... })`)              |

## Attributes Reference
| Name | Description              |
|------|--------------------------|
| `id` | ID of the created cloud credentials     |
