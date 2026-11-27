package main

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/portainer/terraform-provider-portainer/internal"
)

func TestCheck(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "good.md"), []byte("# good\n\n## Import\n\n```\nterraform import ...\n```\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "bad.md"), []byte("# bad\n\nNo heading here about importing.\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	got := check(dir, map[string]bool{"portainer_good": true, "portainer_bad": true, "portainer_gone": true})
	want := []string{
		"portainer_bad (" + filepath.Join(dir, "bad.md") + "): no Import section",
		"portainer_gone (" + filepath.Join(dir, "gone.md") + "): docs file missing",
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("check() =\n%q\nwant\n%q", got, want)
	}
}

// TestImportableResourceTypes checks resources implementing
// resource.ResourceWithImportState are seen, and others are not.
func TestImportableResourceTypes(t *testing.T) {
	importable := internal.ImportableResourceTypes(context.Background())
	if !importable["portainer_tag"] {
		t.Error("portainer_tag (ResourceWithImportState) is not importable")
	}
	if importable["portainer_container_exec"] {
		t.Error("portainer_container_exec has no import but is reported importable")
	}
}
