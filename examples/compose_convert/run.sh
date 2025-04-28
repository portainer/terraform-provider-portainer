#!/bin/bash

set -euo pipefail

echo "Initializing Terraform..."
terraform init -upgrade

echo "▶️ Step 1: Applying only the compose conversion resource..."
terraform apply -target=portainer_compose_convert.docker-compose-yml -auto-approve

echo "✅ Compose conversion completed."

echo "▶️ Step 2: Applying remaining resources to write output files..."
terraform apply -auto-approve

echo "✅ All done. Kubernetes manifests written to the ./output/ directory."
