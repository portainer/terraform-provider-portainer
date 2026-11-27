---
page_title: "portainer_environment Resource - Portainer"
subcategory: ""
description: |-
  The portainer_environment resource allows you to register environments (a.k.a. endpoints) in Portainer.
---

# portainer_environment (Resource)

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
  name                = "Edge Device"
  environment_address = "https://portainer.example.com:9443"
  type                = 4
}

output "edge_key" {
  value     = portainer_environment.edge_env.edge_key
  sensitive = true
}

output "edge_id" {
  value = portainer_environment.edge_env.edge_id
}
```

Pass `edge_key` and `edge_id` to the Edge Agent as `PORTAINER_EDGE_KEY` and `PORTAINER_EDGE_ID`. Portainer assigns the Edge ID itself only while `enforce_edge_id` is enabled on `portainer_settings`; otherwise the provider generates one at create time and Portainer adopts it when the agent first checks in.

Because the value is produced at create time, `terraform import` and Edge Agent environments already in state from an earlier provider version keep an empty `edge_id` until an agent associates — after that a refresh picks up the ID reported by the agent.

**Do not set the `tls_*` attributes on an Edge Agent environment.** Portainer rejects TLS for edge types outright — `POST /endpoints` answers `400 TLS is not supported for Edge Agent environments` — because the agent dials out through the edge tunnel and never presents a certificate. Since `tls_enabled` defaults to `true`, the provider drops all TLS fields for types `4` and `7` rather than letting the default fail the create.

**Changing `environment_address` replaces the environment.** Portainer bakes the address into the edge key when the environment is created and never regenerates it, and its update endpoint does not accept a URL for edge types at all — so moving an edge environment means a new environment, a new edge key and a redeployed agent. Terraform reports this as a forced replacement. A difference in the scheme alone does not trigger it: that is Portainer's own host-only normalisation of `endpoint.URL` showing through for state written by older provider versions.

### Edge Agent settings

```hcl
resource "portainer_environment" "edge_async" {
  name                       = "Edge Device (async)"
  environment_address        = "https://portainer.example.com:9443"
  type                       = 4
  edge_tunnel_server_address = "tunnel.example.com:8000"
  edge_checkin_interval      = 10

  # Business Edition only
  edge_async_mode        = true
  edge_ping_interval     = 60
  edge_snapshot_interval = 300
  edge_command_interval  = 60
}
```

The Business Edition attributes (`edge_async_mode`, `edge_*_interval` except `edge_checkin_interval`, `custom_template_*`) fail with a clear error on Community Edition, which would silently ignore them.

### Register Azure (ACI) environment

```hcl
resource "portainer_environment" "aci" {
  name                                = "Azure"
  environment_address                 = "https://management.azure.com"
  type                                = 3
  azure_application_id                = var.azure_app_id
  azure_tenant_id                     = var.azure_tenant_id
  azure_authentication_key_wo         = var.azure_client_secret
  azure_authentication_key_wo_version = 1
}
```

Portainer validates the credentials against Azure on create and on every update, and never returns them; only changed credentials are sent.

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

<!-- schema generated by tfplugindocs -->
## Schema

### Required

- `environment_address` (String) Connection URL of the environment (e.g. tcp://host:port for Docker, https://host for Kubernetes/Agent). Used as the endpoint URL Portainer connects to.
- `name` (String) Name of the Portainer environment (endpoint) as displayed in the UI.
- `type` (Number) Environment type: 1 = Docker, 2 = Agent, 3 = Azure, 4 = Edge Agent, 5 = Kubernetes, 6 = Kubernetes via agent, 7 = Kubernetes Edge Agent

### Optional

> **NOTE**: [Write-only arguments](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments) are supported in Terraform 1.11 and later.

- `azure_application_id` (String) Azure (type 3) environments: Azure application (client) ID Portainer authenticates with. Sent on create and when changed; Portainer never returns it.
- `azure_authentication_key` (String, Sensitive) Azure (type 3) environments: Azure authentication key (client secret). Stored in state as a sensitive value; use `azure_authentication_key_wo` to keep it out of state.
- `azure_authentication_key_wo` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) Write-only Azure authentication key (supports ephemeral values; never stored in Terraform state). Sent on create and when `azure_authentication_key_wo_version` changes. Requires Terraform 1.11 or later.
- `azure_authentication_key_wo_version` (Number) Version of `azure_authentication_key_wo`. Increment it to send a new key.
- `azure_tenant_id` (String) Azure (type 3) environments: Azure tenant ID. Sent on create and when changed; Portainer never returns it.
- `container_engine` (String) Container engine used by the environment(endpoint). Value must be one of: 'docker' or 'podman'
- `custom_template_content` (String) Portainer Business Edition: custom template content Portainer deploys on the environment right after creating it, instead of `custom_template_id`. Create-time only.
- `custom_template_id` (Number) Portainer Business Edition: ID of a custom template Portainer deploys on the environment right after creating it (the KaaS / Kubernetes onboarding option). Create-time only: changing it later neither redeploys nor replaces anything.
- `custom_template_stack_name` (String) Portainer Business Edition: stack name of the custom template deployment (`custom_template_id` / `custom_template_content`). Create-time only.
- `delete_cluster_on_destroy` (Boolean) Portainer 3.0+: on destroy, send `deleteCluster=true` with DELETE /endpoints/{id}. For a KubeSolo environment installed by Portainer this uninstalls KubeSolo from the machine before the environment is removed; Portainer does that in the background (HTTP 202) and the provider waits until the environment is gone, within the delete timeout. Portainer answers 409 while KubeSolo is still being installed. Older releases ignore the parameter and only remove the environment. Changing it never touches the environment itself.
- `edge_async_mode` (Boolean) Portainer Business Edition, Edge Agent (type 4/7) environments: create the environment for an agent running in async mode. Portainer cannot change the mode later, so changing a configured value replaces the environment. When unset, Portainer's value is reported.
- `edge_checkin_interval` (Number) Edge Agent (type 4/7) environments: check-in (poll) interval of the agent in seconds. 0 uses the default interval from the Portainer settings. When unset, the provider leaves the current value alone and reports it.
- `edge_command_interval` (Number) Portainer Business Edition, Edge Agent environments in async mode: command interval of the agent in seconds. 0 uses the default from the Portainer settings. When unset, the provider leaves the current value alone and reports it.
- `edge_ping_interval` (Number) Portainer Business Edition, Edge Agent environments in async mode: ping interval of the agent in seconds. 0 uses the default from the Portainer settings. When unset, the provider leaves the current value alone and reports it.
- `edge_snapshot_interval` (Number) Portainer Business Edition, Edge Agent environments in async mode: snapshot interval of the agent in seconds. 0 uses the default from the Portainer settings. When unset, the provider leaves the current value alone and reports it.
- `edge_tunnel_server_address` (String) Edge Agent (type 4/7) environments: address (host:port) of the Portainer tunnel server the agent connects to, baked into the edge key at creation. Unset uses the address from the Portainer settings. Changing it replaces the environment, as Portainer never regenerates the edge key.
- `group_id` (Number) ID of the Portainer endpoint group. Default is 1 (Unassigned).
- `public_ip` (String) Public IP/URL used by Portainer for Published Ports (maps to PublicURL). When unset, Portainer uses the environment address.
- `tag_ids` (List of Number) List of tag IDs to assign to the environment.
- `team_access_policies` (Map of Number) Map of team IDs to role IDs (e.g. teamID -> roleID)
- `timeouts` (Block, Optional) (see [below for nested schema](#nestedblock--timeouts))
- `tls_ca_cert` (String, Sensitive) PEM-encoded TLS CA certificate used to verify the environment's server certificate. Stored in state as a sensitive value.
- `tls_cert` (String, Sensitive) PEM-encoded TLS client certificate presented by Portainer when connecting to the environment. Stored in state as a sensitive value.
- `tls_enabled` (Boolean) Whether TLS is enabled for the connection to the environment.
- `tls_key` (String, Sensitive) PEM-encoded TLS client private key paired with tls_cert. Stored in state as a sensitive value; use `tls_key_wo` to keep it out of state.
- `tls_key_wo` (String, Sensitive, [Write-only](https://developer.hashicorp.com/terraform/language/resources/ephemeral#write-only-arguments)) Write-only PEM-encoded TLS client private key (supports ephemeral values; never stored in Terraform state). Uploaded on create, whenever another TLS attribute changes, and when `tls_key_wo_version` changes. Requires Terraform 1.11 or later.
- `tls_key_wo_version` (Number) Version of `tls_key_wo`. Increment it to upload a new key.
- `tls_skip_client_verify` (Boolean) Whether to skip client TLS certificate verification when Portainer connects to the environment.
- `tls_skip_verify` (Boolean) Whether to skip server TLS certificate verification when connecting to the environment.
- `user_access_policies` (Map of Number) Map of user IDs to role IDs (e.g. userID -> roleID)

### Read-Only

- `edge_id` (String) Edge identifier (UUID) for type 4/7 (Edge Agent) environments, to pass to the Edge Agent as PORTAINER_EDGE_ID. Returned by Portainer when the EnforceEdgeID setting is enabled, otherwise generated by the provider at create time. Empty for all other environment types.
- `edge_key` (String, Sensitive) Edge key generated by Portainer for type 4/7 (Edge Agent) environments. Used by the Edge Agent to register with Portainer.
- `id` (String) Identifier of the environment.

<a id="nestedblock--timeouts"></a>
### Nested Schema for `timeouts`

Optional:

- `delete` (String) A string that can be [parsed as a duration](https://pkg.go.dev/time#ParseDuration) consisting of numbers and unit suffixes, such as "30s" or "2h45m". Valid time units are "s" (seconds), "m" (minutes), "h" (hours). Setting a timeout for a Delete operation is only applicable if changes are saved into state before the destroy operation occurs.

## Notes on Arguments and Attributes

- `environment_address`: Target environment address (e.g. `tcp://host:9001`).
- `type`: Portainer also stores agent addresses without the `tcp://` scheme.
- `tls_enabled`: Defaults to `true`. Must be `true` for agent-based environments. **Ignored for Edge Agent types (`4`, `7`)** — Portainer rejects TLS for them, so the provider does not send it.
- `tls_skip_verify`: Defaults to `true`.
- `tls_skip_client_verify`: Defaults to `true`.
- `tls_ca_cert`: Uploaded as `TLSCACertFile` when `tls_enabled = true` and `tls_skip_verify = false`.
- `tls_cert`: Uploaded when `tls_enabled = true` and `tls_skip_client_verify = false`.
- `tls_key`: Uploaded when `tls_enabled = true` and `tls_skip_client_verify = false`.
- `tls_key_wo`: Conflicts with `tls_key`
- `container_engine`: Defaults to `docker`. Changing it forces replacement.
- `delete_cluster_on_destroy`: Defaults to `20m`.
- `azure_authentication_key_wo`: Conflicts with `azure_authentication_key`
- `custom_template_id`: Conflicts with `custom_template_content`.
- `custom_template_stack_name`: **Business Edition.** Stack name of the custom template deployment (sent as `StackName`).

## Timeouts

| Operation | Default | Description |
|-----------|---------|-------------|
| `delete`  | `20m`   | Time to wait for Portainer to uninstall KubeSolo and remove the environment when `delete_cluster_on_destroy = true`. Unused otherwise. |

```hcl
resource "portainer_environment" "kubesolo" {
  # ...
  delete_cluster_on_destroy = true

  timeouts {
    delete = "30m"
  }
}
```

## Import

Environments can be imported using their numeric endpoint ID:

```shell
terraform import portainer_environment.example 7
```

### Import by identity

Terraform 1.12 and later can also import by resource identity: the object's numeric ID as `id` in an `import` block. The ID string above keeps working.

```hcl
import {
  to       = portainer_environment.example
  identity = {
    id = "1"
  }
}
```

`terraform query` (Terraform 1.14+) with the [`portainer_environment` list resource](../list-resources/environment.md) finds the objects to import and can generate the `import` blocks (`terraform query -generate-config-out=generated.tf`). The identity is stored in the state from v3.1.0 on; state written by v3.0.0 gets it on the next refresh, without a plan change. OpenTofu supports neither resource identity nor list resources; it keeps importing by ID.
