# Data Source Documentation: `portainer_edge_configuration_files`

# portainer_edge_configuration_files

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Reads the files of an edge configuration.

## Example Usage

```hcl
data "portainer_edge_configuration_files" "shops" {
  edge_configuration_id = portainer_edge_configurations.shops.id
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `edge_configuration_id` | number | ✅ yes | Identifier of the edge configuration whose files are read. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `files` | string | The configuration's files as Portainer returns them. This is the payload itself, not a JSON document, so it is carried through unchanged. |

The endpoint hands back the payload itself rather than a JSON document, so it is carried through unchanged.
