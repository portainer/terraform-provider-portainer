#!/bin/bash
# password_wo keeps the registry password out of state (write-only attributes
# need Terraform 1.11+ or OpenTofu 1.11+).
#
# The passwords are passed with -var rather than from .tfvars files on purpose:
# the repository's .gitignore excludes *.tfvars.
# shellcheck source=e2e-tests/lib/e2e.sh
source "$(dirname "$0")/../lib/e2e.sh"
cd "$(dirname "$0")" || exit 1

tf_prepare

step "Create the registry with a write-only password"
terraform apply -auto-approve
expect_not_in_state "e2e-first-secret" "the password is not in state"
expect_plan 0 "the write-only password causes no diff"

step "Rotate it (password_wo_version 2)"
ROTATE=(-var="registry_password=e2e-second-secret" -var="registry_password_version=2")
terraform apply -auto-approve "${ROTATE[@]}"
expect_not_in_state "e2e-second-secret" "the rotated password is not in state either"
expect_plan 0 "no diff after the rotation" "${ROTATE[@]}"

step "Clean up"
terraform destroy -auto-approve "${ROTATE[@]}"

echo "✅ password_wo stays out of state across create and rotation."
