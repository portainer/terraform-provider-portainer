# Resource Documentation: `portainer_kubernetes_pod_security_rule`

# portainer_kubernetes_pod_security_rule

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Manages the OPA-backed pod security rule of a Kubernetes environment.

Portainer always has a rule for a cluster - its own UI creates a default one the first time the page is opened - so this resource adopts and configures that rule rather than creating one.

## Example Usage

```hcl
resource "portainer_kubernetes_pod_security_rule" "baseline" {
  endpoint_id = portainer_environment.prod.id
  enabled     = true

  privileged_containers      = true
  allow_privilege_escalation = true
  host_namespaces            = true
  read_only_root_filesystem  = true

  capabilities {
    allowed       = ["NET_BIND_SERVICE"]
    required_drop = ["ALL"]
  }

  host_ports {
    host_network = false
    min          = 30000
    max          = 32767
  }

  host_filesystem {
    allowed_path {
      path_prefix = "/var/log"
      readonly    = true
    }
  }

  users {
    run_as_user {
      type = "MustRunAs"

      id_range {
        min = 1000
        max = 2000
      }
    }
  }
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `endpoint_id` | number | ✅ yes | Kubernetes environment the rule applies to. Changing it forces a new resource. |
| `enabled` | bool | ❌ no | Whether the rule is enforced at all. Defaults to `true`. |
| `privileged_containers` | bool | ❌ no | Block privileged containers. |
| `allow_privilege_escalation` | bool | ❌ no | Block containers that may escalate their privileges. |
| `host_namespaces` | bool | ❌ no | Block pods that join the host's namespaces. |
| `read_only_root_filesystem` | bool | ❌ no | Require a read-only root filesystem. |
| `restrict_default_namespace` | bool | ❌ no | Stop workloads being deployed into the `default` namespace. |
| `restrict_secrets` | bool | ❌ no | Restrict access to secrets. |
| `allow_proc_mount` | block | ❌ no | Permitted proc mount types. At most one. |
| `allow_flex_volumes` | block | ❌ no | Permitted FlexVolume drivers. At most one. |
| `app_armor` | block | ❌ no | Permitted AppArmor profiles. At most one. |
| `capabilities` | block | ❌ no | Permitted and required-drop Linux capabilities. At most one. |
| `forbidden_sysctls` | block | ❌ no | Sysctls a pod may not set. At most one. |
| `host_filesystem` | block | ❌ no | Host paths a pod may mount. At most one. |
| `host_ports` | block | ❌ no | Host ports a pod may bind. At most one. |
| `sec_comp` | block | ❌ no | Permitted seccomp profiles. At most one. |
| `selinux` | block | ❌ no | Permitted SELinux contexts. At most one. |
| `volume_types` | block | ❌ no | Permitted volume types. At most one. |
| `users` | block | ❌ no | Permitted user and group identities. At most one. |

Every block takes an `enabled` flag of its own, defaulting to `true`, so declaring a block is enough to switch that section on.

### Block contents

| Block | Fields |
|-------|--------|
| `allow_proc_mount` | `enabled`, `proc_mount_type` |
| `allow_flex_volumes` | `enabled`, `allowed_volumes` (list) |
| `app_armor` | `enabled`, `types` (list) |
| `capabilities` | `enabled`, `allowed` (list), `required_drop` (list) |
| `forbidden_sysctls` | `enabled`, `sysctls` (list) |
| `host_filesystem` | `enabled`, `allowed_path` blocks of `path_prefix` and `readonly` |
| `host_ports` | `enabled`, `host_network`, `min`, `max` |
| `sec_comp` | `enabled`, `types` (list) |
| `selinux` | `enabled`, `allowed_context` blocks of `user`, `role`, `type`, `level` |
| `volume_types` | `enabled`, `allowed_types` (list) |
| `users` | `enabled`, and `run_as_user`, `run_as_group`, `fs_groups`, `supplemental_groups` blocks, each with a `type` and repeatable `id_range` blocks of `min` and `max` |

## Lifecycle & Behavior

**Only the switches are read back.** The individual sections are lists of permitted values that Portainer normalises and reorders, so reading them into state would turn a stable configuration into a churning plan. What they contain is driven entirely by the configuration; `enabled` and the six plain switches are read back and will show drift.

**Destroying switches the rule off.** Portainer has no endpoint to remove a rule - a cluster always has one - so `enabled` is set to `false` on destroy. That is the only "absent" state available, and leaving an enforced policy behind for a rule Terraform no longer manages would be worse.

## Import

```bash
terraform import portainer_kubernetes_pod_security_rule.baseline 5
```

The import ID is the environment identifier.
