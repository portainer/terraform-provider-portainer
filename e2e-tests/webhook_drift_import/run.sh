#!/bin/bash
# A webhook deleted in Portainer used to stay in state, so the URL handed out
# for it kept failing while every plan was clean. Destroying one also always
# failed on Portainer 2.45, which answers the delete with 204. And it could not
# be imported.
# shellcheck source=e2e-tests/lib/e2e.sh
source "$(dirname "$0")/../lib/e2e.sh"
cd "$(dirname "$0")" || exit 1

tf_prepare

step "Create the webhook"
terraform apply -auto-approve
WEBHOOK_ID="$(state_id portainer_webhook.drift)"

step "Delete it in Portainer, outside Terraform"
api DELETE "/webhooks/${WEBHOOK_ID}" >/dev/null
expect_plan 2 "the deleted webhook is planned again"
expect_plan_contains "portainer_webhook.drift will be created" "it is recreated, not kept in state"

step "Recreate it, then import it into a fresh state"
terraform apply -auto-approve
reimport portainer_webhook.drift
expect_plan 0 "an imported webhook needs no change"

step "Destroy (Portainer answers 204)"
terraform destroy -auto-approve

echo "✅ Webhook drift, import and destroy work."
