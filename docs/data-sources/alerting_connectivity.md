# Data Source Documentation: `portainer_alerting_connectivity`

# portainer_alerting_connectivity

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Tests whether Portainer can reach an Alertmanager.

## Example Usage

```hcl
data "portainer_alerting_connectivity" "main" {
  url = "http://alertmanager.example.com:9093"
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `fail_on_error` | bool | ❌ no | Whether an unreachable Alertmanager fails the plan. Defaults to `true`, which is the point of a pre-flight check; set it to `false` to branch on `reachable` instead. |
| `url` | string | ✅ yes | URL of the Alertmanager to test. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `details` | string | The whole response as Portainer returns it, encoded as JSON. Portainer does not describe its shape in the API specification, so decode it with `jsondecode()` if you need more than `reachable`. |
| `error` | string | What went wrong, empty when the Alertmanager answered. |
| `reachable` | bool | Whether Portainer could reach the Alertmanager. |

`fail_on_error` defaults to `true`. Set it to `false` to read `reachable` and `error` instead of stopping the plan.

Portainer does not describe this response's shape in its API specification, so `details` carries it as JSON.
