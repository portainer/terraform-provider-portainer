#!/usr/bin/env bash

set -e

# Generate terraform state in JSON
terraform show -json > imported.json

echo "Generating generated.tf..."

# Start with clean file
echo "# Auto-generated resource definitions from terraform import" > generated.tf

# Loop over each resource in state and convert it to HCL
jq -r '
  .values.root_module.resources[] |
  select(.type | startswith("portainer_")) |
  "resource \"\(.type)\" \"\(.name)\" {\n" +
  (
    .values | to_entries[] |
    select(.key != "id") |
    "  \(.key) = " + (
      if (.value | type) == "string" then
        "\"\(.value)\""
      elif (.value | type) == "number" or (.value | type) == "boolean" then
        "\(.value)"
      elif (.value | type) == "object" or (.value | type) == "array" then
        (.value | @json)
      else
        "\"\(.value)\""
      end
    ) + "\n"
  ) +
  "}\n"
' imported.json >> generated.tf

echo "✅ Output saved to generated.tf"
