package main

// The field check: for every request body of an operation on a covered path,
// the top-level properties the provider never sends.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// bodyTypes are the request content types whose schema properties the
// provider has to send by name. Other types (raw files, text) have no fields.
var bodyTypes = []string{"application/json", "multipart/form-data", "application/x-www-form-urlencoded"}

var methods = []string{"get", "put", "post", "delete", "options", "head", "patch"}

// operation is one spec operation with a request body and its top-level
// request fields.
type operation struct {
	Method string // upper case
	Path   string
	Fields []string
}

// key identifies one request field of one operation.
func (o operation) key(field string) string { return o.Method + " " + o.Path + " " + field }

// specDoc is a parsed YAML file of a spec. Files are cached by absolute path
// because ~47 paths refer to the same per-area file.
type specDoc struct {
	file string
	root *yaml.Node
}

type specLoader struct{ docs map[string]*specDoc }

func (l *specLoader) load(file string) (*specDoc, error) {
	abs, err := filepath.Abs(file)
	if err != nil {
		return nil, err
	}
	if d, ok := l.docs[abs]; ok {
		return d, nil
	}
	data, err := os.ReadFile(abs)
	if err != nil {
		return nil, err
	}
	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, fmt.Errorf("%s: %w", file, err)
	}
	if root.Kind != yaml.DocumentNode || len(root.Content) == 0 {
		return nil, fmt.Errorf("%s: empty YAML document", file)
	}
	d := &specDoc{file: abs, root: root.Content[0]}
	l.docs[abs] = d
	return d, nil
}

// resolve follows a "$ref" ("other.yaml#/a/b", "#/a/b") relative to the file
// of doc. The tool never fetches anything: a referenced file must sit next to
// the file that refers to it.
func (l *specLoader) resolve(doc *specDoc, ref string) (*specDoc, *yaml.Node, error) {
	file, pointer, _ := strings.Cut(ref, "#")
	target := doc
	if file != "" {
		if strings.Contains(file, "://") {
			return nil, nil, fmt.Errorf("%s: remote $ref %q is not supported; download it next to the spec", doc.file, ref)
		}
		var err error
		if target, err = l.load(filepath.Join(filepath.Dir(doc.file), file)); err != nil {
			return nil, nil, err
		}
	}
	n := target.root
	for _, tok := range strings.Split(strings.TrimPrefix(pointer, "/"), "/") {
		if tok == "" {
			continue
		}
		tok = strings.ReplaceAll(strings.ReplaceAll(tok, "~1", "/"), "~0", "~")
		if n = mapValue(n, tok); n == nil {
			return nil, nil, fmt.Errorf("%s: $ref %q does not resolve", doc.file, ref)
		}
	}
	return target, n, nil
}

// deref follows $ref chains of a node.
func (l *specLoader) deref(doc *specDoc, n *yaml.Node) (*specDoc, *yaml.Node, error) {
	for i := 0; n != nil && i < 32; i++ {
		ref := mapValue(n, "$ref")
		if ref == nil {
			return doc, n, nil
		}
		var err error
		if doc, n, err = l.resolve(doc, ref.Value); err != nil {
			return nil, nil, err
		}
	}
	return doc, n, nil
}

// mapValue returns the value of key in a YAML mapping, or nil.
func mapValue(n *yaml.Node, key string) *yaml.Node {
	if n == nil || n.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(n.Content); i += 2 {
		if n.Content[i].Value == key {
			return n.Content[i+1]
		}
	}
	return nil
}

// readSpecOperations returns the operations of an openapi.yaml that have a
// JSON or form request body, with their top-level request properties. Path
// items and schemas may $ref other files in the spec's directory.
func readSpecOperations(file string) ([]operation, error) {
	l := &specLoader{docs: map[string]*specDoc{}}
	doc, err := l.load(file)
	if err != nil {
		return nil, err
	}
	paths := mapValue(doc.root, "paths")
	if paths == nil {
		return nil, nil
	}
	var ops []operation
	for i := 0; i+1 < len(paths.Content); i += 2 {
		path := paths.Content[i].Value
		itemDoc, item, err := l.deref(doc, paths.Content[i+1])
		if err != nil {
			return nil, err
		}
		for _, m := range methods {
			op := mapValue(item, m)
			if op == nil {
				continue
			}
			fields, hasBody, err := l.requestFields(itemDoc, mapValue(op, "requestBody"))
			if err != nil {
				return nil, fmt.Errorf("%s %s: %w", strings.ToUpper(m), path, err)
			}
			if hasBody {
				ops = append(ops, operation{Method: strings.ToUpper(m), Path: path, Fields: fields})
			}
		}
	}
	return ops, nil
}

