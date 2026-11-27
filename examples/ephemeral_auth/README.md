# Ephemeral `portainer_auth`

Logs in to Portainer for one run and hands the JWT to the `restapi` provider; the session is
logged out (and the token revoked) when the run ends. The JWT is never in the plan or the state.

Requires Terraform >= 1.10 or OpenTofu >= 1.11. See
[docs/ephemeral-resources/auth.md](../../docs/ephemeral-resources/auth.md).
