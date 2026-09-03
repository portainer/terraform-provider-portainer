# Data Source Documentation: `portainer_gitops_source_connection`

# portainer_gitops_source_connection
The `portainer_gitops_source_connection` data source asks Portainer to test whether it can reach and authenticate against a Git repository. It works both for a repository that is not stored yet — to validate credentials before creating a `portainer_gitops_source` — and for an existing source.

## Example Usage

### Validate credentials before creating a source

```hcl
data "portainer_gitops_source_connection" "check" {
  url             = "https://github.com/acme/infra.git"
  username        = "ci-bot"
  password        = var.git_token
  fail_on_error   = true
}

resource "portainer_gitops_source" "infra" {
  depends_on = [data.portainer_gitops_source_connection.check]

  url      = "https://github.com/acme/infra.git"
  username = "ci-bot"
  password = var.git_token
}
```

### Re-test a stored source

```hcl
data "portainer_gitops_source_connection" "infra" {
  source_id = portainer_gitops_source.infra.id
}

output "infra_reachable" {
  value = data.portainer_gitops_source_connection.infra.success
}
```

## Arguments Reference

| Name              | Type   | Required | Default | Description                                                                                                  |
|-------------------|--------|----------|---------|--------------------------------------------------------------------------------------------------------------|
| `source_id`       | number | ❌ no    | –       | Identifier of a stored source to test. Mutually exclusive with `url`; exactly one of the two is required.      |
| `url`             | string | ❌ no    | –       | Repository URL to test without storing a source. Mutually exclusive with `source_id`.                         |
| `username`        | string | ❌ no    | –       | Username used for the test.                                                                                    |
| `password`        | string | ❌ no    | –       | Password or personal access token paired with `username`. Stored in state as a sensitive value.                |
| `tls_skip_verify` | bool   | ❌ no    | `false` | Skip TLS certificate verification during the test.                                                             |
| `fail_on_error`   | bool   | ❌ no    | `false` | Whether a failing connection makes the data source itself fail instead of reporting the outcome.               |

## Attributes Reference

| Name      | Type   | Description                                                                 |
|-----------|--------|-----------------------------------------------------------------------------|
| `success` | bool   | Whether Portainer could reach and authenticate against the repository.       |
| `error`   | string | Reason the connection failed, empty on success.                              |
