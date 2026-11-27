// Command apicoveragelint reports Portainer API paths that the provider does
// not call and that are not listed as deliberately uncovered.
//
// It reads the paths of one or more Portainer OpenAPI specs (the openapi.yaml
// published at https://api-docs.portainer.io, which lists every path), and
// the URL patterns the provider builds in internal/*.go: string literals and
// "+" concatenations that contain a path, with format verbs and non-literal
// operands standing for one path segment. A spec path is covered when a
// provider pattern matches it segment by segment.
//
// Paths the provider leaves out on purpose (runtime actions, websockets, UI
// helpers, the add-ons' own machine APIs) are listed with a reason in
// ignore.txt next to this file, one "<path>  # <reason>" per line. A new
// Portainer release that adds paths makes this tool fail until each new path
// is either covered or listed, so nothing new goes unnoticed.
//
// The field check then goes one level deeper. For every operation on a
// covered path that has a JSON or form request body, it lists the top-level
// properties of the request schema (following $ref, allOf, oneOf and anyOf)
// and reports those the provider never sends. A name counts as sent when the
// provider's non-test internal/*.go use it as a json struct tag, a string key
// of a composite literal, a string index assigned to, or a multipart field
// name, compared case-insensitively because Portainer decodes JSON that way.
// The published openapi.yaml only $refs one file per area (tags.yaml, ...);
// those must sit next to it, the tool reads local files only. Fields left out
// on purpose are listed in ignore-fields.txt, one
// "<METHOD> <path> <field>  # <reason>" per line, where any of the three may
// be "*". Pass -fields=false to run the path check alone.
//
// Exit codes:
//
//	0 — every spec path and request field is covered or ignored
//	1 — one or more spec paths or request fields are neither
//	2 — invalid invocation or I/O error
//
// Usage:
//
//	apicoveragelint -spec openapi.yaml [-spec other.yaml] [-fields=false] [root]
//	apicoveragelint -reverse -spec openapi.yaml [-spec other.yaml] [root]
package main

import (
	"bufio"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type specFlags []string

func (s *specFlags) String() string     { return strings.Join(*s, ",") }
func (s *specFlags) Set(v string) error { *s = append(*s, v); return nil }

var (
	specPathLine = regexp.MustCompile(`^  (/[^\s:]*):\s*$`)
	formatVerb   = regexp.MustCompile(`%[-+# 0-9.]*[a-zA-Z]`)
	specParam    = regexp.MustCompile(`\{[^}]+\}`)
)

// param stands for one path segment that is not a literal.
const param = "{}"

func main() {
	var specs specFlags
	flag.Var(&specs, "spec", "OpenAPI spec file listing the paths (repeatable)")
	checkFieldsFlag := flag.Bool("fields", true, "also check the request fields of covered operations")
	reverseFlag := flag.Bool("reverse", false, "only list the provider paths that no given spec has (see reverse.go)")
	flag.Parse()
	root := "."
	if args := flag.Args(); len(args) > 0 {
		root = args[0]
	}
	if len(specs) == 0 {
		fmt.Fprintln(os.Stderr, "apicoveragelint: at least one -spec is required")
		os.Exit(2)
	}

	paths := map[string]bool{}
	for _, file := range specs {
		if err := readSpecPaths(file, paths); err != nil {
			fmt.Fprintf(os.Stderr, "apicoveragelint: %v\n", err)
			os.Exit(2)
		}
	}
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "apicoveragelint: no paths found in the spec (not an openapi.yaml?)")
		os.Exit(2)
	}

	if *reverseFlag {
		n, err := reverseCheck(root, paths)
		if err != nil {
			fmt.Fprintf(os.Stderr, "apicoveragelint: %v\n", err)
			os.Exit(2)
		}
		if n > 0 {
			os.Exit(1)
		}
		os.Exit(0)
	}

	patterns, err := providerPatterns(filepath.Join(root, "internal"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "apicoveragelint: %v\n", err)
		os.Exit(2)
	}
	ignored, err := readIgnore(filepath.Join(root, "internal", "tools", "apicoveragelint", "ignore.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "apicoveragelint: %v\n", err)
		os.Exit(2)
	}

	var ops []operation
	if *checkFieldsFlag {
		for _, file := range specs {
			o, err := readSpecOperations(file)
			if err != nil {
				fmt.Fprintf(os.Stderr, "apicoveragelint: %v\n", err)
				os.Exit(2)
			}
			ops = append(ops, o...)
		}
	}

	var missing, stale []string
	covered := 0
	for p := range paths {
		isCovered := matchesAny(p, patterns)
		switch {
		case isCovered:
			covered++
			if ignored[p] {
				stale = append(stale, p)
			}
		case !ignored[p]:
			missing = append(missing, p)
		}
	}
	for p := range ignored {
		if !paths[p] {
			stale = append(stale, p)
		}
	}
	sort.Strings(missing)
	sort.Strings(stale)

	for _, p := range stale {
		fmt.Printf("note: %s is in ignore.txt but is covered or no longer in the spec; remove it\n", p)
	}
	if len(missing) > 0 {
		for _, p := range missing {
			fmt.Printf("%s: not called by the provider and not in ignore.txt\n", p)
		}
		fmt.Printf("apicoveragelint: %d of %d API paths are neither covered nor ignored.\n", len(missing), len(paths))
	} else {
		fmt.Printf("apicoveragelint: OK — %d API paths: %d called by the provider, %d ignored with a reason.\n", len(paths), covered, len(paths)-covered)
	}
	status := 0
	if len(missing) > 0 {
		status = 1
	}
	if *checkFieldsFlag && !runFieldCheck(root, ops, patterns) {
		status = 1
	}
	os.Exit(status)
}

