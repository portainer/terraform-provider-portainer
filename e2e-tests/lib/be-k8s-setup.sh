#!/bin/bash
# Connects the k3d cluster "mycluster" to the Portainer Business Edition that
# e2e-tests/lib/be-setup.sh started, through an Edge Agent in tunnel mode -
# the connection D2K needs - and installs the Gateway API CRDs with one
# GatewayClass. Needs the variables be-setup.sh exports and kubectl pointing at
# the cluster.
#
# Prints the environment ID as TF_VAR_k8s_environment_id; in GitHub Actions it
# also goes to $GITHUB_ENV. Locally: eval "$(bash e2e-tests/lib/be-k8s-setup.sh)"

set -euo pipefail

CONTAINER="${BE_CONTAINER:-e2e-portainer-ee}"
VERSION="${PORTAINER_VERSION:-2.45.3}"
NETWORK="${K3D_NETWORK:-k3d-mycluster}"
GATEWAY_API_VERSION="v1.2.1"

fail() {
  echo "❌ $*" >&2
  exit 1
}

api() {
  curl -sk --fail-with-body -H "X-API-Key: ${PORTAINER_API_KEY}" "$@"
}

# The agent reaches Portainer over the k3d network, so the edge key has to
# carry Portainer's address on that network.
docker network connect "${NETWORK}" "${CONTAINER}" 2>/dev/null || true
IP="$(docker inspect -f "{{(index .NetworkSettings.Networks \"${NETWORK}\").IPAddress}}" "${CONTAINER}")"
[ -n "${IP}" ] || fail "${CONTAINER} is not on ${NETWORK}"

echo "▶️ Point Edge agents at https://${IP}:9443" >&2
api -X PUT "${PORTAINER_URL}/api/settings" -H 'Content-Type: application/json' \
  -d "{\"EdgePortainerURL\":\"https://${IP}:9443\",\"Edge\":{\"TunnelServerAddress\":\"${IP}:8000\"},\"enableEdgeComputeFeatures\":true}" >/dev/null ||
  fail "the Edge settings could not be saved"

echo "▶️ Create the Edge environment" >&2
ENV="$(api -X POST "${PORTAINER_URL}/api/endpoints" -F Name=k3d -F EndpointCreationType=4 -F "URL=https://${IP}:9443")"
ENV_ID="$(echo "${ENV}" | jq -r .Id)"
EDGE_KEY="$(echo "${ENV}" | jq -r .EdgeKey)"
EDGE_ID="$(echo "${ENV}" | jq -r '.EdgeID // empty')"
[ -n "${EDGE_ID}" ] || EDGE_ID="$(cat /proc/sys/kernel/random/uuid 2>/dev/null || uuidgen | tr '[:upper:]' '[:lower:]')"

echo "▶️ Deploy Edge Agent ${VERSION} into k3d" >&2
# Replaces the Portainer Agent the e2e-setup action deploys: an agent trusts
# the first Portainer that talks to it.
kubectl delete namespace portainer --wait=true >/dev/null 2>&1 || true
kubectl create namespace portainer >/dev/null
kubectl -n portainer create configmap portainer-agent-edge \
  --from-literal=EDGE_ID="${EDGE_ID}" --from-literal=EDGE_INSECURE_POLL=1 --from-literal=EDGE_SECRET= >/dev/null
kubectl -n portainer create secret generic portainer-agent-edge-key --from-literal=edge.key="${EDGE_KEY}" >/dev/null
curl -sSfL https://downloads.portainer.io/ee-sts/portainer-agent-edge-k8s.yaml |
  sed "s|portainer/agent:sts|portainer/agent:${VERSION}|" | kubectl apply -f - >/dev/null
kubectl -n portainer wait --for=condition=available deployment/portainer-agent --timeout=180s >/dev/null ||
  fail "the Edge Agent did not start"

echo "▶️ Wait for the environment to answer" >&2
READY=""
for _ in $(seq 1 60); do
  if [ "$(api -o /dev/null -w '%{http_code}' "${PORTAINER_URL}/api/kubernetes/${ENV_ID}/namespaces" 2>/dev/null)" = 200 ]; then
    READY=1
    break
  fi
  sleep 3
done
[ -n "${READY}" ] || fail "environment ${ENV_ID} never listed its namespaces"

echo "▶️ Install the Gateway API ${GATEWAY_API_VERSION} CRDs" >&2
kubectl apply -f "https://github.com/kubernetes-sigs/gateway-api/releases/download/${GATEWAY_API_VERSION}/standard-install.yaml" >/dev/null
kubectl wait --for=condition=established crd/gatewayclasses.gateway.networking.k8s.io --timeout=60s >/dev/null
# No controller: the tests only need the objects, not working routing.
printf 'apiVersion: gateway.networking.k8s.io/v1\nkind: GatewayClass\nmetadata:\n  name: e2e\nspec:\n  controllerName: example.com/e2e\n' |
  kubectl apply -f - >/dev/null

echo "export TF_VAR_k8s_environment_id=${ENV_ID}"
if [ -n "${GITHUB_ENV:-}" ]; then
  echo "TF_VAR_k8s_environment_id=${ENV_ID}" >>"${GITHUB_ENV}"
fi
echo "✅ k3d is environment ${ENV_ID}" >&2
