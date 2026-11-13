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

### Create a Kubernetes vulnerability scanning policy

Requires Portainer Business Edition 3.0 or later. Deploys trivy-operator to every Kubernetes environment of the attached groups, which scans the images of running workloads on a schedule.

```hcl
resource "portainer_policy" "vulnerability_scanning" {
  name               = "Image scanning"
  environment_type   = "kubernetes"
  policy_type        = "vulnerability-scanning-k8s"
  environment_groups = [2]

  # Copy of an existing policy's data, e.g. from data.portainer_policy.
  data = var.vulnerability_scanning_data
}
```

### Create a Kubernetes automatic namespace provisioning policy

Requires Portainer Business Edition 3.0 or later (and Edge Agent 3.0 or later on the environments). Gives each eligible user a personal namespace named after a pattern containing `{{user_id}}`, with a namespace-scoped role inside it.

```hcl
resource "portainer_policy" "user_namespaces" {
  name               = "Personal namespaces"
  environment_type   = "kubernetes"
  policy_type        = "automatic-namespace-provisioning-k8s"
  environment_groups = [2]

  data = var.namespace_provisioning_data
}
```

The structure of `data` for these two types is not part of Portainer's published API specification. Create the policy once in the Portainer UI and read its `data` with the `portainer_policy` data source to get the exact keys, then manage it with this resource (or import the UI-created policy).

## Arguments Reference

| Name                 | Type   | Required | Description                                                                                              |
|----------------------|--------|----------|----------------------------------------------------------------------------------------------------------|
| `name`               | string | Yes      | Name of the policy.                                                                                      |
| `environment_type`   | string | Yes      | Environment type: `kubernetes`, `docker`, `podman`, or `swarm`.                                          |
| `policy_type`        | string | Yes      | Policy type. One of: `rbac-k8s`, `rbac-docker`, `security-k8s`, `security-docker`, `setup-k8s`, `setup-docker`, `registry-k8s`, `registry-docker`, `change-confirmation`, `cleanup-docker`, `observability-k8s`, `pod-security-standards-k8s`, `network-security-k8s`, and (Portainer Business Edition 3.0 or later) `vulnerability-scanning-k8s`, `automatic-namespace-provisioning-k8s`. Changing this forces a new resource. |
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
