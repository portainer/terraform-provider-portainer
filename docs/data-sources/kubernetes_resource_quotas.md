# Data Source Documentation: `portainer_kubernetes_resource_quotas`

# portainer_kubernetes_resource_quotas
The `portainer_kubernetes_resource_quotas` data source lists the resource quotas defined in a Kubernetes namespace, together with their current usage. Portainer 2.45 surfaces every quota of a namespace (its UI namespace view was previously limited to one), and this data source exposes the same information.

Requires Portainer **2.45.0** or newer.

## Example Usage

```hcl
data "portainer_kubernetes_resource_quotas" "default" {
  environment_id = 1
  namespace      = "default"
}

output "quota_limits" {
  value = {
    for q in data.portainer_kubernetes_resource_quotas.default.resource_quotas : q.name => q.hard
  }
}

output "cpu_headroom" {
  value = {
    for q in data.portainer_kubernetes_resource_quotas.default.resource_quotas :
    q.name => "${lookup(q.used, "limits.cpu", "0")} of ${lookup(q.hard, "limits.cpu", "unlimited")}"
  }
}
```

## Arguments Reference

| Name             | Type   | Required | Description                                                               |
|------------------|--------|----------|---------------------------------------------------------------------------|
| `environment_id` | number | Yes      | Environment (endpoint) identifier of the Kubernetes environment to query.  |
| `namespace`      | string | Yes      | Namespace whose resource quotas are listed.                                |

## Attributes Reference

| Name              | Type | Description                                                                                                                        |
|-------------------|------|--------------------------------------------------------------------------------------------------------------------------------------|
| `resource_quotas` | list | Resource quotas defined in the namespace. Each entry has `name`, `namespace`, `hard`, `used`, `scopes` and `creation_timestamp`.      |

`hard` and `used` are maps of Kubernetes resource names (`limits.cpu`, `requests.memory`, `pods`, …) to quantity strings such as `2`, `500m` or `1Gi` — compare them as strings, or parse them, but do not assume plain numbers. `hard` reports the limit the quota controller actually enforces, falling back to the spec before the controller has observed the quota.
