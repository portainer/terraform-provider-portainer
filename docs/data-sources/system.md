# Data Source Documentation: `portainer_system`

# portainer_system
The `portainer_system` data source describes the Portainer instance itself: its version, edition, scale and whether it has been initialised. It folds Portainer's four system endpoints plus the administrator check into one lookup.

Its most useful attribute is `server_version`, which lets a configuration gate resources on the Portainer release — several resources in this provider need 2.45 or newer.

## Example Usage

```hcl
data "portainer_system" "this" {}

# Only manage a deployment's scale where the native API exists.
resource "portainer_kubernetes_deployment_scale" "web" {
  count = tonumber(split(".", data.portainer_system.this.server_version)[1]) >= 45 ? 1 : 0

  environment_id  = 1
  namespace       = "prod"
  deployment_name = "web"
  replicas        = 3
}

output "portainer_scale" {
  value = "${data.portainer_system.this.agents} agents, ${data.portainer_system.this.edge_agents} edge agents, ${data.portainer_system.this.nodes} nodes"
}
```

## Arguments Reference

This data source takes no arguments.

## Attributes Reference

| Name                | Type   | Description                                                                                        |
|---------------------|--------|----------------------------------------------------------------------------------------------------|
| `instance_id`       | string | Unique identifier of this Portainer instance.                                                       |
| `version`           | string | Version reported by the status endpoint.                                                            |
| `server_version`    | string | Server version reported by the version endpoint.                                                     |
| `server_edition`    | string | Portainer edition, `CE` or `BE`.                                                                     |
| `database_version`  | string | Schema version of Portainer's database.                                                              |
| `latest_version`    | string | Newest version available upstream, empty when the update service cannot be reached.                  |
| `update_available`  | bool   | Whether a newer version than the running one is available.                                           |
| `version_support`   | string | Support status of the running version, such as `STS` or `LTS`.                                       |
| `agents`            | number | Number of standard agents connected.                                                                 |
| `edge_agents`       | number | Number of Edge agents connected.                                                                     |
| `platform`          | string | Container platform Portainer itself runs on.                                                         |
| `nodes`             | number | Total nodes across every environment, which is what Portainer licences against.                      |
| `admin_initialized` | bool   | Whether an administrator account exists. False means the instance is still awaiting initial setup.   |

`/status` is Portainer's deprecated alias of `/system/status` and returns the same object, so only the current path is called.
