# Resource Documentation: `portainer_policy`

# portainer_policy
The `portainer_policy` resource manages Fleet Governance Policies in Portainer 2.39+. Policies allow administrators to enforce configuration, security, RBAC, and registry rules across groups of environments.

## Example Usage

### Create a Kubernetes RBAC policy

```hcl
resource "portainer_policy" "k8s_rbac" {
  name               = "Production RBAC"
  environment_type   = "kubernetes"
  policy_type        = "rbac-k8s"
  environment_groups = [1, 2]
  allow_override     = false

  data = jsonencode({
    roles = ["viewer"]
  })
}
```

### Create a Docker security policy

```hcl
resource "portainer_policy" "docker_security" {
  name               = "Docker Security Baseline"
  environment_type   = "docker"
  policy_type        = "security-docker"
  environment_groups = [3]
  allow_override     = true

  data = jsonencode({
    disablePrivilegedMode = true
  })
}
```

## Arguments Reference

| Name                 | Type   | Required | Description                                                                                              |
|----------------------|--------|----------|----------------------------------------------------------------------------------------------------------|
| `name`               | string | Yes      | Name of the policy.                                                                                      |
| `environment_type`   | string | Yes      | Environment type: `kubernetes`, `docker`, `podman`, or `swarm`.                                          |
| `policy_type`        | string | Yes      | Policy type. One of: `rbac-k8s`, `rbac-docker`, `security-k8s`, `security-docker`, `setup-k8s`, `setup-docker`, `registry-k8s`, `registry-docker`. Changing this forces a new resource. |
| `environment_groups` | list   | No       | List of environment group IDs to which the policy applies.                                               |
| `data`               | string | No       | Policy data as a JSON-encoded string. Structure depends on `policy_type`.                                |
| `allow_override`     | bool   | No       | Whether environments can override this policy. Defaults to `false`.                                      |

## Attributes Reference

| Name         | Type   | Description                              |
|--------------|--------|------------------------------------------|
| `id`         | string | ID of the policy.                        |
| `created_at` | string | Timestamp when the policy was created.   |
| `updated_at` | string | Timestamp when the policy was updated.   |

## Import

Policies can be imported using their numeric ID:

```shell
terraform import portainer_policy.example 42
```
