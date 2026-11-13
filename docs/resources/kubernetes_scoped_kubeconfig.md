# Resource Documentation: `portainer_kubernetes_scoped_kubeconfig`

# portainer_kubernetes_scoped_kubeconfig

> **Requires Portainer Business Edition 2.45.2 or later** (also in 3.0). On Community Edition or an older server the create fails with an error that says so.

Issues a scoped kubeconfig: a **read-only** kubeconfig for one Kubernetes environment, limited to chosen namespaces within the caller's own access, that expires after at most 7 days. Use it to hand a CI job, an agent or a colleague read access to a few namespaces without sharing your own credentials. Its use is recorded in Portainer's activity log as `<user> (via scoped kubeconfig <id>)`.

## Example Usage

```hcl
resource "portainer_kubernetes_scoped_kubeconfig" "ci" {
  environment_id  = 4
  namespaces      = ["team-a", "team-b"]
  include_secrets = false
  expiry_seconds  = 86400 # 1 day
  description     = "CI read access"
}

resource "local_sensitive_file" "kubeconfig" {
  content  = portainer_kubernetes_scoped_kubeconfig.ci.kubeconfig
  filename = "${path.module}/ci.kubeconfig"
}
```

## Lifecycle & Behavior

* Portainer cannot change an issued kubeconfig, so **every argument forces a new one**.
* **The credential is returned once**, by the call that issues it, and never again. The provider keeps it in state as the sensitive `kubeconfig` attribute, since that is the only way to hand it on; protect the state accordingly (it is a working credential until it expires). A write-only argument does not fit: the value comes from Portainer, not from the configuration.
* When the kubeconfig has **expired** (or was revoked outside Terraform), a refresh removes it from state and the next apply issues a new one, with a new `kubeconfig`. Run `terraform apply` on a schedule shorter than `expiry_seconds` to rotate it.
* Destroying the resource **revokes** the kubeconfig: its service account and role bindings are removed and the credential stops working immediately. When Portainer revokes the credential but cannot remove the Kubernetes objects (for example because the environment is unreachable), the destroy succeeds with a warning; remove the leftovers by hand.
* `include_secrets` is honoured only in namespaces where you can view Secrets. The `portainer` and `portainer-addon-*` namespaces are covered only when named in `namespaces`, and never with Secrets.

## Arguments Reference

| Name              | Type         | Required    | Description |
|-------------------|--------------|-------------|-------------|
| `environment_id`  | int          | ✅ yes       | ID of the Kubernetes environment. Forces a new kubeconfig. |
| `expiry_seconds`  | int          | ✅ yes       | Lifetime in seconds, 1 to 604800 (7 days). Forces a new kubeconfig. |
| `namespaces`      | list(string) | 🚫 optional | Namespaces covered; unset covers every namespace you can view. Forces a new kubeconfig. |
| `include_secrets` | bool         | 🚫 optional | Whether Secrets can be read. Defaults to `false`. Forces a new kubeconfig. |
| `description`     | string       | 🚫 optional | Description shown in Portainer. Forces a new kubeconfig. |

## Attributes Reference

| Name                   | Type         | Description |
|------------------------|--------------|-------------|
| `id`                   | string       | `<environment_id>:<scoped_kubeconfig_id>`. |
| `scoped_kubeconfig_id` | int          | Portainer identifier of the kubeconfig. |
| `kubeconfig`           | string       | **Sensitive.** The kubeconfig document; empty after an import. |
| `service_account`      | string       | Kubernetes service account the kubeconfig authenticates as. |
| `covered_namespaces`   | list(string) | Namespaces the kubeconfig covers, as reported by Portainer. |
| `active`               | bool         | Whether the kubeconfig works; an inactive one is dropped on refresh. |
| `created_at`           | int          | Issue time, Unix timestamp. |
| `expires_at`           | int          | Expiry time, Unix timestamp. |

## Import

Scoped kubeconfigs can be imported with `<environment_id>:<scoped_kubeconfig_id>`:

```shell
terraform import portainer_kubernetes_scoped_kubeconfig.ci 4:17
```

The import reads `namespaces`, `include_secrets`, `description` and `expiry_seconds` back, so a configuration that matches does not plan a replacement (for a kubeconfig issued without `namespaces`, the covered namespaces are imported; set `namespaces` to them). The credential itself cannot be recovered: `kubeconfig` stays empty.