// runFieldCheck prints the field check and reports whether it passed. I/O
// errors exit with 2.
func runFieldCheck(root string, ops []operation, patterns [][]string) bool {
	sent, err := providerFieldNames(filepath.Join(root, "internal"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "apicoveragelint: %v\n", err)
		os.Exit(2)
	}
	ignores, err := readFieldIgnore(filepath.Join(root, "internal", "tools", "apicoveragelint", "ignore-fields.txt"))
	if err != nil {
		fmt.Fprintf(os.Stderr, "apicoveragelint: %v\n", err)
		os.Exit(2)
	}
	r := checkFields(ops, patterns, sent, ignores)
	for _, e := range r.Stale {
		fmt.Printf("note: %s is in ignore-fields.txt but is sent or no longer in the spec; remove it\n", e)
	}
	if len(r.Missing) > 0 {
		for _, k := range r.Missing {
			fmt.Printf("%s: request field not sent by the provider and not in ignore-fields.txt\n", k)
		}
		fmt.Printf("apicoveragelint: %d of %d request fields are neither sent nor ignored.\n", len(r.Missing), r.Fields)
		return false
	}
	fmt.Printf("apicoveragelint: OK — %d request fields of %d covered operations: %d sent by the provider, %d ignored with a reason.\n", r.Fields, r.Operations, r.Sent, r.Ignored)
	return true
}

// readSpecPaths adds the paths of an openapi.yaml: the keys under "paths:",
// indented by two spaces.
func readSpecPaths(file string, into map[string]bool) error {
	f, err := os.Open(file)
	if err != nil {
		return err
	}
	defer f.Close()
	inPaths := false
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 1024*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "paths:" {
			inPaths = true
			continue
		}
		if inPaths && line != "" && !strings.HasPrefix(line, " ") {
			inPaths = false
		}
		if m := specPathLine.FindStringSubmatch(line); inPaths && m != nil {
			into[m[1]] = true
		}
	}
	return sc.Err()
}

// readIgnore reads ignore.txt: one path per line, a reason after "#" is
// required so every entry says why it is left out.
func readIgnore(file string) (map[string]bool, error) {
	data, err := os.ReadFile(file)
	if err != nil {
		return nil, err
	}
	ignored := map[string]bool{}
	for i, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		path, reason, _ := strings.Cut(line, "#")
		if strings.TrimSpace(reason) == "" {
			return nil, fmt.Errorf("%s:%d: %q has no reason after #", file, i+1, strings.TrimSpace(path))
		}
		ignored[strings.TrimSpace(path)] = true
	}
	return ignored, nil
}

