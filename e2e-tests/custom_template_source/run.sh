#!/bin/bash
# source_id (Portainer 2.43+) builds a custom template from a Portainer Source
# instead of an inline repository URL. Every update has to name the Source
# again, or Portainer finds neither content nor a repository and rejects it.
# shellcheck source=e2e-tests/lib/e2e.sh
source "$(dirname "$0")/../lib/e2e.sh"
cd "$(dirname "$0")" || exit 1

tf_prepare

step "Create a Source and a template from it"
terraform apply -auto-approve
TEMPLATE_ID="$(terraform output -raw template_id)"
url="$(api GET "/custom_templates/${TEMPLATE_ID}" | jq -r '.GitConfig.URL // empty')"
[ -n "${url}" ] || fail "the template has no Git configuration: it was not built from the Source"
echo "   the template clones ${url}"
expect_plan 0 "no diff after create (the .git suffix of the Source URL included)"

step "Update the template"
UPDATE=(-var="template_description=Custom template created from a Portainer Source, updated")
terraform apply -auto-approve "${UPDATE[@]}"
expect_plan 0 "no diff after the update" "${UPDATE[@]}"

step "Clean up"
terraform destroy -auto-approve "${UPDATE[@]}"

echo "✅ A custom template can be created and updated from a Portainer Source."
