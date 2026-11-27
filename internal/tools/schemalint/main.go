// Command schemalint reports attributes and nested blocks of the provider's
// schemas that have no Description.
//
// It reads the schemas as Terraform receives them, from the provider server
// (internal.NewProviderServer, GetProviderSchema): the provider block and
// every resource, data source, ephemeral resource, list resource and action,
// nested blocks included. The "timeouts" block of
// terraform-plugin-framework-timeouts, which cannot carry a description, is
// exempt. So are the fields of an attribute's object type, which carry no
// description in the protocol.
//
// Exit codes:
//
//	0 — no violations
//	1 — one or more fields lack a Description
//	2 — the provider schema could not be loaded
//
// Usage:
//
//	schemalint [paths...]
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

func main() {
	flag.Parse()

	schemas, err := internal.LintSchemas(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "schemalint: %v\n", err)
		os.Exit(2)
	}

	violations := check(schemas)
	if len(violations) == 0 {
		fmt.Println("schemalint: OK — every schema field has a Description.")
		return
	}
	for _, v := range violations {
		fmt.Println(v)
	}
	fmt.Fprintf(os.Stderr, "\nschemalint: %d schema field(s) without Description.\n", len(violations))
	os.Exit(1)
}

// check returns one line per attribute or block without a Description.
func check(schemas []internal.LintSchema) []string {
	var out []string
	for _, s := range schemas {
		for _, f := range s.Fields() {
			if f.Implicit || f.TypeOnly || strings.TrimSpace(f.Description) != "" {
				continue
			}
			out = append(out, fmt.Sprintf("%s %s: schema field %q missing Description", s.Kind, s.TypeName, f.Path))
		}
	}
	return out
}
