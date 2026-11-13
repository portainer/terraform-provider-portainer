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
// Exit codes:
//
//	0 — every spec path is covered or ignored
//	1 — one or more spec paths are neither
//	2 — invalid invocation or I/O error
//
// Usage:
//
//	apicoveragelint -spec openapi.yaml [-spec other.yaml] [root]
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
		os.Exit(1)
	}
	fmt.Printf("apicoveragelint: OK — %d API paths: %d called by the provider, %d ignored with a reason.\n", len(paths), covered, len(paths)-covered)
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
