// Command docsdriftlint guards against schema/documentation drift.
//
// The provider's docs (docs/resources/*.md, docs/data-sources/*.md) are
// hand-written prose — their argument descriptions are intentionally NOT verbatim
// copies of the schema `Description` strings, so a text-equality check is not
// meaningful. What DOES drift and cost maintenance time is field parity: someone
// adds, removes or renames a schema field and forgets to update the doc page.
//
// This linter enforces that every top-level schema field of every registered
// resource and data source is mentioned by name in its documentation page. It
// deliberately checks field-name presence (snake_case identifiers are
// distinctive enough to avoid false positives) rather than description wording.
//
// Usage: docsdriftlint [module-root]   (defaults to ".")
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

type entry struct {
	tfName      string // e.g. portainer_tag
	constructor string // e.g. resourceTag
	kind        string // "resources" or "data-sources"
}

type violation struct {
	tfName string
	doc    string
	msg    string
}

func main() {
	flag.Parse()
	root := "."
	if args := flag.Args(); len(args) > 0 {
		root = args[0]
	}

	internalDir := filepath.Join(root, "internal")
	providerFile := filepath.Join(internalDir, "provider.go")

	fset := token.NewFileSet()

	// 1. Read the registration maps: tfName -> constructor, per kind.
	entries, err := parseProviderMaps(fset, providerFile)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docsdriftlint: %v\n", err)
		os.Exit(2)
	}

	// 2. Extract top-level schema field names per constructor function.
	fields, err := parseConstructorFields(fset, internalDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "docsdriftlint: %v\n", err)
		os.Exit(2)
	}

	// 3. For every registered entry, check its doc page mentions each field.
	var violations []violation
	skipped := 0
	for _, e := range entries {
		names, ok := fields[e.constructor]
		if !ok || len(names) == 0 {
			// Constructor builds its schema in a way we can't statically read
			// (e.g. via a helper). Skip rather than false-fail; report count.
			skipped++
			continue
		}
		docName := strings.TrimPrefix(e.tfName, "portainer_")
		docPath := filepath.Join(root, "docs", e.kind, docName+".md")
		content, readErr := os.ReadFile(docPath)
		if readErr != nil {
			violations = append(violations, violation{e.tfName, docPath, "doc page not found"})
			continue
		}
		text := string(content)
		for _, f := range names {
			if !mentionsField(text, f) {
				violations = append(violations, violation{
					e.tfName, docPath,
					fmt.Sprintf("schema field %q is not documented", f),
				})
			}
		}
	}

	if len(violations) == 0 {
		fmt.Printf("docsdriftlint: OK — every schema field is documented (%d entries checked, %d skipped as non-static).\n",
			len(entries)-skipped, skipped)
		return
	}

	sort.Slice(violations, func(i, j int) bool {
		if violations[i].tfName != violations[j].tfName {
			return violations[i].tfName < violations[j].tfName
		}
		return violations[i].msg < violations[j].msg
	})
	for _, v := range violations {
		fmt.Printf("%s (%s): %s\n", v.tfName, v.doc, v.msg)
	}
	fmt.Fprintf(os.Stderr, "\ndocsdriftlint: %d schema/doc drift issue(s). Update the doc page(s) to match the schema.\n", len(violations))
	os.Exit(1)
}

// mentionsField reports whether the doc text references the field name as a
// whole snake_case token (optionally backtick-wrapped), avoiding substring
// false matches like "name" inside "namespace".
func mentionsField(text, field string) bool {
	re := regexp.MustCompile(`(^|[^A-Za-z0-9_])` + regexp.QuoteMeta(field) + `([^A-Za-z0-9_]|$)`)
	return re.MatchString(text)
}

// parseProviderMaps extracts the ResourcesMap and DataSourcesMap entries
// (tfName -> constructor function name) from provider.go.
func parseProviderMaps(fset *token.FileSet, path string) ([]entry, error) {
	f, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	var entries []entry
	ast.Inspect(f, func(n ast.Node) bool {
		kv, ok := n.(*ast.KeyValueExpr)
		if !ok {
			return true
		}
		keyIdent, ok := kv.Key.(*ast.Ident)
		if !ok {
			return true
		}
		var kind string
		switch keyIdent.Name {
		case "ResourcesMap":
			kind = "resources"
		case "DataSourcesMap":
			kind = "data-sources"
		default:
			return true
		}
		mapLit, ok := kv.Value.(*ast.CompositeLit)
		if !ok {
			return true
		}
		for _, elt := range mapLit.Elts {
			ekv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			keyLit, ok := ekv.Key.(*ast.BasicLit)
			if !ok || keyLit.Kind != token.STRING {
				continue
			}
			call, ok := ekv.Value.(*ast.CallExpr)
			if !ok {
				continue
			}
			fnIdent, ok := call.Fun.(*ast.Ident)
			if !ok {
				continue
			}
			entries = append(entries, entry{
				tfName:      strings.Trim(keyLit.Value, `"`),
				constructor: fnIdent.Name,
				kind:        kind,
			})
		}
		return true
	})
	return entries, nil
}

// parseConstructorFields maps each constructor function name to the top-level
// field names of the schema.Resource it returns.
func parseConstructorFields(fset *token.FileSet, dir string) (map[string][]string, error) {
	out := map[string][]string{}

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if d.Name() == "tools" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		f, perr := parser.ParseFile(fset, path, nil, 0)
		if perr != nil {
			return nil // tolerate; schemalint reports parse errors separately
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if names := schemaFieldsInFunc(fn); len(names) > 0 {
				out[fn.Name.Name] = names
			}
		}
		return nil
	})
	return out, err
}

// schemaFieldsInFunc finds the value of the "Schema" key of a schema.Resource
// composite literal inside fn and returns its top-level map keys.
func schemaFieldsInFunc(fn *ast.FuncDecl) []string {
	var names []string
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		cl, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		if !isSchemaResource(cl.Type) {
			return true
		}
		for _, elt := range cl.Elts {
			kv, ok := elt.(*ast.KeyValueExpr)
			if !ok {
				continue
			}
			keyIdent, ok := kv.Key.(*ast.Ident)
			if !ok || keyIdent.Name != "Schema" {
				continue
			}
			schemaMap, ok := kv.Value.(*ast.CompositeLit)
			if !ok {
				continue
			}
			for _, se := range schemaMap.Elts {
				fkv, ok := se.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				if lit, ok := fkv.Key.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					names = append(names, strings.Trim(lit.Value, `"`))
				}
			}
		}
		return false // don't descend into a matched resource lit
	})
	return names
}

// isSchemaResource matches `schema.Resource` and `&schema.Resource` type exprs.
func isSchemaResource(t ast.Expr) bool {
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	sel, ok := t.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "schema" && sel.Sel.Name == "Resource"
}
