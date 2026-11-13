# Data Source Documentation: `portainer_kubesolo_offline_bundles`

# portainer_kubesolo_offline_bundles

> **Requires Portainer Business Edition 3.0 or later.** On Community Edition or an older server the read fails with an error that says so.

Lists the offline tarballs of the bundled KubeSolo release uploaded to Portainer for machines that cannot reach the internet. Upload them with `portainer_kubesolo_offline_bundle`.

Portainer's own KubeSolo install over SSH (`/kubesolo/tests` and `/kubesolo/install`), which is what consumes these bundles, is not available in the provider: it is an asynchronous background job over many machines with host keys confirmed in between, which does not fit a Terraform resource. Run it from the Portainer UI, or install KubeSolo with the machine's own provisioning (see the `kubesolo_edge` example).

## Example Usage

```hcl
data "portainer_kubesolo_offline_bundles" "all" {}

output "offline_architectures" {
  value = [for b in data.portainer_kubesolo_offline_bundles.all.bundles : "${b.arch}/${b.libc}"]
}
```

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `bundles` | list(object) | The uploaded tarballs. |

### `bundles`

| Name | Type | Description |
|------|------|-------------|
| `name` | string | File name of the tarball. |
| `arch` | string | CPU architecture, for example `amd64`. |
| `libc` | string | C library of the target machines: `glibc` or `musl`. |
| `size` | number | Size in bytes. |
