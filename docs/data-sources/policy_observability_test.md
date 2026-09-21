# Data Source Documentation: `portainer_policy_observability_test`

# portainer_policy_observability_test

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Checks whether Portainer can reach a OneUptime instance with the given details, before a policy is pointed at it.

## Example Usage

```hcl
data "portainer_policy_observability_test" "oneuptime" {
  one_uptime_url = "https://oneuptime.example.com"
  api_key        = var.oneuptime_api_key
  project_id     = var.oneuptime_project
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `api_key` | string | ❌ no | OneUptime API key to authenticate with. |
| `auto_provision_ingestion_key` | bool | ❌ no | Whether the test should provision an ingestion key as part of the check. |
| `auto_rotate_key` | bool | ❌ no | Whether the test should rotate the key as part of the check. |
| `fail_on_error` | bool | ❌ no | Whether a failed connection fails the plan. Defaults to `true`, which is the point of a pre-flight check; set it to `false` to branch on `success` instead. |
| `one_uptime_url` | string | ✅ yes | URL of the OneUptime instance to test against. |
| `policy_id` | number | ❌ no | Existing policy to test the connection for, when there is one. |
| `project_id` | string | ❌ no | OneUptime project the policy would report into. |
| `tls_skip_verify` | bool | ❌ no | Whether to skip verification of the OneUptime server's TLS certificate. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `message` | string | What Portainer reported about the attempt. |
| `success` | bool | Whether Portainer could reach OneUptime with these details. |

The credentials are omitted from the request when unset, so a blank key is never sent and rejected for the wrong reason.

`fail_on_error` defaults to `true`, which is the point of a pre-flight check. Set it to `false` to read `success` and `message` and branch on them instead.
