# ⚙️🧭 **Resource Documentation: `portainer_kubernetes_cronjob`**

# portainer_kubernetes_cronjob

The `portainer_kubernetes_cronjob` resource allows you to deploy one-off Kubernetes `Cronjob` workloads into a specified namespace on a Kubernetes environment (endpoint) managed via Portainer.

---

## Example Usage
### Create Kubernetes Cronjob from YAML
```hcl
resource "portainer_kubernetes_cronjob" "example" {
  endpoint_id = 4
  namespace   = "default"
  manifest    = file("${path.module}/cronjob.yaml")
}
```

## Lifecycle & Behavior
The Cronjob is created via the Portainer Kubernetes API.

A change of `manifest` updates the existing object in place (a JSON merge patch), so a Deployment rolls out and a Service keeps its cluster IP; fields removed from the manifest are removed from the object. The object is deleted and created again only when it cannot be changed in place: another `metadata.name` or kind, another `namespace`, or a field Kubernetes does not allow to change (for example a Job's pod template or a Deployment's selector).

To update the Cronjob (e.g. name, image), simply modify the manifest and re-apply:

```sh
terraform apply
```

To remove the cronjob:
```sh
terraform destroy
```

**Drift detection.** On refresh, the fields set in `manifest` are compared with the live object: a value changed outside Terraform shows up in the plan, as the drifted fields with their live values, and the next apply puts the manifest back. Fields Kubernetes adds itself (status, defaults, `resourceVersion`) and canonical quantities (`500m` for `0.5`) are not drift. Fields the manifest does not set are not managed, so a key added outside Terraform is not reported.

### Arguments Reference
| Name        | Type   | Required | Description                                                  |
|-------------|--------|----------|--------------------------------------------------------------|
| endpoint_id | int    | ✅ yes   | ID of the Portainer environment (Kubernetes cluster).        |
| namespace   | string | ✅ yes   | Kubernetes namespace where the CronJob should be created.    |
| manifest    | string | ✅ yes   | Kubernetes CronJob manifest (JSON or YAML as a string).      |

---

### Attributes Reference
| Name | Description                               |
|------|-------------------------------------------|
| `id` | 	ID in the format endpoint_id:namespace:cronjob:name    |

## Import

Kubernetes CronJob resources can be imported using the composite ID `endpointID:namespace:name`:

```shell
terraform import portainer_kubernetes_cronjob.example 1:default:my-cronjob
```

After import, set the `manifest` field in config to match the live object — Read only confirms the resource exists and restores identity fields, it does not reconstruct the manifest. If `manifest` is left blank after import, the next `terraform apply` treats the configured manifest as a change and updates the live object in place to match it.
