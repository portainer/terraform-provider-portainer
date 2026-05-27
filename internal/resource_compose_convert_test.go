package internal

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// TestComposeConvertCreate_HappyPath runs the resource against a minimal
// docker-compose payload. The implementation shells out to `kompose convert`
// and reads the generated YAMLs back from a tempdir.
//
// The test is skipped automatically if the `kompose` binary is not on PATH.
func TestComposeConvertCreate_HappyPath(t *testing.T) {
	if _, err := exec.LookPath("kompose"); err != nil {
		t.Skip("kompose binary not available; skipping")
	}

	r := resourceComposeConvertResource()
	d := r.TestResourceData()
	_ = d.Set("compose_content", `services:
  web:
    image: nginx:1.25
    ports:
      - "80:80"
`)

	diags := r.CreateContext(context.Background(), d, nil)
	if diags.HasError() {
		t.Fatalf("CreateContext returned diagnostics: %+v", diags)
	}

	if d.Id() == "" {
		t.Error("expected non-empty ID after Create")
	}
	if !strings.HasPrefix(d.Id(), "convert-") {
		t.Errorf("ID: expected prefix \"convert-\", got %q", d.Id())
	}

	manifests, ok := d.Get("manifests").(map[string]interface{})
	if !ok {
		t.Fatalf("manifests: expected map, got %T", d.Get("manifests"))
	}
	if len(manifests) == 0 {
		t.Error("expected at least one generated manifest, got 0")
	}
}

// TestComposeConvertCreate_InvalidCompose verifies the resource surfaces
// `kompose convert` failures (e.g. when the compose YAML is gibberish).
func TestComposeConvertCreate_InvalidCompose(t *testing.T) {
	if _, err := exec.LookPath("kompose"); err != nil {
		t.Skip("kompose binary not available; skipping")
	}

	r := resourceComposeConvertResource()
	d := r.TestResourceData()
	_ = d.Set("compose_content", "::: this is not valid yaml :::")

	diags := r.CreateContext(context.Background(), d, nil)
	if !diags.HasError() {
		t.Fatal("expected error diagnostics for invalid compose content")
	}
}

// TestComposeConvertRead_NoOp verifies Read is a no-op (NoopContext).
func TestComposeConvertRead_NoOp(t *testing.T) {
	r := resourceComposeConvertResource()
	d := r.TestResourceData()
	d.SetId("convert-test")

	// schema.NoopContext just returns nil diagnostics.
	diags := r.ReadContext(context.Background(), d, nil)
	if diags.HasError() {
		t.Errorf("Read should be a no-op, got diagnostics: %+v", diags)
	}
}

// TestComposeConvertDelete_NoOp verifies Delete is a no-op (NoopContext).
// The framework clears the ID after a successful destroy regardless.
func TestComposeConvertDelete_NoOp(t *testing.T) {
	r := resourceComposeConvertResource()
	d := r.TestResourceData()
	d.SetId("convert-test")

	diags := r.DeleteContext(context.Background(), d, nil)
	if diags.HasError() {
		t.Errorf("Delete should be a no-op, got diagnostics: %+v", diags)
	}
}

// TestComposeConvertSchema verifies the resource schema declares compose_content
// as required and manifests as computed (sanity check on the schema definition).
func TestComposeConvertSchema(t *testing.T) {
	r := resourceComposeConvertResource()
	if r.Schema["compose_content"] == nil {
		t.Fatal("schema is missing compose_content")
	}
	if !r.Schema["compose_content"].Required {
		t.Error("compose_content should be Required")
	}
	if r.Schema["manifests"] == nil {
		t.Fatal("schema is missing manifests")
	}
	if !r.Schema["manifests"].Computed {
		t.Error("manifests should be Computed")
	}
	if r.Schema["manifests"].Type != schema.TypeMap {
		t.Errorf("manifests should be TypeMap, got %v", r.Schema["manifests"].Type)
	}
}
