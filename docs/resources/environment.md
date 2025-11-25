# 🌐 Resource Documentation: `portainer_environment`

# portainer_environment
The `portainer_environment` resource allows you to register environments (a.k.a. endpoints) in Portainer.

## Example Usage

### Register Docker host (non-agent)

```hcl
resource "portainer_environment" "your-host" {
  name                = "Your Host"
  environment_address = "tcp://192.168.1.100:2375"
  public_ip           = "public.example.com"
  type                = 1
  group_id            = 1
}
````

### Register agent-based environment

```hcl
resource "portainer_tag" "your-tag" {
  name = "your-tag"
}

resource "portainer_environment" "your-host" {
  name                = "Your Host"
  environment_address = "tcp://192.168.1.101:9001"
  public_ip           = "public.example.com"
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
  name                   = "Edge Device"
  environment_address    = "edge-device.local"
  public_ip              = "edge.public.example.com"
  type                   = 4
  tls_enabled            = true
  tls_skip_verify        = true
  tls_skip_client_verify = true
}

output "edge_key" {
  value = portainer_environment.edge_env.edge_key
}

output "edge_id" {
  value = portainer_environment.edge_env.edge_id
}
```

### Register Docker host secured via TLS (certs example from Vault/TLS)

```hcl
resource "meu_portainer_environment" "docker_tls" {
  name                = "Docker over TLS"
  environment_address = "tcp://192.168.1.100:2376"
  public_ip           = "docker-tls.example.com"
  type                = 1
  group_id            = 1

  tls_enabled     = true
  tls_skip_verify = false

  tls_ca_cert = vault_pki_secret_backend_cert.portainer_client.ca_chain
  tls_cert    = vault_pki_secret_backend_cert.portainer_client.certificate
  tls_key     = tls_private_key.portainer_client.private_key_pem
}
```

> TLS files (`tls_ca_cert`, `tls_cert`, `tls_key`) will be uploaded to Portainer only if:
>
> * `tls_enabled = true` **and**
> * `tls_skip_verify = false`.

---

## Arguments Reference

| Name                     | Type                          | Required                     | Description                                                                                                             |
| ------------------------ | ----------------------------- | ---------------------------- | ----------------------------------------------------------------------------------------------------------------------- |
| `name`                   | string                        | ✅ yes                       | Display name of the environment in Portainer.                                                                           |
| `environment_address`    | string                        | ✅ yes                       | Target environment address (e.g. `tcp://host:9001`).                                                                    |
| `public_ip`              | string                        | 🚫 optional                  | Public URL/IP shown in Portainer UI (maps to `PublicURL` field). Useful for correct Published Ports rendering.          |
| `type`                   | int                           | ✅ yes                       | Environment type: `1` = Docker, `2` = Agent, `3` = Azure, `4` = Edge Agent, `5` = Kubernetes, `6` = Kubernetes via Agent |
| `group_id`               | int                           | 🚫 optional (default `1`)    | ID of the Portainer endpoint group. Default is `1` (Unassigned).                                                        |
| `tag_ids`                | list(int)                     | 🚫 optional                  | List of Portainer tag IDs to assign to the environment. Only used during creation.                                      |
| `tls_enabled`            | bool                          | 🚫 optional (default `true`) | Enable TLS for connection to the agent. Must be `true` for agent-based environments.                                    |
| `tls_skip_verify`        | bool                          | 🚫 optional (default `true`) | Skip server certificate verification. Useful for self-signed certificates.                                              |
| `tls_skip_client_verify` | bool                          | 🚫 optional (default `true`) | Skip client certificate verification. Used when mutual TLS is not required.                                             |
| `tls_ca_cert`            | string                        | 🚫 optional (sensitive)      | PEM-encoded CA certificate. Uploaded as `TLSCACertFile` when `tls_enabled = true` and `tls_skip_verify = false`.        |
| `tls_cert`               | string                        | 🚫 optional (sensitive)      | PEM-encoded client certificate. Uploaded as `TLSCertFile` when `tls_enabled = true` and `tls_skip_verify = false`.      |
| `tls_key`                | string                        | 🚫 optional (sensitive)      | PEM-encoded client private key. Uploaded as `TLSKeyFile` when `tls_enabled = true` and `tls_skip_verify = false`.       |
| `user_access_policies`   | map(object({ RoleId = int })) | 🚫 optional                  | Access control for users (applies to environments only).                                                                |
| `team_access_policies`   | map(object({ RoleId = int })) | 🚫 optional                  | Access control for teams (applies to environments only).                                                                |

---

## Attributes Reference
| Name       | Description                                |
| ---------- | ------------------------------------------ |
| `id`       | ID of the Portainer environment            |
| `edge_key` | Edge key/token for Edge Agent registration |
| `edge_id`  | Unique Edge Agent identifier (EdgeID)      |