// providerPatterns collects the URL paths built in the provider's non-test Go
// files, as segment lists with param for anything not literal.
func providerPatterns(dir string) ([][]string, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}
	fset := token.NewFileSet()
	var parsed []*ast.File
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, file, nil, 0)
		if err != nil {
			return nil, err
		}
		parsed = append(parsed, f)
	}

	// URL helpers such as addonURL(client, id) return a path that callers
	// extend ("addonURL(...) + \"/access\""); their result stands in for the
	// call.
	helpers := map[string]string{}
	for _, f := range parsed {
		for _, decl := range f.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || len(fn.Body.List) != 1 {
				continue
			}
			if ret, ok := fn.Body.List[0].(*ast.ReturnStmt); ok && len(ret.Results) == 1 {
				if s := render(ret.Results[0], nil); pathSegments(s) != nil {
					helpers[fn.Name.Name] = s
				}
			}
		}
	}

	seen := map[string]bool{}
	var patterns [][]string
	for _, f := range parsed {
		ast.Inspect(f, func(n ast.Node) bool {
			var s string
			switch e := n.(type) {
			case *ast.BinaryExpr:
				if e.Op != token.ADD {
					return true
				}
				s = render(e, helpers)
			case *ast.BasicLit:
				s = render(e, helpers)
			default:
				return true
			}
			if segs := pathSegments(s); segs != nil && !seen[strings.Join(segs, "/")] {
				seen[strings.Join(segs, "/")] = true
				patterns = append(patterns, segs)
			}
			return true
		})
	}
	return patterns, nil
}

// render renders an expression that builds a URL: string literals as they
// are, "+" chains joined, fmt.Sprintf by its format string, calls of a URL
// helper by what the helper returns, and anything else as param.
func render(e ast.Expr, helpers map[string]string) string {
	switch x := e.(type) {
	case *ast.BinaryExpr:
		if x.Op == token.ADD {
			return render(x.X, helpers) + render(x.Y, helpers)
		}
	case *ast.BasicLit:
		if x.Kind == token.STRING {
			if s, err := strconv.Unquote(x.Value); err == nil {
				return s
			}
		}
	case *ast.ParenExpr:
		return render(x.X, helpers)
	case *ast.CallExpr:
		switch fun := x.Fun.(type) {
		case *ast.SelectorExpr:
			if pkg, ok := fun.X.(*ast.Ident); ok && pkg.Name == "fmt" && fun.Sel.Name == "Sprintf" && len(x.Args) > 0 {
				return render(x.Args[0], helpers)
			}
		case *ast.Ident:
			if s, ok := helpers[fun.Name]; ok {
				return s
			}
		}
	}
	return param
}

// pathSegments turns "%s/endpoints/%d/docker?x=1" into
// ["endpoints", "{}", "docker"]. The leading base URL (a verb or operand
// before the first "/") is dropped. Strings that do not look like an API path
// return nil.
func pathSegments(s string) []string {
	s, _, _ = strings.Cut(s, "?")
	s = formatVerb.ReplaceAllString(s, param)
	i := strings.Index(s, "/")
	if i < 0 || (i > 0 && s[:i] != param) || strings.ContainsAny(s, " \n\t\"") {
		return nil
	}
	var segs []string
	literal := false
	for _, seg := range strings.Split(strings.Trim(s[i:], "/"), "/") {
		if seg == "" {
			continue
		}
		if strings.Contains(seg, param) {
			seg = param
		} else {
			literal = true
		}
		segs = append(segs, seg)
	}
	if !literal {
		return nil
	}
	return segs
}

func matchesAny(specPath string, patterns [][]string) bool {
	want := strings.Split(strings.Trim(specParam.ReplaceAllString(specPath, param), "/"), "/")
	for _, p := range patterns {
		if segmentsMatch(p, want) {
			return true
		}
	}
	return false
}

// segmentsMatch compares segment by segment; a param in the provider's
// pattern matches any one segment. A spec parameter only matches a param, so
// a literal such as "/stacks/create" does not cover "/stacks/{id}".
func segmentsMatch(pattern, path []string) bool {
	if len(pattern) != len(path) {
		return false
	}
	for i := range pattern {
		if pattern[i] != path[i] && pattern[i] != param {
			return false
		}
	}
	return true
}
