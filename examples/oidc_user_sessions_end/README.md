# Sign a user out of every OIDC client

Requires Portainer Business Edition 3.1 or later.

Signs a user out of every client and add-on that signed them in through Portainer's OIDC provider, revoking their tokens at once, and again whenever `reason` changes.

With Terraform >= 1.14 the `portainer_oidc_user_sessions_end` action does the same without a resource in state.
