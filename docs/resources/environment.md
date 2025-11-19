# 🌐 Resource Documentation: `portainer_environment`

# portainer_environment
The `portainer_environment` resource allows you to register environments (a.k.a. endpoints) in Portainer.

## Example Usage

### Register Docker host (non-agent)

```hcl
resource "portainer_environment" "your-host" {
  name                = "Your Host"
  environment_address = "tcp://192.168.1.100:2375"
  type                = 1
  group_id            = 1
}
```

### Register agent-based environment
```hcl
resource "portainer_tag" "your-tag" {
  name = "your-tag"
}

resource "portainer_environment" "your-host" {
  name                = "Your Host"
  environment_address = "tcp://192.168.1.101:9001"
  type                = 2
  group_id            = 1
  tag_ids             = [portainer_tag.your-group.id]

  user_access_policies = {
    "3" = 1  # userID 3 -> roleID 1
  }

  team_access_policies = {
    "2" = 2  # teamID 2 -> roleID 2
  }
}
```
### Register Edge Agent environment
```hcl
resource "portainer_environment" "edge_env" {
  name                 = "Edge Device"
  environment_address  = "edge-device.local"
  type                 = 4
  tls_enabled          = true
  tls_skip_verify      = true
  tls_skip_client_verify = true
}

output "edge_key" {
  value = portainer_environment.edge_env.edge_key
}

output "edge_id" {
  value = portainer_environment.edge_env.edge_id
}
```

### Register Docker host secured via TLS (certs rxample from Vault/TLS)

```hcl
resource "vault_pki_secret_backend_cert" "portainer_client" {
  backend     = "pki"
  name        = "portainer-client"
  common_name = "client.example.com"
}

resource "tls_private_key" "portainer_client" {
  algorithm = "RSA"
  rsa_bits  = 4096
}

resource "portainer_environment" "docker_tls" {
  name                = "Docker over TLS"
  environment_address = "tcp://192.168.1.100:2376"
  type                = 1
  group_id            = 1

  # TLS must be enabled and server verification must NOT be skipped
  tls_enabled     = true
  tls_skip_verify = false

  # These values are uploaded to Portainer as TLSCACertFile, TLSCertFile and TLSKeyFile
  tls_ca_cert = vault_pki_secret_backend_cert.portainer_client.ca_chain
  tls_cert    = vault_pki_secret_backend_cert.portainer_client.certificate
  tls_key     = tls_private_key.portainer_client.private_key_pem
}
```
> TLS files (`tls_ca_cert`, `tls_cert`, `tls_key`) will be upload to Portaienr only if variables set this:
> `tls_enabled = true` **and** `tls_skip_verify = false`.

## Lifecycle & Behavior

Environments are updated if any of the attributes change (e.g., name, address, type, group_id, tag_ids).

- To delete an environment created via Terraform, simply run:
```hcl
terraform destroy
```

- To update an environment, change any attribute and re-apply::
```hcl
terraform apply
```
> ⚠️ Portainer does not allow updating existing environments via API. Any change will destroy and recreate the environment.

## Arguments Reference

| Name                       | Type       | Required                     | Description                                                                                      |
|----------------------------|------------|------------------------------|--------------------------------------------------------------------------------------------------|
| `name`                     | string     | ✅ yes                       | Display name of the environment in Portainer.                                                   |
| `environment_address`      | string     | ✅ yes                       | Target environment address (e.g. `tcp://host:9001`).                                            |
| `type`                     | int        | ✅ yes                       | Environment type: `1` = Docker, `2` = Agent, `3` = Azure, `4` = Edge Agent, `5` = Kubernetes.   |
| `group_id`                 | int        | 🚫 optional (default `1`)   | ID of the Portainer endpoint group. Default is `1` (Unassigned).                                |
| `tag_ids`                  | list(int)  | 🚫 optional                 | List of Portainer tag IDs to assign to the environment. Only used during creation.              |
| `tls_enabled`              | bool       | 🚫 optional (default `true`)| Enable TLS for connection to the agent. Must be `true` for agent-based environments.            |
| `tls_skip_verify`          | bool       | 🚫 optional (default `true`)| Skip server certificate verification. Useful for self-signed certificates.                      |
| `tls_skip_client_verify`   | bool       | 🚫 optional (default `true`)| Skip client certificate verification. Used when mutual TLS is not required.                     |
| `tls_ca_cert`              | string     | 🚫 optional (sensitive)     | PEM-encoded CA certificate. Uploaded as `TLSCACertFile` when `tls_enabled = true` and `tls_skip_verify = false`. |
| `tls_cert`                 | string     | 🚫 optional (sensitive)     | PEM-encoded client certificate. Uploaded as `TLSCertFile` when `tls_enabled = true` and `tls_skip_verify = false`. |
| `tls_key`                  | string     | 🚫 optional (sensitive)     | PEM-encoded client private key. Uploaded as `TLSKeyFile` when `tls_enabled = true` and `tls_skip_verify = false`. |
| `user_access_policies`     | map(object({ RoleId = int })) | 🚫 optional | Access control for users (applies to environments only).                                       |
| `team_access_policies`     | map(object({ RoleId = int })) | 🚫 optional | Access control for teams (applies to environments only).                                       |

## Attributes Reference
| Name       | Description                                |
| ---------- | ------------------------------------------ |
| `id`       | ID of the Portainer environment            |
| `edge_key` | Edge key/token for Edge Agent registration |
| `edge_id`  | Unique Edge Agent identifier (EdgeID)      |
> `edge_key` and `edge_id` are only populated for environments of type `4` (Edge Agent).

> `edge_id` will only be available **immediately after creation** if the Portainer setting  
**Enforce use of Portainer generated Edge ID** is enabled:
```hcl
resource "portainer_settings" "portainer_settings" {
   enforce_edge_id = true
}
```
> Otherwise, `edge_id` is assigned only after the Edge Agent connects to Portainer for the first time.