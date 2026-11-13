# Data Source Documentation: `portainer_observability_environment`

# portainer_observability_environment

> **Requires Portainer Business Edition 2.45 or later.** On Community Edition or an older server the read fails with an error that says so.

Reports whether observability is enabled for an environment, which provider serves it, and the non-secret connection details, from `GET /observability/environments/{id}`. With an [Observability policy](../resources/policy.md) in place, this is where the address of the OneUptime instance and the environment's cluster in it come from - useful for dashboards or links built by the same configuration.

## Example Usage

```hcl
data "portainer_observability_environment" "prod" {
  environment_id = 3
}

output "observability_url" {
  value = data.portainer_observability_environment.prod.enabled ? data.portainer_observability_environment.prod.url : null
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environment_id` | number | ✅ yes | ID of the environment to report on. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `enabled` | bool | Whether observability is enabled for the environment. |
| `provider_name` | string | Observability provider serving the environment (the API's `provider` field), empty when none does. |
| `url` | string | URL of the observability provider. |
| `project_id` | string | Project of the environment at the provider. |
| `cluster_id` | string | Provider's internal cluster identifier (OneUptime's, used for deep links); empty when Portainer could not resolve it. |
| `cluster_name` | string | Name of the cluster at the provider. |
