# Data Source Documentation: `portainer_kubesolo_config`

# portainer_kubesolo_config

> **Requires Portainer Business Edition 3.0 or later.** On Community Edition or an older server the read fails with an error that says so.

Reads the version and configuration of KubeSolo running in a KubeSolo environment. Portainer takes them from the environment's latest snapshot: an async Edge Agent sends it, and for other agents Portainer reads it through the agent. The read fails while the environment has no snapshot with KubeSolo's state yet, for example right after the machine was provisioned.

The configuration includes KubeSolo's secrets, such as the Edge key, so `config_json` is sensitive.

Portainer's own KubeSolo install over SSH (`/kubesolo/tests` and `/kubesolo/install`) is not available in the provider: it is an asynchronous background job, which does not fit a Terraform resource. See the `kubesolo_edge` example for installing KubeSolo against a `portainer_environment` through cloud-init.

## Example Usage

```hcl
data "portainer_kubesolo_config" "edge" {
  environment_id = portainer_environment.kubesolo.id
}

output "kubesolo_version" {
  value = data.portainer_kubesolo_config.edge.version
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `environment_id` | number | ✅ yes | ID of the KubeSolo environment. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `api` | string | Whether the agent could reach the KubeSolo API: `available`, `notConfigured` or `unreachable`. |
| `version` | string | KubeSolo version. |
| `config_json` | string | KubeSolo configuration as a JSON object (sensitive). Decode it with `jsondecode`. Empty when it could not be read. |
| `error` | string | Why the configuration could not be read. |
| `read_at` | number | When the snapshot carrying the configuration was taken, as a Unix timestamp. |
