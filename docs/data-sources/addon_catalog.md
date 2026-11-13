# Data Source Documentation: `portainer_addon_catalog`

# portainer_addon_catalog

> **Requires Portainer Business Edition 2.45.2 or later.** On Community Edition or an older server the read fails with an error that says so.

Reads the addon catalog document as published at the configured catalog URL: what each addon is, where its chart comes from and how Portainer serves it. Unlike `portainer_addons`, it carries no install state, records or environment data, and it is readable without being an administrator.

When the catalog source cannot be reached, Portainer serves its last good copy. With no copy at all it answers 503 and the read fails, rather than reporting an empty catalog. Use `portainer_addon_catalog_refresh` to drop the cached copy.

## Example Usage

```hcl
data "portainer_addon_catalog" "catalog" {}

output "addon_charts" {
  value = { for a in data.portainer_addon_catalog.catalog.addons : a.id => a.helm_chart_repository }
}
```

## Arguments Reference

This data source takes no arguments.

## Attributes Reference

| Name     | Type | Description |
|----------|------|-------------|
| `addons` | list | Catalog addons, each with `id`, `display_name`, `description`, `short_description`, `icon`, `path`, `service_url`, `health_check_path`, `helm_chart_repository`, `tls_verify` and `machine_access`. |
