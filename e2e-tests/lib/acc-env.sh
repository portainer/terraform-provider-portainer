#!/bin/bash
# Prepares a running Portainer for the Go acceptance tests (TestAcc* in
# internal/) and prints the variables that name what they run against:
#
#   PORTAINER_ACC_DOCKER_ENV_ID  the local Docker environment
#   PORTAINER_ACC_K8S_ENV_ID     the first Kubernetes environment, if any
#   PORTAINER_ACC_SWARM_ENV_ID   with ACC_SWARM=1: the local environment, once
#   PORTAINER_ACC_SWARM_SERVICE  this host is a Swarm manager running a
#   PORTAINER_ACC_CONTAINER      service and a container for the tests
#
# Needs PORTAINER_ENDPOINT and PORTAINER_API_KEY. Variables already set are
# kept. In GitHub Actions they also go to $GITHUB_ENV. Locally:
#   eval "$(ACC_SWARM=1 bash e2e-tests/lib/acc-env.sh)"

set -euo pipefail

URL="${PORTAINER_ENDPOINT:?PORTAINER_ENDPOINT must be set}"
: "${PORTAINER_API_KEY:?PORTAINER_API_KEY must be set}"
SERVICE="${ACC_SERVICE_NAME:-acc-service}"
CONTAINER="${ACC_CONTAINER_NAME:-acc-container}"
IMAGE="busybox:1.36"

fail() {
  echo "❌ $*" >&2
  exit 1
}

api() {
  # api PATH [curl args...]
  local path="$1"
  shift
  curl -sk --fail-with-body -H "X-API-Key: ${PORTAINER_API_KEY}" "$@" "${URL}/api${path}"
}

vars=()
add() {
  # add NAME VALUE: kept when the caller already set NAME.
  local current="${!1:-}"
  vars+=("$1=${current:-$2}")
}

# Edge groups, jobs and stacks answer 503 until Edge compute is turned on.
# Business Edition also needs the Edge compute URLs before it generates an
# Edge key (400 "Portainer API server URL is not set in Edge Compute
# settings") or creates an Edge agent environment (500 "Tunnel server address
# not set"); Community Edition falls back to the request's host. Values already
# configured are kept; ACC_EDGE_PORTAINER_URL and ACC_EDGE_TUNNEL_ADDRESS
# override the defaults (this URL, and its host on port 8000).
SETTINGS="$(api /settings)" || fail "the settings could not be read at ${URL}"
EDGE_HOST="$(sed -E 's#^[a-z]+://##; s#[/:].*$##' <<<"${URL}")"
EDGE_URL="$(jq -r '.EdgePortainerUrl // empty' <<<"${SETTINGS}")"
EDGE_TUNNEL="$(jq -r '.Edge.TunnelServerAddress // empty' <<<"${SETTINGS}")"
EDGE_URL="${EDGE_URL:-${ACC_EDGE_PORTAINER_URL:-${URL%/}}}"
EDGE_TUNNEL="${EDGE_TUNNEL:-${ACC_EDGE_TUNNEL_ADDRESS:-${EDGE_HOST}:8000}}"
# The update payload names the URL EdgePortainerURL (the settings object
# returns it as EdgePortainerUrl).
jq -n --arg url "${EDGE_URL}" --arg tunnel "${EDGE_TUNNEL}" \
  '{EnableEdgeComputeFeatures: true, EdgePortainerURL: $url, Edge: {TunnelServerAddress: $tunnel}}' |
  api /settings -X PUT -H 'Content-Type: application/json' --data-binary @- >/dev/null ||
  fail "Edge compute could not be enabled at ${URL}"

ENVS="$(api /endpoints)" || fail "the environments could not be listed at ${URL}"
# 1 = Docker (local socket), 5/6/7 = Kubernetes (local, agent, Edge agent).
DOCKER_ID="$(jq -r '[.[] | select(.Type == 1)][0].Id // empty' <<<"${ENVS}")"
K8S_ID="$(jq -r '[.[] | select(.Type == 5 or .Type == 6 or .Type == 7)][0].Id // empty' <<<"${ENVS}")"
[ -n "${DOCKER_ID}" ] || fail "no local Docker environment (type 1) at ${URL}"
add PORTAINER_ACC_DOCKER_ENV_ID "${DOCKER_ID}"
if [ -n "${K8S_ID}" ]; then
  add PORTAINER_ACC_K8S_ENV_ID "${K8S_ID}"
fi

if [ "${ACC_SWARM:-}" = 1 ]; then
  echo "▶️ Swarm, service ${SERVICE} and container ${CONTAINER} for the Docker tests" >&2
  docker info --format '{{.Swarm.LocalNodeState}}' | grep -q '^active$' || docker swarm init >/dev/null
  docker service rm "${SERVICE}" >/dev/null 2>&1 || true
  docker service create --detach --quiet --name "${SERVICE}" "${IMAGE}" sleep 86400 >/dev/null
  docker rm -f "${CONTAINER}" >/dev/null 2>&1 || true
  CONTAINER_ID="$(docker run -d --name "${CONTAINER}" "${IMAGE}" sleep 86400)"
  add PORTAINER_ACC_SWARM_ENV_ID "${DOCKER_ID}"
  # Used as the service name (portainer_endpoint_service_update) and as the
  # resource ID of a service webhook, which Docker resolves by name too.
  add PORTAINER_ACC_SWARM_SERVICE "${SERVICE}"
  add PORTAINER_ACC_CONTAINER "${CONTAINER_ID}"
fi

for v in "${vars[@]}"; do
  echo "export ${v}"
  if [ -n "${GITHUB_ENV:-}" ]; then
    echo "${v}" >>"${GITHUB_ENV}"
  fi
done
