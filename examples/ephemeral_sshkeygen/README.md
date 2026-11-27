# Ephemeral `portainer_sshkeygen`

Has Portainer generate an SSH key pair without writing the private key to the plan or the state.
A new pair is generated on every run.

Requires Terraform >= 1.10 or OpenTofu >= 1.11. See
[docs/ephemeral-resources/sshkeygen.md](../../docs/ephemeral-resources/sshkeygen.md).
