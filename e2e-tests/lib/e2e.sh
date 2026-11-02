#!/bin/bash
# Shared helpers for the scripted E2E tests (the directories with a run.sh).
#
# These tests check what a single `terraform apply` cannot: that a change made
# in Portainer outside Terraform shows up in the plan, that an import yields a
# clean plan, that a secret stays out of state, or that a failed deployment
# fails the apply. Source this file from a run.sh:
#
#   source "$(dirname "$0")/../lib/e2e.sh"

set -euo pipefail

PORTAINER_URL="${PORTAINER_URL:-https://localhost:9443}"
PORTAINER_API_KEY="${PORTAINER_API_KEY:-ptr_xrP7XWqfZEOoaCJRu5c8qKaWuDtVc2Zb07Q5g22YpS8=}"

# The last plan's output, kept outside the test directory.
PLAN_OUT="$(mktemp)"

# A test that fails midway destroys what it created, so a local rerun against
# the shared Portainer fixture does not start from a dirty instance. Scripts
# that need their own cleanup replace this trap.
e2e_exit() {
  local rc=$?
  if [ "${rc}" != 0 ] && [ -n "$(terraform state list 2>/dev/null)" ]; then
    echo "   cleaning up after the failure"
    terraform destroy -auto-approve -input=false >/dev/null 2>&1 || echo "   cleanup failed; destroy by hand"
  fi
  rm -f "${PLAN_OUT}"
}
trap e2e_exit EXIT

# api calls the Portainer API with the admin key: api <method> <path> [curl args].
api() {
  local method="$1" path="$2"
  shift 2
  curl -sk --fail-with-body -X "${method}" -H "X-API-Key: ${PORTAINER_API_KEY}" "$@" "${PORTAINER_URL}/api${path}"
}

fail() {
  echo "❌ $*"
  exit 1
}

step() {
  echo "▶️ $*"
}

tf_prepare() {
  terraform init -input=false
  terraform fmt -check
  terraform validate
}

# expect_plan <exit code> <what> [plan args]: runs a plan and fails the test
# unless `terraform plan -detailed-exitcode` returns the given code
# (0 = no changes, 2 = changes). The plan is printed when it does not match.
expect_plan() {
  local want="$1" what="$2" rc=0
  shift 2
  terraform plan -detailed-exitcode -input=false -no-color "$@" >"${PLAN_OUT}" 2>&1 || rc=$?
  if [ "${rc}" != "${want}" ]; then
    cat "${PLAN_OUT}"
    fail "plan exited ${rc}, expected ${want}: ${what}"
  fi
  echo "   ${what}"
}

# expect_plan_contains <pattern> <what>: the last plan must mention pattern.
expect_plan_contains() {
  grep -qE "$1" "${PLAN_OUT}" || { cat "${PLAN_OUT}"; fail "the plan does not mention '$1': $2"; }
  echo "   $2"
}

# state_id <address>: the ID of a resource in state.
state_id() {
  terraform state show -no-color "$1" | sed -n -E 's/^ *id *= *"([^"]+)".*/\1/p'
}

# reimport <address>: forgets a resource and imports it again by the same ID,
# the way someone adopting an existing object would.
reimport() {
  local address="$1" id
  id="$(state_id "${address}")"
  [ -n "${id}" ] || fail "no ID in state for ${address}"
  terraform state rm "${address}" >/dev/null
  terraform import -input=false "${address}" "${id}"
}

# expect_not_in_state <secret> <what>: the secret must not appear anywhere in
# the state file.
expect_not_in_state() {
  if terraform state pull | grep -qF -- "$1"; then
    fail "the secret was written to state: $2"
  fi
  echo "   $2"
}
