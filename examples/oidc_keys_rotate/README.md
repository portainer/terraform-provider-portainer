# OIDC signing key rotation

Requires Portainer Business Edition 3.1 or later.

Rotates the signing key of Portainer's OIDC provider, and again whenever `rotation_id` changes. Tokens signed with the replaced key verify until they expire; rotating twice within a token lifetime refuses them.

With Terraform >= 1.14 the `portainer_oidc_keys_rotate` action does the same without a resource in state.
