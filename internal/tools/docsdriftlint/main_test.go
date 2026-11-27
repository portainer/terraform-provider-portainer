package main

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/portainer/terraform-provider-portainer/internal"
)

func TestCheck(t *testing.T) {
	root := t.TempDir()
	write := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	const schema = "{{ .SchemaMarkdown | trimspace }}\n"
	write("templates/index.md.tmpl", "# Portainer\n\n"+schema+"\n| Name | Status |\n|---|---|\n| `portainer_good` | done |\n| `portainer_removed` | done |\n")
	// A kept table of object members, nested paths and notes that all exist.
	write("templates/resources/good.md.tmpl", "# good\n\n"+schema+
		"\n## Notes on Arguments and Attributes\n\n- `name`: Changing it forces a new resource.\n- `git.url`: Defaults to `x`.\n\n"+
		"### `items`\n\n| Name | Description |\n|---|---|\n| `member` | x |\n| `id` | x |\n\n"+
		"```hcl\n| `not_a_field` | inside a code block |\n```\n")
	// A renamed field lingers in a table and in a note.
	write("templates/resources/drift.md.tmpl", "# drift\n\n"+schema+
		"\n| Name | Description |\n|---|---|\n| `old_name` | x |\n\n## Notes on Arguments and Attributes\n\n- `gone`: x\n")
	// No generated schema section.
	write("templates/data-sources/good.md.tmpl", "Uses `namespace` only.\n")
	// No registered type.
	write("templates/actions/orphan.md.tmpl", "# orphan\n\n"+schema)

	block := &tfprotov5.SchemaBlock{
		Attributes: []*tfprotov5.SchemaAttribute{
			{Name: "id"},
			{Name: "name"},
			{Name: "items", Type: tftypes.List{ElementType: tftypes.Object{AttributeTypes: map[string]tftypes.Type{"member": tftypes.String}}}},
		},
		BlockTypes: []*tfprotov5.SchemaNestedBlock{{TypeName: "git", Block: &tfprotov5.SchemaBlock{
			Attributes: []*tfprotov5.SchemaAttribute{{Name: "url"}},
		}}},
	}
	schemas := []internal.LintSchema{
		{Kind: "provider", TypeName: "portainer", Block: block},
		{Kind: "resource", TypeName: "portainer_good", Block: block},
		{Kind: "resource", TypeName: "portainer_drift", Block: block},
		{Kind: "resource", TypeName: "portainer_nodoc", Block: block},
		{Kind: "data source", TypeName: "portainer_good", Block: block},
	}
	violations, checked, err := check(root, schemas)
	if err != nil {
		t.Fatal(err)
	}
	got := make([]string, 0, len(violations))
	for _, v := range violations {
		got = append(got, v.tfName+": "+v.msg)
	}
	sort.Strings(got)
	want := []string{
		`portainer: documented type "portainer_removed" is not registered`,
		`portainer_drift: documented field "gone" is not in the schema`,
		`portainer_drift: documented field "old_name" is not in the schema`,
		"portainer_good: template does not embed {{ .SchemaMarkdown }}",
		"portainer_nodoc: template not found",
		"portainer_orphan: template has no registered action",
	}
	if len(got) != len(want) {
		t.Fatalf("violations =\n%q\nwant\n%q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("violations =\n%q\nwant\n%q", got, want)
		}
	}
	if checked != 4 {
		t.Fatalf("checked = %d, want 4", checked)
	}
}
