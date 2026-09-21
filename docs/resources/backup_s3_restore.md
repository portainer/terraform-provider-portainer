# Resource Documentation: `portainer_backup_s3_restore`

# portainer_backup_s3_restore

> **Business Edition only.** This endpoint does not exist in Portainer CE.


Restores a Portainer instance from an archive in an S3 bucket.

## Example Usage

```hcl
resource "portainer_backup_s3_restore" "recover" {
  bucket_name       = "acme-backups"
  filename          = "portainer-2026-09-15.tar.gz"
  region            = "eu-central-1"
  access_key_id     = var.aws_access_key_id
  secret_access_key = var.aws_secret_access_key
  password          = var.backup_archive_password
}
```

## Arguments Reference

| Name | Type | Required | Description |
|------|------|----------|-------------|
| `bucket_name` | string | ✅ yes | Bucket holding the archive. |
| `filename` | string | ✅ yes | Name of the archive in the bucket. |
| `region` | string | ✅ yes | AWS region of the bucket. |
| `access_key_id` | string | ✅ yes | Access key ID. Sensitive. |
| `secret_access_key` | string | ✅ yes | Secret access key. Sensitive. |
| `s3_compatible_host` | string | ❌ no | Endpoint of an S3-compatible service such as MinIO. |
| `password` | string | ❌ no | Password the archive was encrypted with. Sensitive. |

## Attributes Reference

| Name | Type | Description |
|------|------|-------------|
| `id` | string | Synthetic identifier carrying a timestamp. |

A restore replaces the instance's entire database. Every argument forces a new resource.
