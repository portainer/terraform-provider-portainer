# 🔑 **Data Source Documentation: `portainer_cloud_credentials`**

# portainer_cloud_credentials
The `portainer_cloud_credentials` data source allows you to look up existing Portainer cloud credentials by their name.

## Example Usage

### Look up cloud credentials by name

```hcl
data "portainer_cloud_credentials" "aws" {
  name = "My AWS Keys"
}

output "credentials_id" {
  value = data.portainer_cloud_credentials.aws.id
}
```

## Arguments Reference

| Name   | Type   | Required | Description                       |
|--------|--------|----------|-----------------------------------|
| `name` | string | ✅ yes   | Name of the cloud credentials set. |

## Attributes Reference

| Name             | Type   | Description                            |
|------------------|--------|----------------------------------------|
| `id`             | string | ID of the Portainer cloud credentials. |
| `cloud_provider` | string | The provider (aws/gcp/azure/etc).      |
