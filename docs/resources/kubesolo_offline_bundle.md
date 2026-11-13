# Resource Documentation: `portainer_kubesolo_offline_bundle`

# portainer_kubesolo_offline_bundle

> **Requires Portainer Business Edition 3.0 or later.** On Community Edition or an older server the upload fails with an error that says so.

Uploads an offline tarball of the KubeSolo release bundled with Portainer, so Portainer can install KubeSolo on machines that cannot reach the internet. KubeSolo publishes one tarball per architecture with each release, named `kubesolo-<version>-linux-<arch>[-musl]-offline.tar.gz`; Portainer keeps an upload only if it belongs to its bundled release (see `portainer_kubesolo_versions`) and matches the release's `SHA256SUMS`.

The tarball is streamed from `file_path`, never read into memory. Replacing the file at the same path is detected through its SHA256 at plan time and uploads it again in place.

## Example Usage

```hcl
data "portainer_kubesolo_versions" "this" {}

resource "portainer_kubesolo_offline_bundle" "amd64" {
  name      = "kubesolo-${data.portainer_kubesolo_versions.this.bundled}-linux-amd64-offline.tar.gz"
  file_path = "${path.module}/kubesolo-${data.portainer_kubesolo_versions.this.bundled}-linux-amd64-offline.tar.gz"
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `name` | string | ✅ yes | File name of the tarball as published, for example `kubesolo-v1.2.3-linux-amd64-offline.tar.gz`. Changing it forces a new resource. |
| `file_path` | string | ✅ yes | Local path of the tarball to upload. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `id` | string | The tarball name. |
| `file_sha256` | string | SHA256 of the uploaded file, used to detect a replaced file. |
| `arch` | string | CPU architecture the bundle installs on, for example `amd64`. |
| `libc` | string | C library of the target machines: `glibc` or `musl`. |
| `size` | number | Size of the stored tarball in bytes. |

## Import

```bash
terraform import portainer_kubesolo_offline_bundle.amd64 kubesolo-v1.2.3-linux-amd64-offline.tar.gz
```

The import ID is the tarball name. Portainer does not expose the stored file's content or digest, so the first apply after an import uploads the file at `file_path` again.
