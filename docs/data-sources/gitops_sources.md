# Data Source Documentation: `portainer_gitops_sources`

# portainer_gitops_sources
The `portainer_gitops_sources` data source lists Portainer's GitOps sources with optional filtering and paging, together with the cluster-wide status summary.

## Example Usage

```hcl
data "portainer_gitops_sources" "all" {
  search = "acme"
  order  = "asc"
  sort   = "name"
}

output "broken_sources" {
  value = [
    for s in data.portainer_gitops_sources.all.sources : "${s.name}: ${s.status_error}"
    if s.status == "error"
  ]
}

output "sources_needing_attention" {
  value = data.portainer_gitops_sources.all.summary[0].error
}
```

## Arguments Reference

| Name     | Type   | Required | Description                                                                          |
|----------|--------|----------|--------------------------------------------------------------------------------------|
| `search` | string | ❌ no    | Free-text filter matched against the source name and URL.                             |
| `sort`   | string | ❌ no    | Field to sort by, as accepted by the Portainer API (for example `name`).              |
| `order`  | string | ❌ no    | Sort direction, `asc` or `desc`.                                                       |
| `start`  | number | ❌ no    | Zero-based index of the first result, for paging.                                      |
| `limit`  | number | ❌ no    | Maximum number of results to return.                                                   |
| `status` | string | ❌ no    | Only return sources in this status: `healthy`, `syncing`, `error`, `paused`, `unknown`.|
| `type`   | string | ❌ no    | Only return sources of this type: `git`, `helm` or `oci`.                              |

## Attributes Reference

| Name      | Type | Description                                                                                                                                                        |
|-----------|------|--------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| `sources` | list | Sources matching the query. Each entry has `id`, `name`, `url`, `type`, `status`, `status_error`, `interval`, `last_sync`, `used_by` and `environments`.             |
| `summary` | list | Exactly one element with the counts `healthy`, `syncing`, `error`, `paused` and `unknown`.                                                                          |

The summary comes from its own Portainer endpoint and is **not** filtered by the arguments above — it always describes every source.
