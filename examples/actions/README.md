# Action `portainer_endpoint_snapshot`

Snapshots Portainer environments from a Terraform action: once after
`terraform_data.release` is created, and again whenever `release` changes. Nothing
is stored in state for the snapshot itself, unlike with the
`portainer_endpoint_snapshot` resource.

```shell
terraform init
terraform apply -var release=2              # snapshot through the action trigger
terraform apply -invoke action.portainer_endpoint_snapshot.refresh   # snapshot on demand
```

Requires Terraform >= 1.14; OpenTofu does not support actions (use the resource
there). Every action of the provider is described under
[docs/actions](../../docs/actions/), for example
[docs/actions/endpoint_snapshot.md](../../docs/actions/endpoint_snapshot.md).

## Inputs

| Name | Description | Type | Default | Required |
|------|-------------|------|---------|:--------:|
| portainer_url | Default Portainer URL | `string` | n/a | yes |
| portainer_api_key | Default Portainer Admin API Key | `string` | n/a | yes |
| environment_id | Environment (endpoint) to snapshot; null snapshots every environment | `number` | `null` | no |
| release | Release identifier; changing it triggers the snapshot | `string` | `"1"` | no |
