# Data Source Documentation: `portainer_licenses_info`

# portainer_licenses_info

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Summarises the licensing of the Portainer instance.

## Example Usage

```hcl
data "portainer_licenses_info" "current" {}

output "licence_overused" {
  value = data.portainer_licenses_info.current.overuse_started_at > 0
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `company` | string | Company the licence is issued to. |
| `enforced_at` | number | Unix timestamp enforcement begins at, zero when nothing is being enforced. |
| `expires_at` | number | Unix timestamp the licence expires at. |
| `nodes` | number | Nodes the licence covers. |
| `overuse_started_at` | number | Unix timestamp the instance started exceeding its node count, zero when it is within it. A non-zero value here is the one worth alerting on. |
| `type` | number | Portainer's numeric licence type. |
| `valid` | bool | Whether the licensing on this instance is currently valid. |

A non-zero `overuse_started_at` means the instance is running more nodes than the licence covers, which is the field worth alerting on.
