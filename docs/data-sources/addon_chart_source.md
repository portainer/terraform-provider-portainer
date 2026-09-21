# Data Source Documentation: `portainer_addon_chart_source`

# portainer_addon_chart_source

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Checks where an addon's chart would be pulled from, before an install is attempted. Use it to find out whether a registry is reachable and which versions it offers.

## Example Usage

```hcl
data "portainer_addon_chart_source" "portal" {
  addon_id    = "portal-template"
  registry_id = portainer_registry.internal.id
  chart       = "portainer/charts/portal"
}

resource "portainer_addon" "portal" {
  addon_id    = data.portainer_addon_chart_source.portal.addon_id
  registry_id = portainer_registry.internal.id
  chart       = "portainer/charts/portal"
  version     = data.portainer_addon_chart_source.portal.versions[0]
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `addon_id` | string | ✅ yes | Catalog identifier of the addon whose chart source is checked. |
| `registry_id` | number | ❌ no | Portainer registry to check against. Leave unset to check the catalog's public chart reference. |
| `chart` | string | ❌ no | Chart path within `registry_id` to check. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `reachable` | bool | Whether the registry answered. |
| `reason` | string | Why the source is unreachable, empty when `reachable` is true. |
| `tls_verify` | bool | Whether the certificate the registry served verified against the trust store. |
| `versions` | list(string) | Published stable versions of the chart, newest first. |

A registry that answered but holds no such chart is still `reachable`, with an empty `versions`. Indexing into `versions` in a configuration therefore needs the list checked first, or the plan fails on a registry that is up but has nothing published.
