// Command importdoclint reports Terraform resources that declare an Importer
// but whose documentation page has no Import section.
//
// A resource is "importable" when its constructor returns a *schema.Resource
// whose literal sets a non-nil Importer field. Importable resources must
// document how to import them (an "## Import" heading) in
// docs/resources/<name>.md, where <name> is the Terraform resource name
// (from provider.go) minus the "portainer_" prefix.
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
// and scans ./internal + ./docs.
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var importHeading = regexp.MustCompile(`(?mi)^#{1,6}\s+.*import`)

func main() {
	flag.Parse()
	root := "."
	if args := flag.Args(); len(args) > 0 {
		root = args[0]
	}

	internalDir := filepath.Join(root, "internal")
	providerFile := filepath.Join(internalDir, "provider.go")
	docsDir := filepath.Join(root, "docs", "resources")

	fset := token.NewFileSet()

	// 1. tfName -> constructor function name, from the ResourcesMap block.
	tfToCtor, err := parseResourcesMap(providerFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "importdoclint: %v\n", err)
		os.Exit(2)
	}
	if len(tfToCtor) == 0 {
		fmt.Fprintln(os.Stderr, "importdoclint: no resources parsed from provider.go (parser out of sync?)")
		os.Exit(2)
	}

	// 2. Set of constructor names whose returned resource sets Importer.
	importable, err := importableConstructors(fset, internalDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "importdoclint: %v\n", err)
		os.Exit(2)
	}

	// 3. For each importable resource, require an Import section in its doc.
	var missing []string
	for tfName, ctor := range tfToCtor {
		if !importable[ctor] {
			continue
		}
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

	if len(missing) == 0 {
		fmt.Println("importdoclint: OK — every importable resource documents import.")
		return
	}

	sort.Strings(missing)
	for _, m := range missing {
		fmt.Println(m)
	}
	fmt.Fprintf(os.Stderr, "\nimportdoclint: %d importable resource(s) without an Import section.\n", len(missing))
	os.Exit(1)
}

// parseResourcesMap extracts "portainer_x": constructorIdent pairs from the
// ResourcesMap block in provider.go.
func parseResourcesMap(providerFile string) (map[string]string, error) {
	fset := token.NewFileSet()
	f, err := parser.ParseFile(fset, providerFile, nil, 0)
	if err != nil {
		return nil, err
	}

	out := map[string]string{}
	ast.Inspect(f, func(n ast.Node) bool {
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != "ResourcesMap" {
			return true
		}
		mapLit, ok := kv.Value.(*ast.CompositeLit)
		if !ok {
			return true
		}
		for _, elt := range mapLit.Elts {
			entry, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			name, ok := entry.Key.(*ast.BasicLit)
			if !ok {
				continue
			}
			tfName := strings.Trim(name.Value, "`\"")
			if ctor := calleeName(entry.Value); ctor != "" {
				out[tfName] = ctor
			}
		}
		return false
	})
	return out, nil
}

// calleeName returns the function name of a call expression like resourceTag().
func calleeName(e ast.Expr) string {
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return ""
	}
	if id, ok := call.Fun.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// importableConstructors scans internal/*.go for functions whose body contains
// a composite literal setting a non-nil Importer field, and returns the set of
// such function names.
func importableConstructors(fset *token.FileSet, internalDir string) (map[string]bool, error) {
	out := map[string]bool{}
	entries, err := os.ReadDir(internalDir)
	if err != nil {
		return nil, err
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".go") || strings.HasSuffix(e.Name(), "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, filepath.Join(internalDir, e.Name()), nil, 0)
		if err != nil {
			return nil, err
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if funcSetsImporter(fn) {
				out[fn.Name.Name] = true
			}
		}
	}
	return out, nil
}

// funcSetsImporter reports whether fn's body contains an "Importer:" field
// assigned a non-nil value.
func funcSetsImporter(fn *ast.FuncDecl) bool {
	found := false
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		key, ok := kv.Key.(*ast.Ident)
		if !ok || key.Name != "Importer" {
			return true
		}
		if id, ok := kv.Value.(*ast.Ident); ok && id.Name == "nil" {
			return true
		}
		found = true
		return false
	})
	return found
}
