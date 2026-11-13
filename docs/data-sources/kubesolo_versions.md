# Data Source Documentation: `portainer_kubesolo_versions`

# portainer_kubesolo_versions

> **Requires Portainer Business Edition 3.0 or later.** On Community Edition or an older server the read fails with an error that says so.

Lists the KubeSolo releases Portainer can install: the release it was built with (bundled), and the releases published on GitHub from the minimum supported one, newest first. When GitHub cannot be reached, only the bundled release is listed and `github_error` says why.

Portainer's own KubeSolo install over SSH (`/kubesolo/tests` and `/kubesolo/install`) is not available in the provider: it is an asynchronous background job over many machines, with one connection test held per Portainer instance and host keys confirmed in between, which does not fit a Terraform resource. Install KubeSolo with the machine's own provisioning instead (see the `kubesolo_edge` example, which feeds the edge ID and key of a `portainer_environment` into cloud-init).

## Example Usage

```hcl
data "portainer_kubesolo_versions" "this" {}

output "kubesolo_bundled" {
  value = data.portainer_kubesolo_versions.this.bundled
}
```

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `bundled` | string | Release Portainer was built with, installed by default and on offline machines. |
| `minimum` | string | Oldest release Portainer installs. |
| `versions` | list(string) | Releases Portainer can install, newest first. |
| `github_error` | string | Why the GitHub releases could not be listed, empty when they could. |
