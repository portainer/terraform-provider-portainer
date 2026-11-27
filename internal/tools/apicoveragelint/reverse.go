package main

// The reverse check (-reverse, opt-in) looks the other way round: it lists
// the URL paths the provider builds that no given spec has, such as an
// endpoint Portainer removed (the per-user git credentials went in Business
// Edition 2.43). Patterns are collected as for the path check (render and
// pathSegments). A provider pattern is known when it matches a spec path
// segment by segment, where a provider param matches any one segment (a
// spec param only a provider param, else "/x/{a}/{b}" would match any
// fragment); a fragment
// appended to a base built elsewhere is known when it matches the end of a
// spec path, and a leading /api is dropped.
//
// Paths that legitimately are in no spec are listed in reverse-ignore.txt,
// one "<path prefix>  # <reason>" per line: a pattern starting with the
// listed segments ("{}" for any one segment) is ignored. They are the
// Docker and Kubernetes API proxies (/endpoints/{id}/docker/...), file
// paths, documentation links and the like.

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
)

// patternSites maps a provider pattern ("users/{}/gitcredentials") to the
// file:line positions that build it.
func patternSites(dir string) (map[string][]string, error) {
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
	// URL helpers, as in providerPatterns.
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
	sites := map[string][]string{}
	// The operands of a "+" chain are parts of the whole path
	// (Endpoint + "/upload/tls/" + kind), not paths of their own.
	inner := map[ast.Expr]bool{}
	for _, f := range parsed {
		ast.Inspect(f, func(n ast.Node) bool {
			var s string
			switch e := n.(type) {
			case *ast.BinaryExpr:
				if e.Op != token.ADD {
					return true
				}
				inner[e.X], inner[e.Y] = true, true
				if inner[e] {
					return true
				}
				s = render(e, helpers)
			case *ast.BasicLit:
				if inner[e] {
					return true
				}
				s = render(e, helpers)
			default:
				return true
			}
			if segs := pathSegments(s); segs != nil {
				key := strings.Join(segs, "/")
				pos := fset.Position(n.Pos())
				site := fmt.Sprintf("%s:%d", filepath.Base(pos.Filename), pos.Line)
				if l := sites[key]; len(l) == 0 || l[len(l)-1] != site {
					sites[key] = append(sites[key], site)
				}
			}
			return true
		})
	}
	return sites, nil
}

// looseMatch reports whether pattern matches the end of path, segment by
// segment (a provider param matches any segment). Matching the end lets the fragments the
// provider appends to a base built elsewhere (base + "/access") count as
// known.
func looseMatch(pattern, path []string) bool {
	if len(pattern) > len(path) {
		return false
	}
	path = path[len(path)-len(pattern):]
	for i := range pattern {
		if pattern[i] != path[i] && pattern[i] != param {
			return false
		}
	}
	return true
}

// hasPrefixSegments reports whether segs starts with prefix ("{}" in the
// prefix matches any one segment).
func hasPrefixSegments(segs, prefix []string) bool {
	if len(prefix) > len(segs) {
		return false
	}
	for i := range prefix {
		if prefix[i] != segs[i] && prefix[i] != param {
			return false
		}
	}
	return true
}

// reverseCheck prints the provider paths that no spec path matches and are
// not in reverse-ignore.txt, and returns how many there are.
func reverseCheck(root string, specPaths map[string]bool) (int, error) {
	sites, err := patternSites(filepath.Join(root, "internal"))
	if err != nil {
		return 0, err
	}
	ignoreSet, err := readIgnore(filepath.Join(root, "internal", "tools", "apicoveragelint", "reverse-ignore.txt"))
	if err != nil {
		return 0, err
	}
	var prefixes [][]string
	for p := range ignoreSet {
		prefixes = append(prefixes, strings.Split(strings.Trim(specParam.ReplaceAllString(p, param), "/"), "/"))
	}
	var spec [][]string
	for p := range specPaths {
		spec = append(spec, strings.Split(strings.Trim(specParam.ReplaceAllString(p, param), "/"), "/"))
	}
	var unknown []string
	for key, where := range sites {
		segs := strings.Split(key, "/")
		// The specs' paths are relative to /api (webhook URLs are built
		// with it).
		if len(segs) > 1 && segs[0] == "api" {
			segs = segs[1:]
		}
		known := len(segs) == 1 && segs[0] == "api"
		for _, s := range spec {
			if looseMatch(segs, s) {
				known = true
				break
			}
		}
		for _, p := range prefixes {
			if known {
				break
			}
			known = hasPrefixSegments(segs, p)
		}
		if !known {
			unknown = append(unknown, fmt.Sprintf("/%s: built by the provider (%s) but in no given spec", key, strings.Join(where, ", ")))
		}
	}
	sort.Strings(unknown)
	for _, u := range unknown {
		fmt.Println(u)
	}
	if len(unknown) == 0 {
		fmt.Printf("apicoveragelint: OK — every provider path (%d) is in a given spec or in reverse-ignore.txt.\n", len(sites))
	} else {
		fmt.Printf("apicoveragelint: %d of %d provider paths are in no given spec and not in reverse-ignore.txt.\n", len(unknown), len(sites))
	}
	return len(unknown), nil
}
