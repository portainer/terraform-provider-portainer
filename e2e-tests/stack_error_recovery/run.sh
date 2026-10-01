#!/bin/bash
# E2E coverage for issue #147: a stack whose deployment failed must stay
# recoverable through Terraform, with no manual step in the Portainer UI.
#
# This cannot live in the ordinary e2e loop, which applies a directory once and
# expects every command to succeed. Reproducing #147 needs three applies with a
# deliberately failed deployment in the middle.
#
# The image overrides are passed with -var rather than from .tfvars files on
# purpose: the repository's .gitignore excludes *.tfvars, so fixture files here
# would exist locally and be missing from CI.

set -euo pipefail

PORTAINER_URL="${PORTAINER_URL:-https://localhost:9443}"
PORTAINER_API_KEY="${PORTAINER_API_KEY:-ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8=}"

BROKEN_IMAGE="nginx:tag-that-does-not-exist-147"
# Different from the default too, so the recovery step always has something to
# apply regardless of what the broken step managed to write to state.
RECOVERED_IMAGE="nginx:alpine"

# await_stack_status polls until the stack reaches the wanted status, and fails
# the test if it never does. Portainer deploys asynchronously and this provider
# returns from an active = true apply before the deployment settles, so a
# command that exited 0 proves nothing on its own.
await_stack_status() {
  local want="$1" what="$2" status=""
  for _ in $(seq 1 45); do
    status="$(curl -sk -H "X-API-Key: ${PORTAINER_API_KEY}" \
      "${PORTAINER_URL}/api/stacks/${STACK_ID}" | jq -r '.Status')"
    [ "${status}" = "${want}" ] && { echo "   ${what}"; return 0; }
    sleep 2
  done
  echo "❌ The stack never reached status ${want} (last seen: ${status:-none}): ${what}"
  return 1
}

terraform init -input=false
terraform fmt -check
terraform validate

echo "▶️ Step 1: deploy a healthy stack"
terraform apply -auto-approve

STACK_ID="$(terraform output -raw stack_id)"
echo "   stack id: ${STACK_ID}"
await_stack_status 1 "the stack is running, so the starting point is sound"

echo "▶️ Step 2: break it with an image tag that cannot be pulled"
# Portainer 2.45 deploys asynchronously, so this apply usually returns before
# the deployment fails. Whether the failure surfaces here at all depends on the
# Portainer version and is not what this test is about, so the exit status is
# deliberately not asserted — the status check below is what matters.
terraform apply -auto-approve -var="stack_image=${BROKEN_IMAGE}" || true

echo "▶️ Step 3: Portainer must report the stack as failed (status 4)"
if ! await_stack_status 4 "the stack is in Error, as the issue describes"; then
  echo "   Step 4 would then pass without exercising issue #147 at all, so this"
  echo "   is a failure of the test, not of the provider. Fix the test rather"
  echo "   than ignoring it: the broken image tag may have started resolving,"
  echo "   or Portainer may have changed how it reports failed deployments."
  exit 1
fi

echo "▶️ Step 4: recover it — the regression test for #147"
# Before the fix the provider refused this in a fraction of a second without
# sending a single request, and every later apply failed the same way.
terraform apply -auto-approve -var="stack_image=${RECOVERED_IMAGE}"

echo "▶️ Step 5: the recovery must have actually brought the stack back up"
# An apply that exits 0 is not proof: the deployment settles after it returns,
# and could still end in Error.
await_stack_status 1 "the stack is running again"

echo "▶️ Step 6: clean up"
terraform destroy -auto-approve -var="stack_image=${RECOVERED_IMAGE}"

echo "✅ A stack left in Error was recovered through Terraform (issue #147)."