// requestFields lists the properties of the first JSON or form schema of a
// requestBody. hasBody is false when the body is absent or of another type.
func (l *specLoader) requestFields(doc *specDoc, body *yaml.Node) (fields []string, hasBody bool, err error) {
	if body == nil {
		return nil, false, nil
	}
	if doc, body, err = l.deref(doc, body); err != nil {
		return nil, false, err
	}
	content := mapValue(body, "content")
	for _, t := range bodyTypes {
		media := mapValue(content, t)
		if media == nil {
			continue
		}
		set := map[string]bool{}
		if err := l.schemaProperties(doc, mapValue(media, "schema"), set, 0); err != nil {
			return nil, false, err
		}
		for f := range set {
			fields = append(fields, f)
		}
		sort.Strings(fields)
		return fields, true, nil
	}
	return nil, false, nil
}

// schemaProperties adds the property names of a schema, following $ref and
// merging allOf, oneOf and anyOf. Arrays and free-form maps have none.
func (l *specLoader) schemaProperties(doc *specDoc, s *yaml.Node, into map[string]bool, depth int) error {
	if s == nil || depth > 16 {
		return nil
	}
	doc, s, err := l.deref(doc, s)
	if err != nil {
		return err
	}
	if props := mapValue(s, "properties"); props != nil && props.Kind == yaml.MappingNode {
		for i := 0; i < len(props.Content); i += 2 {
			into[props.Content[i].Value] = true
		}
	}
	for _, comb := range []string{"allOf", "oneOf", "anyOf"} {
		list := mapValue(s, comb)
		if list == nil || list.Kind != yaml.SequenceNode {
			continue
		}
		for _, sub := range list.Content {
			if err := l.schemaProperties(doc, sub, into, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// providerFieldNames collects, lower-cased, every name the provider can send
// in a request body, from its non-test Go files:
//   - json struct tag names,
//   - string keys of composite literals (map[string]any{"Name": ...}) and
//     the string values of map[string]string literals, which map attribute
//     names to API field names,
//   - string indexes assigned to (body["Name"] = ...),
//   - multipart field names: the first string argument of WriteField,
//     CreateFormFile and helpers named like addField, the first element of
//     [2]string{"Name", value} pairs and Field: "Name" in multipartFile.
//
// Portainer decodes JSON case-insensitively, hence the lower case. The check
// is per name, not per request: a field counts as sent when any request of
// the provider uses that name.
func providerFieldNames(dir string) (map[string]bool, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	names := map[string]bool{}
	add := func(e ast.Expr) {
		if lit, ok := e.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if s, err := strconv.Unquote(lit.Value); err == nil && s != "" && !strings.ContainsAny(s, " /\n\t") {
				names[strings.ToLower(s)] = true
			}
		}
	}
	fset := token.NewFileSet()
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			return nil, err
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Field:
				if x.Tag == nil {
					return true
				}
				tag, err := strconv.Unquote(x.Tag.Value)
				if err != nil {
					return true
				}
				if name, _, _ := strings.Cut(reflect.StructTag(tag).Get("json"), ","); name != "" && name != "-" {
					names[strings.ToLower(name)] = true
				}
			case *ast.KeyValueExpr:
				add(x.Key)
				if id, ok := x.Key.(*ast.Ident); ok && id.Name == "Field" {
					add(x.Value)
				}
			case *ast.AssignStmt:
				for _, lhs := range x.Lhs {
					if ix, ok := lhs.(*ast.IndexExpr); ok {
						add(ix.Index)
					}
				}
			case *ast.CallExpr:
				if isFieldWriter(x.Fun) && len(x.Args) > 0 {
					add(x.Args[0])
				}
			case *ast.CompositeLit:
				// map[string]string{"storage_account_key": "StorageAccountKey"}
				// maps attribute names to API field names.
				if m, ok := x.Type.(*ast.MapType); ok {
					if v, ok := m.Value.(*ast.Ident); ok && v.Name == "string" {
						for _, elt := range x.Elts {
							if kv, ok := elt.(*ast.KeyValueExpr); ok {
								add(kv.Value)
							}
						}
					}
				}
				if arr, ok := x.Type.(*ast.ArrayType); ok && arr.Len != nil && len(x.Elts) == 2 {
					if elt, ok := arr.Elt.(*ast.Ident); ok && elt.Name == "string" {
						add(x.Elts[0])
					}
				}
			}
			return true
		})
	}
	return names, nil
}

