# 🚀 **Resource Documentation: `portainer_portainer_kubernetes_application`**

# portainer_portainer_kubernetes_application

The `portainer_portainer_kubernetes_application` resource allows you to deploy one-off Kubernetes `Application` workloads into a specified namespace on a Kubernetes environment (endpoint) managed via Portainer.

---

## Example Usage
### Create Kubernetes Application from YAML
```hcl
resource "portainer_portainer_kubernetes_application" "example" {
  endpoint_id = 4
  namespace   = "default"
  manifest    = file("${path.module}/application.yaml")
}
```

## Lifecycle & Behavior
The Application is created via the Portainer Kubernetes API.

A change of `manifest` updates the existing object in place (a JSON merge patch), so a Deployment rolls out and a Service keeps its cluster IP; fields removed from the manifest are removed from the object. The object is deleted and created again only when it cannot be changed in place: another `metadata.name` or kind, another `namespace`, or a field Kubernetes does not allow to change (for example a Job's pod template or a Deployment's selector). `spec.replicas` is only sent when the manifest changes it, so an update does not undo an autoscaler or `portainer_kubernetes_deployment_scale`.

To update the Application (e.g. name, image), simply modify the manifest and re-apply:

```sh
terraform apply
```

To remove the application:
```sh
terraform destroy
```

**Drift detection.** On refresh, the fields set in `manifest` are compared with the live object: a value changed outside Terraform shows up in the plan, as the drifted fields with their live values, and the next apply puts the manifest back. Fields Kubernetes adds itself (status, defaults, `resourceVersion`) and canonical quantities (`500m` for `0.5`) are not drift. Fields the manifest does not set are not managed, so a key added outside Terraform is not reported. `spec.replicas` is not compared either, because a HorizontalPodAutoscaler or `portainer_kubernetes_deployment_scale` changes it; reverting it on every apply would fight them.

### Arguments Reference
| Name        | Type   | Required | Description                                                  |
|-------------|--------|----------|--------------------------------------------------------------|
| endpoint_id | int    | ✅ yes   | ID of the Portainer environment (Kubernetes cluster).        |
| namespace   | string | ✅ yes   | Kubernetes namespace where the Application should be created.    |
| manifest    | string | ✅ yes   | Kubernetes Application manifest (JSON or YAML as a string).      |

---

### Timeouts

`portainer_portainer_kubernetes_application` provides the following [Timeouts](https://developer.hashicorp.com/terraform/language/resources/syntax#operation-timeouts) configuration options:

| Operation | Default  | Description                              |
|-----------|----------|------------------------------------------|
| `create`  | 10 minutes | Time to wait for application creation.  |
| `update`  | 10 minutes | Time to wait for application update.    |
| `delete`  | 5 minutes  | Time to wait for application deletion.  |

#### Example

```hcl
resource "portainer_portainer_kubernetes_application" "example" {
  endpoint_id = 4
  namespace   = "default"
  manifest    = file("${path.module}/application.yaml")

  timeouts {
    create = "15m"
    update = "15m"
    delete = "10m"
  }
}
```

---

### Attributes Reference
| Name | Description                               |
|------|-------------------------------------------|
| `id` | 	ID in the format endpoint_id:namespace:application:name    |

## Import

Kubernetes application resources can be imported using the composite ID `endpointID:namespace:name`:

```shell
terraform import portainer_kubernetes_application.example 1:default:my-app
```

After import, set the `manifest` field in config to match the live object — Read only confirms the resource exists and restores identity fields, it does not reconstruct the manifest. If `manifest` is left blank after import, the next `terraform apply` treats the configured manifest as a change and updates the live object in place to match it.
