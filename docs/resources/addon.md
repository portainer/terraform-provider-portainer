# Resource Documentation: `portainer_addon`

# portainer_addon

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Installs, upgrades and uninstalls an addon from the Portainer add-on store. Portainer installs an addon as a Helm release, so the resource is essentially a managed release pinned to a catalog entry.

## Example Usage

```hcl
resource "portainer_addon" "portal" {
  addon_id = "portal-template"
  version  = "1.2.3"

  values = jsonencode({
    ingress = {
      enabled = true
      host    = "apps.example.com"
    }
    replicas = 2
  })
}
```

Installing from a private registry instead of the catalog's public chart reference:

```hcl
resource "portainer_addon" "portal" {
  addon_id          = "portal-template"
  registry_id       = portainer_registry.internal.id
  chart             = "portainer/charts/portal"
  image_registry_id = portainer_registry.internal.id
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `addon_id` | string | ✅ yes | Catalog identifier of the addon, for example `portal-template`. Changing it forces a new resource. |
| `chart` | string | ❌ no | Chart path within `registry_id`. Required when `registry_id` is set. |
| `version` | string | ❌ no | Chart version to install. Leave unset to resolve the latest published version. |
| `registry_id` | number | ❌ no | Portainer registry the chart is pulled from. Only honoured on a first install, so changing it forces a new resource. |
| `image_registry_id` | number | ❌ no | Portainer registry whose credentials the addon's pods pull their container image with. Can be changed on an upgrade. |
| `values` | string | ❌ no | Helm value overrides as a JSON object. Build it with `jsonencode({...})`. |
| `force_uninstall` | bool | ❌ no | Whether to force the uninstall on destroy, which lets Portainer remove a broken release. Defaults to `false`. |
| `prune_config_on_uninstall` | bool | ❌ no | Whether to also delete the addon's stored configuration on destroy. Defaults to `false`, so the configuration survives a reinstall. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `display_name` | string | Human-readable name of the addon. |
| `description` | string | Full catalog description. |
| `short_description` | string | One-line catalog tagline. |
| `icon` | string | Icon the Portainer UI shows for the addon. |
| `path` | string | Path the addon is served under in Portainer. |
| `enabled` | bool | Whether the addon is installed and switched on. Manage it with `portainer_addon_access`. |
| `lifecycle_status` | string | Helm release state: `installing`, `upgrading`, `installed`, `failed`, `unknown` or `uninstalling`. |
| `lifecycle_status_message` | string | Explanation of a failed install or upgrade. |
| `health_status` | string | Live probe result: `healthy`, `unhealthy`, `credential-invalid` or `unknown`. |
| `health_message` | string | Explanation of an unhealthy probe result. |
| `chart_version` | string | Chart version actually installed. Read this when `version` is left unset. |
| `chart_path` | string | Chart path recorded for the installed release. |
| `helm_chart_repository` | string | Public OCI chart reference published for the addon. |
| `available_versions` | list(string) | Published stable chart versions, newest first. |
| `upgrade_available` | bool | Whether a newer chart version has been published. |
| `version_check_error` | string | Why the available-version lookup failed. |

`lifecycle_status` and `health_status` are independent: an addon can report `installed` while its probe says `unhealthy`, and it can be `upgrading` while the previous version still serves traffic. A `credential-invalid` health status is what `portainer_addon_repair` fixes.

## Import

```bash
terraform import portainer_addon.portal portal-template
```

The import ID is the addon's catalog identifier.
