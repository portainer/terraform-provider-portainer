#!/bin/bash
# Read used to do nothing for portainer_endpoint_settings, so a security
# switch flipped in the Portainer UI never showed up in the plan, and the
# resource could not be imported.
# shellcheck source=e2e-tests/lib/e2e.sh
source "$(dirname "$0")/../lib/e2e.sh"
cd "$(dirname "$0")" || exit 1

ENDPOINT_ID="${ENDPOINT_ID:-3}"
export TF_VAR_endpoint_id="${ENDPOINT_ID}"

tf_prepare

step "Apply the settings"
terraform apply -auto-approve

step "Forbid bind mounts in Portainer, outside Terraform"
api PUT "/endpoints/${ENDPOINT_ID}/settings" -d '{"allowBindMountsForRegularUsers":false}' >/dev/null
expect_plan 2 "the change made in Portainer shows up in the plan"
expect_plan_contains "allow_bind_mounts +=.*false -> true" "the plan restores allow_bind_mounts"

step "Apply restores it"
terraform apply -auto-approve
[ "$(api GET "/endpoints/${ENDPOINT_ID}" | jq -r '.SecuritySettings.allowBindMountsForRegularUsers')" = "true" ] ||
  fail "Portainer still forbids bind mounts after the apply"
echo "   Portainer allows bind mounts again"

step "Import into a fresh state"
reimport portainer_endpoint_settings.drift
expect_plan 0 "imported settings need no change"

echo "✅ Endpoint settings drift and import work."
