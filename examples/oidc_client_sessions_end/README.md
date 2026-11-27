# End the sessions of an OIDC client

Requires Portainer Business Edition 3.1 or later.

Ends every session of an OIDC client (or only `user_id`'s), revoking their tokens at once, and again whenever `reason` changes.

With Terraform >= 1.14 the `portainer_oidc_client_sessions_end` action does the same without a resource in state.
