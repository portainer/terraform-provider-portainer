#!/bin/bash
# E2E coverage for issue #147: a stack whose deployment failed must stay
# recoverable through Terraform, with no manual step in the Portainer UI.
#
# This cannot live in the ordinary e2e loop, which applies a directory once and
# expects every command to succeed. Reproducing #147 needs three applies with a
# deliberately failed deployment in the middle.

set -euo pipefail

PORTAINER_URL="${PORTAINER_URL:-https://localhost:9443}"
PORTAINER_API_KEY="${PORTAINER_API_KEY:-ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8=}"

terraform init -input=false
terraform fmt -check
terraform validate

echo "▶️ Step 1: deploy a healthy stack"
terraform apply -auto-approve

STACK_ID="$(terraform output -raw stack_id)"
echo "   stack id: ${STACK_ID}"

echo "▶️ Step 2: break it with an image tag that cannot be pulled"
# Portainer 2.45 deploys asynchronously, so this apply usually returns before
# the deployment fails. Whether the failure surfaces here at all depends on the
# Portainer version and is not what this test is about, so the exit status is
# deliberately not asserted.
terraform apply -auto-approve -var-file=broken.tfvars || true

echo "▶️ Step 3: wait for Portainer to report the stack as failed (status 4)"
status=""
for _ in $(seq 1 45); do
  status="$(curl -sk -H "X-API-Key: ${PORTAINER_API_KEY}" \
    "${PORTAINER_URL}/api/stacks/${STACK_ID}" | jq -r '.Status')"
  [ "${status}" = "4" ] && break
  sleep 2
done
if [ "${status}" != "4" ]; then
  echo "❌ The stack never reached status 4 (last seen: ${status:-none})."
  echo "   Step 4 would then pass without exercising issue #147 at all, so this"
  echo "   is a failure of the test, not of the provider. Fix the test rather"
  echo "   than ignoring it: the broken image tag may have started resolving,"
  echo "   or Portainer may have changed how it reports failed deployments."
  exit 1
fi
echo "   the stack is in Error, as the issue describes"

echo "▶️ Step 4: recover it — the regression test for #147"
# Before the fix the provider refused this in a fraction of a second without
# sending a single request, and every later apply failed the same way.
terraform apply -auto-approve -var-file=recovered.tfvars

echo "▶️ Step 5: clean up"
terraform destroy -auto-approve -var-file=recovered.tfvars

echo "✅ A stack left in Error was recovered through Terraform (issue #147)."
