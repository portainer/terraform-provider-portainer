# Data Source Documentation: `portainer_registry_connection`

# portainer_registry_connection
The `portainer_registry_connection` data source asks Portainer to test whether it can reach and authenticate against a container registry, before a `portainer_registry` is created with the same credentials.

## Example Usage

```hcl
data "portainer_registry_connection" "harbor" {
  url           = "registry.example.com"
  type          = 3
  username      = "robot$ci"
  password      = var.registry_password
  tls           = true
  fail_on_error = true
}
```

## Arguments Reference

| Name            | Type   | Required | Default | Description                                                                                  |
|-----------------|--------|----------|---------|----------------------------------------------------------------------------------------------|
| `url`           | string | ✅ yes   | –       | URL of the registry, without a scheme for most types.                                          |
| `type`          | number | ✅ yes   | –       | Registry type: 1 = Quay, 2 = Azure, 3 = Custom, 4 = GitLab, 5 = ProGet, 6 = DockerHub, 7 = ECR. |
| `username`      | string | ❌ no    | –       | Username used for the test.                                                                    |
| `password`      | string | ❌ no    | –       | Password or token paired with `username`. Stored in state as a sensitive value.                |
| `tls`           | bool   | ❌ no    | `false` | Whether the registry is contacted over TLS.                                                    |
| `fail_on_error` | bool   | ❌ no    | `false` | Whether an unreachable registry makes the data source itself fail.                             |

## Attributes Reference

| Name      | Type   | Description                                                            |
|-----------|--------|------------------------------------------------------------------------|
| `success` | bool   | Whether Portainer could reach and authenticate against the registry.   |
| `message` | string | Message describing the outcome, carrying the reason on failure.        |
