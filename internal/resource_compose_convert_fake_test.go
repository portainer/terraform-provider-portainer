package internal

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The tests in resource_compose_convert_test.go need the real kompose binary
// and are skipped without it. These put a fake kompose script first on PATH,
// so the Create logic around the binary runs everywhere except Windows.

func withFakeKompose(t *testing.T, script string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("fake kompose is a shell script")
	}
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "kompose"), []byte("#!/bin/sh\n"+script), 0o755); err != nil {
		t.Fatalf("failed to write fake kompose: %v", err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
}

func TestComposeConvertCreate_FakeKompose_CollectsYAMLManifests(t *testing.T) {
	// Copies the compose file into a manifest so the test can check it was
	// written, and adds files that must not be collected.
	withFakeKompose(t, `[ "$1" = "convert" ] || exit 2
cp docker-compose.yml web-deployment.yaml
echo "kind: Service" > web-service.yaml
echo "not a manifest" > notes.txt
mkdir nested.yaml
`)

	r := resourceComposeConvertResource()
	d := r.TestResourceData()
	compose := "services:\n  web:\n    image: nginx:1.25\n"
	_ = d.Set("compose_content", compose)

	if diags := r.CreateContext(context.Background(), d, nil); diags.HasError() {
		t.Fatalf("CreateContext returned diagnostics: %+v", diags)
	}

	if !strings.HasPrefix(d.Id(), "convert-") {
		t.Errorf("ID: expected prefix %q, got %q", "convert-", d.Id())
	}
	manifests := d.Get("manifests").(map[string]interface{})
	if len(manifests) != 2 {
		t.Fatalf("expected the 2 .yaml files, got %v", manifests)
	}
	if manifests["web-deployment.yaml"] != compose {
		t.Errorf("web-deployment.yaml: expected the compose content, got %q", manifests["web-deployment.yaml"])
	}
	if manifests["web-service.yaml"] != "kind: Service\n" {
		t.Errorf("web-service.yaml: unexpected content %q", manifests["web-service.yaml"])
	}
}

func TestComposeConvertCreate_FakeKompose_FailureIncludesOutput(t *testing.T) {
	withFakeKompose(t, `echo "unsupported compose key" >&2
exit 1
`)

	r := resourceComposeConvertResource()
	d := r.TestResourceData()
	_ = d.Set("compose_content", "services: {}\n")

	diags := r.CreateContext(context.Background(), d, nil)
	if !diags.HasError() {
		t.Fatal("expected error diagnostics when kompose fails")
	}
	if !strings.Contains(diags[0].Summary, "unsupported compose key") {
		t.Errorf("expected the kompose output in the error, got %q", diags[0].Summary)
	}
	if d.Id() != "" {
		t.Errorf("expected no ID after a failed convert, got %q", d.Id())
	}
}

func TestComposeConvertCreate_KomposeMissing(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	r := resourceComposeConvertResource()
	d := r.TestResourceData()
	_ = d.Set("compose_content", "services: {}\n")

	if diags := r.CreateContext(context.Background(), d, nil); !diags.HasError() {
		t.Fatal("expected error diagnostics when kompose is not installed")
	}
}
