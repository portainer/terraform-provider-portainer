# 🔐 **Resource Documentation: `portainer_kubernetes_secret`**

# portainer_kubernetes_secret

The `portainer_kubernetes_secret` resource allows you to deploy one-off Kubernetes `Secret` workloads into a specified namespace on a Kubernetes environment (endpoint) managed via Portainer.

---

## Example Usage
### Create Kubernetes Secret from YAML
```hcl
resource "portainer_kubernetes_secret" "example" {
  endpoint_id = 4
  namespace   = "default"
  manifest    = file("${path.module}/secret.yaml")
}
```

## Lifecycle & Behavior
The Secret is created via the Portainer Kubernetes API.

A change of `manifest` updates the existing object in place (a JSON merge patch), so a Deployment rolls out and a Service keeps its cluster IP; fields removed from the manifest are removed from the object. The object is deleted and created again only when it cannot be changed in place: another `metadata.name` or kind, another `namespace`, or a field Kubernetes does not allow to change (for example a Job's pod template or a Deployment's selector).

To update the Secret (e.g. name, image), simply modify the manifest and re-apply:

```sh
terraform apply
```

To remove the secret:
```sh
terraform destroy
```

**Drift detection.** On refresh, the fields set in `manifest` are compared with the live object: a value changed outside Terraform shows up in the plan, as the drifted fields with their live values, and the next apply puts the manifest back. Fields Kubernetes adds itself (status, defaults, `resourceVersion`) and canonical quantities (`500m` for `0.5`) are not drift. Fields the manifest does not set are not managed, so a key added outside Terraform is not reported. `stringData` is compared with the base64 `data` Kubernetes stores. The manifest is sensitive, so the plan does not show the secret values.

### Arguments Reference
| Name        | Type   | Required | Description                                                  |
|-------------|--------|----------|--------------------------------------------------------------|
| endpoint_id | int    | ✅ yes   | ID of the Portainer environment (Kubernetes cluster).        |
| namespace   | string | ✅ yes   | Kubernetes namespace where the Secret should be created.    |
| manifest    | string | ✅ yes   | Kubernetes Secret manifest (JSON or YAML as a string).      |

---

### Attributes Reference
| Name | Description                               |
|------|-------------------------------------------|
| `id` | 	ID in the format endpoint_id:namespace:secret:name    |

## Import

Kubernetes Secret resources can be imported using the composite ID `endpointID:namespace:name`:

```shell
terraform import portainer_kubernetes_secret.example 1:default:my-secret
```

After import, set the `manifest` field in config to match the live object — Read only confirms the resource exists and restores identity fields, it does not reconstruct the manifest. If `manifest` is left blank after import, the next `terraform apply` treats the configured manifest as a change and updates the live object in place to match it.
