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
// In the other direction, every field listed in a doc page's argument/attribute
// table (header "Name", "Attribute" or "Field") must exist in the schema, at
// the top level or in a nested block, so renamed fields do not linger.
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
	fields, allFields, err := parseConstructorFields(fset, internalDir)
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
		// The other direction: a field listed in an argument/attribute table
		// must exist somewhere in the schema (top-level or nested), so renamed
		// or removed fields do not linger in the docs.
		known := allFields[e.constructor]
		for _, f := range docTableFields(text) {
			if !known[f] {
				violations = append(violations, violation{
					e.tfName, docPath,
					fmt.Sprintf("documented field %q is not in the schema", f),
				})
			}
		}
	}

	if len(violations) == 0 {
		fmt.Printf("docsdriftlint: OK — every schema field is documented and every documented field exists (%d entries checked, %d skipped as non-static).\n",
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
// field names of the schema.Resource it returns, and to every field name
// (top-level or nested) declared anywhere in the function.
func parseConstructorFields(fset *token.FileSet, dir string) (map[string][]string, map[string]map[string]bool, error) {
	out := map[string][]string{}
	all := map[string]map[string]bool{}
	funcs := map[string]*ast.FuncDecl{}

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
			return nil //nolint:nilerr // tolerate; schemalint reports parse errors separately
		}
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				continue
			}
			if fn.Recv == nil {
				funcs[fn.Name.Name] = fn
			}
			if names := schemaFieldsInFunc(fn); len(names) > 0 {
				out[fn.Name.Name] = names
			}
		}
		return nil
	})
	for name := range out {
		if fn := funcs[name]; fn != nil {
			all[name] = allSchemaKeysInFunc(fn, funcs, map[string]bool{})
		}
	}
	return out, all, err
}

// allSchemaKeysInFunc returns every string key of a map[string]*schema.Schema
// literal in fn, including the ones of nested blocks, plus "id". Keys of the
// package functions fn calls count too, so a nested block built by a shared
// helper (Elem: someElem()) is seen.
func allSchemaKeysInFunc(fn *ast.FuncDecl, funcs map[string]*ast.FuncDecl, seen map[string]bool) map[string]bool {
	keys := map[string]bool{"id": true}
	seen[fn.Name.Name] = true
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		if call, ok := n.(*ast.CallExpr); ok {
			if id, ok := call.Fun.(*ast.Ident); ok && funcs[id.Name] != nil && !seen[id.Name] {
				for k := range allSchemaKeysInFunc(funcs[id.Name], funcs, seen) {
					keys[k] = true
				}
			}
			return true
		}
		cl, ok := n.(*ast.CompositeLit)
		if !ok {
			return true
		}
		mt, ok := cl.Type.(*ast.MapType)
		if !ok || !isSchemaSchemaPtr(mt.Value) {
			// The Schema field of a schema.Resource literal has an implicit
			// type, so recognise it through its parent key below as well.
			if !isSchemaResource(cl.Type) {
				return true
			}
			for _, elt := range cl.Elts {
				if kv, ok := elt.(*ast.KeyValueExpr); ok {
					if id, ok := kv.Key.(*ast.Ident); ok && id.Name == "Schema" {
						if m, ok := kv.Value.(*ast.CompositeLit); ok {
							addStringKeys(keys, m)
						}
					}
				}
			}
			return true
		}
		addStringKeys(keys, cl)
		return true
	})
	return keys
}

func addStringKeys(keys map[string]bool, cl *ast.CompositeLit) {
	for _, elt := range cl.Elts {
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			if lit, ok := kv.Key.(*ast.BasicLit); ok && lit.Kind == token.STRING {
				keys[strings.Trim(lit.Value, `"`)] = true
			}
		}
	}
}

// isSchemaSchemaPtr matches the `*schema.Schema` type expression.
func isSchemaSchemaPtr(t ast.Expr) bool {
	star, ok := t.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	return ok && pkg.Name == "schema" && sel.Sel.Name == "Schema"
}

var (
	docTableHeader = regexp.MustCompile(`^\|\s*(Name|Attribute|Field)\s*\|`)
	docTableField  = regexp.MustCompile("^\\|\\s*`([a-z][a-z0-9_]*)`\\s*\\|")
)

// docTableFields returns the field names listed in the first column of the
// argument/attribute tables (header "Name", "Attribute" or "Field").
func docTableFields(text string) []string {
	var names []string
	inTable := false
	for _, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case docTableHeader.MatchString(line):
			inTable = true
		case !strings.HasPrefix(line, "|"):
			inTable = false
		case inTable:
			if m := docTableField.FindStringSubmatch(line); m != nil {
				names = append(names, m[1])
			}
		}
	}
	return names
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