// isFieldWriter reports whether a call writes a multipart field whose name is
// the first argument: WriteField, CreateFormFile, CreateFormField or a local
// helper named like addField.
func isFieldWriter(fun ast.Expr) bool {
	var name string
	switch f := fun.(type) {
	case *ast.Ident:
		name = f.Name
	case *ast.SelectorExpr:
		name = f.Sel.Name
	}
	switch name {
	case "WriteField", "CreateFormFile", "CreateFormField":
		return true
	}
	lower := strings.ToLower(name)
	return strings.HasPrefix(lower, "add") && strings.HasSuffix(lower, "field")
}

// fieldIgnore is one line of ignore-fields.txt. Method, Path and Field may be
// "*"; Field compares case-insensitively.
type fieldIgnore struct {
	Method, Path, Field string
	line                int
	used                bool
}

func (ig *fieldIgnore) matches(method, path, field string) bool {
	return (ig.Method == "*" || ig.Method == method) &&
		(ig.Path == "*" || ig.Path == path) &&
		(ig.Field == "*" || strings.EqualFold(ig.Field, field))
}

func (ig *fieldIgnore) String() string { return ig.Method + " " + ig.Path + " " + ig.Field }

// readFieldIgnore reads ignore-fields.txt: "<METHOD> <path> <field>  # reason"
// per line, the reason required.
func readFieldIgnore(file string) ([]*fieldIgnore, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	var out []*fieldIgnore
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		entry, reason, _ := strings.Cut(line, "#")
		if strings.TrimSpace(reason) == "" {
			return nil, fmt.Errorf("%s:%d: %q has no reason after #", file, i+1, strings.TrimSpace(entry))
		}
		parts := strings.Fields(entry)
		if len(parts) != 3 {
			return nil, fmt.Errorf("%s:%d: want \"<METHOD> <path> <field>  # reason\", got %q", file, i+1, strings.TrimSpace(entry))
		}
		out = append(out, &fieldIgnore{Method: strings.ToUpper(parts[0]), Path: parts[1], Field: parts[2], line: i + 1})
	}
	return out, nil
}

// fieldReport is the outcome of the field check.
type fieldReport struct {
	Operations, Fields, Sent, Ignored int
	Missing                           []string // "<METHOD> <path> <field>"
	Stale                             []string // ignore-fields.txt entries that match no unsent field
}

// checkFields compares the request fields of the operations on covered paths
// with the names the provider sends. ops may hold the same operation from
// several specs; each field is counted once.
func checkFields(ops []operation, patterns [][]string, sent map[string]bool, ignores []*fieldIgnore) fieldReport {
	var r fieldReport
	seenOp := map[string]bool{}
	seenField := map[string]bool{}
	for _, op := range ops {
		if !matchesAny(op.Path, patterns) {
			continue
		}
		if !seenOp[op.Method+" "+op.Path] {
			seenOp[op.Method+" "+op.Path] = true
			r.Operations++
		}
		for _, f := range op.Fields {
			k := op.key(f)
			if seenField[k] {
				continue
			}
			seenField[k] = true
			r.Fields++
			if sent[strings.ToLower(f)] {
				r.Sent++
				continue
			}
			ignored := false
			for _, ig := range ignores {
				if ig.matches(op.Method, op.Path, f) {
					ig.used = true
					ignored = true
				}
			}
			if ignored {
				r.Ignored++
			} else {
				r.Missing = append(r.Missing, k)
			}
		}
	}
	for _, ig := range ignores {
		if !ig.used {
			r.Stale = append(r.Stale, ig.String())
		}
	}
	sort.Strings(r.Missing)
	sort.Strings(r.Stale)
	return r
}
