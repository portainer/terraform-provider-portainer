// Command sensitivelint reports schema fields whose name implies a secret
// (password, token, api_key, private_key, client_secret, jwt, ...) but that
// are not marked Sensitive: true. Marking such fields sensitive keeps their
// values out of CLI plan output.
//
// A field whose name matches a secret pattern must either set Sensitive: true
// or appear in the allowlist below (for names that look secret-y but are not,
// e.g. a boolean "generate_api_key" toggle or an "..._description" string).
//
// Exit codes:
//
//	0 — no violations
//	1 — one or more fields look like secrets but are not Sensitive
//	2 — invalid invocation or I/O error
//
// Usage:
//
//	sensitivelint [paths...]
//
// Without arguments the tool scans "./internal/".
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
}

// allowlist contains field names that match a secret pattern but are NOT
// sensitive values. Each entry should be obviously non-secret (a toggle, a
// reference name, a description, a public key).
var allowlist = map[string]bool{
	"generate_api_key":    true, // bool toggle, not a key
	"api_key_description": true, // human label for an API key
	"secret_name":         true, // reference to a Docker secret, not its value
	"secret_names":        true,
	"token_name":          true,
	// docker_volume CSI cluster-volume secrets: these are references to Docker
	// Swarm secrets by NAME, never the secret value itself.
	"secrets": true, // TypeList of {key, secret-name} references
	"secret":  true, // name of the Swarm secret providing the value
}

// nameExclusionSuffixes mark a field as non-secret regardless of pattern
// (identifiers, counts, types, names, public material).
var nameExclusionSuffixes = []string{
	"_id", "_ids", "_name", "_names", "_count", "_type",
	"_enabled", "_expiry", "_url", "_uri", "_description",
	"_length", // e.g. required_password_length is a policy int, not a secret
}

func looksSecret(name string) bool {
	if allowlist[name] {
		return false
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
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}

			f, err := parser.ParseFile(fset, path, nil, 0)
			if err != nil {
				fmt.Fprintf(os.Stderr, "parse %s: %v\n", path, err)
				return nil
			}

			ast.Inspect(f, func(n ast.Node) bool {
				cl, ok := n.(*ast.CompositeLit)
				if !ok || !isSchemaMap(cl.Type) {
					return true
				}
				for _, elt := range cl.Elts {
					kv, ok := elt.(*ast.KeyValueExpr)
					if !ok {
						continue
					}
					name := fieldNameFromKey(kv.Key)
					if !looksSecret(name) {
						continue
					}
					inner := unwrapSchemaLit(kv.Value)
					if inner == nil || hasSensitiveTrue(inner) {
						continue
					}
					pos := fset.Position(inner.Pos())
					violations = append(violations, violation{file: pos.Filename, line: pos.Line, name: name})
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
		fmt.Println("sensitivelint: OK — every secret-like field is marked Sensitive.")
		return
	}

	sort.Slice(violations, func(i, j int) bool {
		if violations[i].file != violations[j].file {
			return violations[i].file < violations[j].file
		}
		return violations[i].line < violations[j].line
	})
	for _, v := range violations {
		fmt.Printf("%s:%d: secret-like field %q is not marked Sensitive: true\n", v.file, v.line, v.name)
	}
	fmt.Fprintf(os.Stderr, "\nsensitivelint: %d secret-like field(s) missing Sensitive: true.\n", len(violations))
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

func hasSensitiveTrue(cl *ast.CompositeLit) bool {
	for _, elt := range cl.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		ident, ok := kv.Key.(*ast.Ident)
		if !ok || ident.Name != "Sensitive" {
			continue
		}
		lit, ok := kv.Value.(*ast.Ident)
		return ok && lit.Name == "true"
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
	return ""
}
