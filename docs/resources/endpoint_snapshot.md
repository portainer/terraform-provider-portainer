# 📸 **Resource Documentation: `portainer_endpoints_snapshot`**

# portainer_endpoints_snapshot
The `portainer_endpoints_snapshot` resource allows you to trigger an immediate snapshot of environment(s) (also called endpoints) in Portainer.
## Example Usage
### Snapshot All Endpoints
```hcl
resource "portainer_endpoints_snapshot" "all" {}
```

### Snapshot Specific Endpoint
```hcl
resource "portainer_endpoints_snapshot" "specific" {
  endpoint_id = 3
}
```

## Lifecycle & Behavior
Snapshot is executed during:
```hcl
trraform apply
```

## Arguments Reference
| Name         | Type   | Required | Description                                                                 |
|--------------|--------|----------|-----------------------------------------------------------------------------|
| `endpoint_id`| number | 🚫 no    | ID of the environment (endpoint) to snapshot. If not set, all endpoints will be snapshotted. |

## Attributes Reference

| Name | Description                                                                 |
|------|-----------------------------------------------------------------------------|
| `id` | Always `"snapshot"` or `endpoint-{id}` depending on target.                |

