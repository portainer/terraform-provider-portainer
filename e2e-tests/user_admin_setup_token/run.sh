#!/bin/bash
# portainer_user_admin against Portainer 2.45+, which refuses to initialize a
# fresh instance without the X-Setup-Token header, and which answers the init
# call with 403 (not 409) on an initialized instance until it restarts. Before
# the fix the resource also never converged: every plan proposed it again.
#
# It needs an uninitialized Portainer, so it starts its own instead of using
# the shared one (which the e2e fixture ships initialized).
# shellcheck source=e2e-tests/lib/e2e.sh
source "$(dirname "$0")/../lib/e2e.sh"
cd "$(dirname "$0")" || exit 1

CONTAINER="e2e-portainer-setup-token"
PORT="${SETUP_TOKEN_PORT:-19443}"
export TF_VAR_portainer_url="https://localhost:${PORT}"

cleanup() {
  docker rm -f "${CONTAINER}" >/dev/null 2>&1 || true
  rm -f "${PLAN_OUT}"
}
trap cleanup EXIT
cleanup

step "Start a fresh Portainer ${PORTAINER_VERSION:-2.45.3}"
docker run -d --name "${CONTAINER}" -p "${PORT}:9443" "portainer/portainer-ce:${PORTAINER_VERSION:-2.45.3}" >/dev/null
UP=""
for _ in $(seq 1 60); do
  curl -sk -o /dev/null "${TF_VAR_portainer_url}/api/system/status" && UP=1 && break
  sleep 1
done
[ -n "${UP}" ] || fail "Portainer ${PORTAINER_VERSION:-2.45.3} did not come up on port ${PORT}"
TOKEN=""
for _ in $(seq 1 30); do
  TOKEN="$(docker logs "${CONTAINER}" 2>&1 | sed -E 's/\x1b\[[0-9;]*m//g' | grep -o -E 'setup_token=[0-9a-f]+' | head -1 | cut -d= -f2 || true)"
  [ -n "${TOKEN}" ] && break
  sleep 1
done
[ -n "${TOKEN}" ] || fail "no setup_token= line in the Portainer log"
echo "   got the setup token from the log"

tf_prepare

step "Initialize the admin with the setup token"
terraform apply -auto-approve -var="setup_token=${TOKEN}"
expect_plan 0 "the plan is clean right after the bootstrap" -var="setup_token=${TOKEN}"
expect_plan 0 "and stays clean" -var="setup_token=${TOKEN}"

step "Adopt the initialized instance without the token"
# Portainer still enforces the token until it restarts and would answer the
# init call with 403; the resource has to see the admin and not call it.
terraform state rm portainer_user_admin.bootstrap >/dev/null
terraform apply -auto-approve
expect_plan 0 "an initialized instance is adopted with a clean plan"

echo "✅ portainer_user_admin works with the setup token and converges."
