// Command typenames prints the type names the provider serves, one
// "<kind> <name>" per line (kinds as internal.LintSchemas reports them, with
// spaces turned into dashes: resource, data-source, ephemeral-resource,
// list-resource, action), for shell scripts that check docs and examples.
//
// Usage: typenames [kind]   (prints every kind when none is given)
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/portainer/terraform-provider-portainer/internal"
)

func main() {
	schemas, err := internal.LintSchemas(context.Background())
	if err != nil {
		fmt.Fprintf(os.Stderr, "typenames: %v\n", err)
		os.Exit(2)
	}
	only := ""
	if len(os.Args) > 1 {
		only = os.Args[1]
	}
	for _, s := range schemas {
		kind := strings.ReplaceAll(s.Kind, " ", "-")
		if kind == "provider" || (only != "" && kind != only) {
			continue
		}
		fmt.Println(kind, s.TypeName)
	}
}
