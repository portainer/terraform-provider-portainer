// Command sensitivelint reports schema attributes whose name implies a secret
// (password, token, api_key, private_key, client_secret, jwt, ...) but that
// are not marked Sensitive: true. Marking such fields sensitive keeps their
// values out of CLI plan output.
//
// It reads the schemas as Terraform receives them, from the provider server
// (internal.NewProviderServer, GetProviderSchema): the provider block and
// every resource, data source, ephemeral resource, list resource and action,
// attributes of nested blocks included.
//
// A field whose name matches a secret pattern must either set Sensitive: true
// or appear in the allowlist below (for names that look secret-y but are not,
// e.g. a boolean "generate_api_key" toggle or an "..._description" string).
//
// Exit codes:
//
//	0 — no violations
//	1 — one or more fields look like secrets but are not Sensitive
//	2 — the provider schema could not be loaded
//
// Usage:
//
//	sensitivelint [paths...]
//
// The paths (CI passes ./internal/) are accepted for compatibility and
// ignored: the schema comes from the compiled provider.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/portainer/terraform-provider-portainer/internal"
)

// secretPatterns are substrings that, when present in a field name, suggest
// the field carries a secret value.
var secretPatterns = []string{
	"password", "passwd", "passphrase",
	"token",
	"secret",
	"api_key", "apikey",
	"private_key", "privatekey",
	"access_key", "accesskey",
	"client_secret",
	"jwt",
	"credential",
	"edge_key", // Edge Agent join key: grants an agent access to the Portainer instance
	"tls_key",  // PEM private key
}

// secretNames are exact field names that carry a secret although an exclusion
// suffix below would otherwise clear them.
var secretNames = map[string]bool{
	"webhook_url": true, // unauthenticated URL that redeploys the stack: a bearer capability
}

// allowlist contains field names that match a secret pattern but are NOT
// sensitive values. Each entry should be obviously non-secret (a toggle, a
// reference name, a description, a public key).
var allowlist = map[string]bool{
	"generate_api_key":     true, // bool toggle, not a key
	"api_key_description":  true, // human label for an API key
	"requires_setup_token": true, // bool toggle: whether a token is demanded, not the token
	"setup_token_required": true,
	"secret_name":          true, // reference to a Docker secret, not its value
	"secret_names":         true,
	"token_name":           true,
	"restrict_secrets":     true, // bool toggle: whether access to secrets is restricted, not a secret
	"include_secrets":      true, // bool toggle: whether a scoped kubeconfig may read Secrets
	// Kubernetes service accounts: a toggle, and a list of secret NAMES.
	"automount_service_account_token": true, // bool toggle: whether the token is mounted, not the token
	"image_pull_secrets":              true, // references to secrets by name, never their values
	// docker_volume CSI cluster-volume secrets: these are references to Docker
	// Swarm secrets by NAME, never the secret value itself.
	"secrets": true, // TypeList of {key, secret-name} references
	"secret":  true, // name of the Swarm secret providing the value
	// OIDC provider: times and durations of secrets and tokens, not their values.
	"secret_expires_at":               true, // Unix time the first OIDC client secret expires
	"old_secret_grace_period":         true, // duration a regenerated OIDC client secret keeps working
	"access_token_lifetime":           true, // duration OIDC access tokens last
	"refresh_token_idle_lifetime":     true, // duration an OIDC session lasts unused
	"refresh_token_absolute_lifetime": true, // duration an OIDC session lasts at most
}

// nameExclusionSuffixes mark a field as non-secret regardless of pattern
// (identifiers, counts, types, names, public material).
var nameExclusionSuffixes = []string{
	"_id", "_ids", "_name", "_names", "_count", "_type",
	"_enabled", "_expiry", "_url", "_uri", "_description",
	"_length",  // e.g. required_password_length is a policy int, not a secret
	"_version", // e.g. password_wo_version counts write-only updates, it is not the value
}

func looksSecret(name string) bool {
	if allowlist[name] {
		return false
	}
	if secretNames[name] {
		return true
	}
	if strings.Contains(name, "public") {
		return false
	}
	for _, suf := range nameExclusionSuffixes {
		if strings.HasSuffix(name, suf) {
			return false
		}
	}
	for _, p := range secretPatterns {
		if strings.Contains(name, p) {
			return true
		}
	}
	return false
}

func main() {
	flag.Parse()

	schemas, err := internal.LintSchemas(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "sensitivelint: %v\n", err)
		os.Exit(2)
	}

	violations := check(schemas)
	if len(violations) == 0 {
		fmt.Println("sensitivelint: OK — every secret-like field is marked Sensitive.")
		return
	}
	for _, v := range violations {
		fmt.Println(v)
	}
	fmt.Fprintf(os.Stderr, "\nsensitivelint: %d secret-like field(s) missing Sensitive: true.\n", len(violations))
	os.Exit(1)
}

// check returns one line per secret-like attribute not marked Sensitive.
// Nested blocks carry no Sensitive flag in the protocol, so only attributes
// (top-level or inside blocks) are checked; an attribute is matched by its own
// name, as the field names of a nested schema were before.
func check(schemas []internal.LintSchema) []string {
	var out []string
	for _, s := range schemas {
		for _, f := range s.Fields() {
			// Action attributes cannot be Sensitive (and the framework rejects
			// write-only ones): a secret there is as sensitive as the
			// expression that sets it.
			if s.Kind == "action" {
				continue
			}
			if f.Block || f.Sensitive || !looksSecret(f.Name) {
				continue
			}
			out = append(out, fmt.Sprintf("%s %s: secret-like field %q is not marked Sensitive: true", s.Kind, s.TypeName, f.Path))
		}
	}
	return out
}
