# Data Source Documentation: `portainer_omni_machine_logs`

# portainer_omni_machine_logs

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Fetches the logs of an Omni machine.

## Example Usage

```hcl
data "portainer_omni_machine_logs" "cp_1" {
  credential_id = portainer_cloud_credentials.omni.id
  machine_name  = "cp-1"
}

output "machine_logs" {
  value = data.portainer_omni_machine_logs.cp_1.logs
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `credential_id` | number | ✅ yes | Portainer cloud credential holding the Omni endpoint and service account key. |
| `machine_name` | string | ✅ yes | Name of the machine to fetch logs for. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `logs` | string | The machine's logs as Omni returns them. |

Logs change on every read, so anything that depends on this value will show a difference on every plan. Use it for an output or a local file, not as an input to another resource.
