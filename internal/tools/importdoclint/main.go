// Command importdoclint reports Terraform resources that declare an Importer
// but whose documentation page has no Import section.
//
// A resource is "importable" when it implements
// resource.ResourceWithImportState (see
// internal.ImportableResourceTypes). Importable resources must document how
// to import them (an "## Import" heading) in docs/resources/<name>.md, where
// <name> is the Terraform resource name minus the "portainer_" prefix.
//
// Exit codes:
//
//	0 — every importable resource documents import
//	1 — one or more importable resources lack an Import section
//	2 — invalid invocation or I/O error
//
// Usage:
//
//	importdoclint [root]
//
// Without arguments the tool assumes the repository root is the working dir
// and reads ./docs.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/portainer/terraform-provider-portainer/internal"
)

var importHeading = regexp.MustCompile(`(?mi)^#{1,6}\s+.*import`)

func main() {
	flag.Parse()
	root := "."
	if args := flag.Args(); len(args) > 0 {
		root = args[0]
	}

	importable := internal.ImportableResourceTypes(context.Background())
	if len(importable) == 0 {
		fmt.Fprintln(os.Stderr, "importdoclint: no importable resources found in the provider")
		os.Exit(2)
	}

	missing := check(filepath.Join(root, "docs", "resources"), importable)
	if len(missing) == 0 {
		fmt.Println("importdoclint: OK — every importable resource documents import.")
		return
	}
	for _, m := range missing {
		fmt.Println(m)
	}
	fmt.Fprintf(os.Stderr, "\nimportdoclint: %d importable resource(s) without an Import section.\n", len(missing))
	os.Exit(1)
}

// check returns one sorted line per importable resource whose doc page in
// docsDir is missing or has no Import section.
func check(docsDir string, importable map[string]bool) []string {
	var missing []string
	for tfName := range importable {
		docName := strings.TrimPrefix(tfName, "portainer_")
		docPath := filepath.Join(docsDir, docName+".md")
		content, err := os.ReadFile(docPath)
		if err != nil {
			missing = append(missing, fmt.Sprintf("%s (%s): docs file missing", tfName, docPath))
			continue
		}
		if !importHeading.Match(content) {
			missing = append(missing, fmt.Sprintf("%s (%s): no Import section", tfName, docPath))
		}
	}
	sort.Strings(missing)
	return missing
}
