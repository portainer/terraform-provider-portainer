# OIDC client

Requires Portainer Business Edition 3.1 or later.

Turns on Portainer's built-in OIDC provider (`portainer_settings.oidc_provider`) and registers Grafana as a client that the members of the `ops` team may sign in to. Configure Grafana's generic OAuth with the issuer, client ID and client secret from the outputs:

```shell
terraform apply
terraform output -raw grafana_oauth_client_secret
```

Portainer returns the client secret only once, when the client is created: it is kept in the Terraform state, so treat the state as a secret. To rotate it without downtime, add a `portainer_oidc_client_secret` (see `examples/oidc_client_secret`).
