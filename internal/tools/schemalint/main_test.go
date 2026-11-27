package main

import (
	"reflect"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
	"github.com/portainer/terraform-provider-portainer/internal"
)

func TestCheck(t *testing.T) {
	schemas := []internal.LintSchema{{
		Kind:     "resource",
		TypeName: "portainer_x",
		Implicit: map[string]bool{"id": true, "timeouts": true},
		Block: &tfprotov5.SchemaBlock{
			Attributes: []*tfprotov5.SchemaAttribute{
				{Name: "id"},
				{Name: "name", Description: "Name."},
				{Name: "blank", Description: "  "},
				{Name: "items", Description: "Items.", Type: tftypes.List{ElementType: tftypes.Object{
					AttributeTypes: map[string]tftypes.Type{"type_only": tftypes.String},
				}}},
			},
			BlockTypes: []*tfprotov5.SchemaNestedBlock{
				{TypeName: "git", Block: &tfprotov5.SchemaBlock{
					Description: "Git settings.",
					Attributes:  []*tfprotov5.SchemaAttribute{{Name: "url"}, {Name: "ref", Description: "Ref."}},
				}},
				{TypeName: "auth", Block: &tfprotov5.SchemaBlock{
					Attributes: []*tfprotov5.SchemaAttribute{{Name: "user", Description: "User."}},
				}},
				{TypeName: "timeouts", Block: &tfprotov5.SchemaBlock{
					Attributes: []*tfprotov5.SchemaAttribute{{Name: "create"}},
				}},
			},
		},
	}}
	got := check(schemas)
	want := []string{
		`resource portainer_x: schema field "auth" missing Description`,
		`resource portainer_x: schema field "blank" missing Description`,
		`resource portainer_x: schema field "git.url" missing Description`,
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("check() =\n%q\nwant\n%q", got, want)
	}

	// A declared (framework) "id" is not exempt.
	schemas[0].Implicit = nil
	if got := check(schemas); len(got) != 6 {
		t.Fatalf("without implicit names: %d violations, want 6: %q", len(got), got)
	}
}

// TestComputedObjectList checks the fields of a computed list of objects (an
// object-typed attribute in the protocol, which carries no description of
// its fields) are not reported: only the attribute itself can be described.
func TestComputedObjectList(t *testing.T) {
	schemas := []internal.LintSchema{{
		Kind:     "data source",
		TypeName: "portainer_x",
		Block: &tfprotov5.SchemaBlock{Attributes: []*tfprotov5.SchemaAttribute{{
			Name: "items", Description: "Items.",
			Type: tftypes.List{ElementType: tftypes.Object{AttributeTypes: map[string]tftypes.Type{"a": tftypes.String, "b": tftypes.String}}},
		}}},
	}}
	if got := check(schemas); len(got) != 0 {
		t.Fatalf("check() = %q, want no violations", got)
	}
}
