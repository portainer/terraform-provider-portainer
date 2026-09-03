# Data Source Documentation: `portainer_kubernetes_ingress_classes`

# portainer_kubernetes_ingress_classes
The `portainer_kubernetes_ingress_classes` data source lists the ingress classes available in a Kubernetes cluster, so an Ingress can reference a class that actually exists instead of hard-coding its name per environment.

Requires Portainer **2.45.0** or newer.

## Example Usage

```hcl
data "portainer_kubernetes_ingress_classes" "all" {
  environment_id = 1
}

output "ingress_class_names" {
  value = [for c in data.portainer_kubernetes_ingress_classes.all.ingress_classes : c.name]
}

# Fall back to the cluster default when nothing else is configured.
locals {
  ingress_class = coalesce(
    var.ingress_class,
    data.portainer_kubernetes_ingress_classes.all.default_ingress_class,
  )
}
```

## Arguments Reference

| Name             | Type   | Required | Description                                                              |
|------------------|--------|----------|--------------------------------------------------------------------------|
| `environment_id` | number | Yes      | Environment (endpoint) identifier of the Kubernetes environment to query. |

## Attributes Reference

| Name                    | Type   | Description                                                                                                     |
|-------------------------|--------|-----------------------------------------------------------------------------------------------------------------|
| `ingress_classes`       | list   | Ingress classes available in the cluster. Each entry has `name`, `controller`, `is_default` and `annotations`.   |
| `default_ingress_class` | string | Name of the cluster's default ingress class, empty when no class is marked as default.                          |
