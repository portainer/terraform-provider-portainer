package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type violation struct {
	file string
	line int
	name string
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) == 0 {
		args = []string{"./internal/"}
	}

	fset := token.NewFileSet()
	var violations []violation

	for _, root := range args {
		err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				name := d.Name()
				if name == "vendor" || name == "tools" || strings.HasPrefix(name, ".") {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}

			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				fmt.Fprintf(os.Stderr, "parse %s: %v\n", path, err)
				return nil
			}

			ast.Inspect(f, func(n ast.Node) bool {
				cl, ok := n.(*ast.CompositeLit)
				if !ok {
					return true
				}
				if !isSchemaMap(cl.Type) {
					return true
				}
				for _, elt := range cl.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					inner := unwrapSchemaLit(kv.Value)
					if inner == nil {
						continue
					}
					if hasNonEmptyDescription(inner) {
						continue
					}
					pos := fset.Position(inner.Pos())
					violations = append(violations, violation{
						file: pos.Filename,
						line: pos.Line,
						name: fieldNameFromKey(kv.Key),
					})
				}
				return true
			})

			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "walk %s: %v\n", root, err)
			os.Exit(2)
		}
	}

	if len(violations) == 0 {
		fmt.Println("schemalint: OK — every schema field has a Description.")
		return
	}

	sort.Slice(violations, func(i, j int) bool {
		if violations[i].file != violations[j].file {
			return violations[i].file < violations[j].file
		}
		return violations[i].line < violations[j].line
	})

	for _, v := range violations {
		fmt.Printf("%s:%d: schema field %q missing Description\n", v.file, v.line, v.name)
	}
	fmt.Fprintf(os.Stderr, "\nschemalint: %d schema field(s) without Description.\n", len(violations))
	os.Exit(1)
}

func isSchemaMap(t ast.Expr) bool {
	mt, ok := t.(*ast.MapType)
	if !ok {
		return false
	}
	keyIdent, ok := mt.Key.(*ast.Ident)
	if !ok || keyIdent.Name != "string" {
		return false
	}
	star, ok := mt.Value.(*ast.StarExpr)
	if !ok {
		return false
	}
	sel, ok := star.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	pkg, ok := sel.X.(*ast.Ident)
	if !ok || pkg.Name != "schema" {
		return false
	}
	return sel.Sel.Name == "Schema"
}

func unwrapSchemaLit(e ast.Expr) *ast.CompositeLit {
	switch v := e.(type) {
	case *ast.CompositeLit:
		return v
	case *ast.UnaryExpr:
		if cl, ok := v.X.(*ast.CompositeLit); ok {
			return cl
		}
	}
	return nil
}

func hasNonEmptyDescription(cl *ast.CompositeLit) bool {
	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		ident, ok := kv.Key.(*ast.Ident)
		if !ok || ident.Name != "Description" {
			continue
		}
		bl, ok := kv.Value.(*ast.BasicLit)
		if !ok {
			return true
		}
		raw := strings.Trim(bl.Value, "`\"")
		return strings.TrimSpace(raw) != ""
	}
	return false
}

func fieldNameFromKey(k ast.Expr) string {
	switch v := k.(type) {
	case *ast.BasicLit:
		return strings.Trim(v.Value, "`\"")
	case *ast.Ident:
		return v.Name
	}
	return "<unknown>"
}
