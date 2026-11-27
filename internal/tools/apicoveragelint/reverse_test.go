package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLooseMatch(t *testing.T) {
	seg := func(s string) []string { return strings.Split(s, "/") }
	for _, tc := range []struct {
		pattern, path string
		want          bool
	}{
		{"users/{}/gitcredentials", "users/{}/gitcredentials", true},
		{"users/{}/gitcredentials", "cloud/gitcredentials", false},
		{"access", "addons/{}/access", true},             // fragment matching the end
		{"users/{}/gitcredentials", "x/{}/{}/{}", false}, // a spec param does not match a literal
		{"stacks/{}", "stacks/create", true},             // a provider param matches a literal
	} {
		if got := looseMatch(seg(tc.pattern), seg(tc.path)); got != tc.want {
			t.Errorf("looseMatch(%q, %q) = %v, want %v", tc.pattern, tc.path, got, tc.want)
		}
	}
}

func TestReverseCheck(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "internal")
	if err := os.MkdirAll(filepath.Join(dir, "tools", "apicoveragelint"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := `package internal

import "fmt"

func calls(base string, id int) {
	_ = fmt.Sprintf("%s/tags/%d", base, id)
	_ = base + "/endpoints/" + fmt.Sprint(id) + "/docker/containers/json"
	_ = fmt.Sprintf("%s/users/%d/gitcredentials", base, id)
}
`
	if err := os.WriteFile(filepath.Join(dir, "calls.go"), []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	ignore := "/endpoints/{id}/docker  # proxy\n"
	if err := os.WriteFile(filepath.Join(dir, "tools", "apicoveragelint", "reverse-ignore.txt"), []byte(ignore), 0o600); err != nil {
		t.Fatal(err)
	}
	n, err := reverseCheck(root, map[string]bool{"/tags/{id}": true})
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("reverseCheck reported %d unknown paths, want 1 (/users/{}/gitcredentials)", n)
	}
}
