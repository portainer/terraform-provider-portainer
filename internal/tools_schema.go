package internal

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/provider"
	"github.com/hashicorp/terraform-plugin-framework/resource"
	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
	"github.com/hashicorp/terraform-plugin-go/tftypes"
)

// This file serves the lint tools under internal/tools (schemalint,
// sensitivelint, docsdriftlint, importdoclint). They check the provider's
// schema as Terraform receives it from the provider server.

// LintSchema is one schema served by the provider.
type LintSchema struct {
	// Kind is "provider", "resource", "data source", "ephemeral resource",
	// "list resource" or "action".
	Kind string
	// TypeName is the Terraform type name, e.g. portainer_tag ("portainer"
	// for the provider schema).
	TypeName string
	Block    *tfprotov5.SchemaBlock
	// Implicit holds the top-level names a library adds without letting the
	// provider describe them: the "timeouts" block of
	// terraform-plugin-framework-timeouts.
	Implicit map[string]bool
}

// LintField is one attribute or nested block of a LintSchema.
type LintField struct {
	// Path is the dotted path from the schema root, e.g. "git.url".
	Path        string
	Name        string
	Block       bool
	TopLevel    bool
	Description string
	Sensitive   bool
	WriteOnly   bool
	// Implicit is set for a field a library added (see
	// LintSchema.Implicit) and for everything nested in one.
	Implicit bool
	// TypeOnly is set for a field known only from the object type of an
	// attribute (no own Description or Sensitive flag exists for it in the
	// protocol); its Sensitive is the enclosing attribute's.
	TypeOnly bool
}

var lintKindOrder = map[string]int{
	"provider": 0, "resource": 1, "data source": 2,
	"ephemeral resource": 3, "list resource": 4, "action": 5,
}

// LintSchemas returns every schema the provider server reports, sorted by kind and
// type name.
func LintSchemas(ctx context.Context) ([]LintSchema, error) {
	server, err := NewProviderServer(ctx)
	if err != nil {
		return nil, fmt.Errorf("starting the provider server: %w", err)
	}
	resp, err := server.GetProviderSchema(ctx, &tfprotov5.GetProviderSchemaRequest{})
	if err != nil {
		return nil, fmt.Errorf("GetProviderSchema: %w", err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov5.DiagnosticSeverityError {
			return nil, fmt.Errorf("GetProviderSchema: %s: %s", d.Summary, d.Detail)
		}
	}

	var out []LintSchema
	add := func(kind, name string, s *tfprotov5.Schema, implicit map[string]bool) {
		if s == nil || s.Block == nil {
			return
		}
		out = append(out, LintSchema{Kind: kind, TypeName: name, Block: s.Block, Implicit: implicit})
	}
	add("provider", "portainer", resp.Provider, nil)
	for name, s := range resp.ResourceSchemas {
		add("resource", name, s, map[string]bool{"timeouts": true})
	}
	for name, s := range resp.DataSourceSchemas {
		add("data source", name, s, nil)
	}
	for name, s := range resp.EphemeralResourceSchemas {
		add("ephemeral resource", name, s, nil)
	}
	for name, s := range resp.ListResourceSchemas {
		add("list resource", name, s, nil)
	}
	for name, a := range resp.ActionSchemas {
		if a != nil {
			add("action", name, a.Schema, nil)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Kind != out[j].Kind {
			return lintKindOrder[out[i].Kind] < lintKindOrder[out[j].Kind]
		}
		return out[i].TypeName < out[j].TypeName
	})
	return out, nil
}

// Fields returns every attribute and nested block of s, nested ones included,
// sorted by path.
func (s LintSchema) Fields() []LintField {
	var out []LintField
	walkLintBlock(s.Block, "", false, s.Implicit, &out)
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func walkLintBlock(b *tfprotov5.SchemaBlock, prefix string, implicit bool, top map[string]bool, out *[]LintField) {
	if b == nil {
		return
	}
	for _, a := range b.Attributes {
		if a == nil {
			continue
		}
		imp := implicit || (prefix == "" && top[a.Name])
		*out = append(*out, LintField{
			Path: prefix + a.Name, Name: a.Name, TopLevel: prefix == "",
			Description: a.Description, Sensitive: a.Sensitive, WriteOnly: a.WriteOnly,
			Implicit: imp,
		})
		walkLintType(a.Type, prefix+a.Name+".", imp, a.Sensitive, out)
	}
	for _, nb := range b.BlockTypes {
		if nb == nil {
			continue
		}
		imp := implicit || (prefix == "" && top[nb.TypeName])
		desc := ""
		if nb.Block != nil {
			desc = nb.Block.Description
		}
		*out = append(*out, LintField{
			Path: prefix + nb.TypeName, Name: nb.TypeName, Block: true, TopLevel: prefix == "",
			Description: desc, Implicit: imp,
		})
		walkLintBlock(nb.Block, prefix+nb.TypeName+".", imp, nil, out)
	}
}

// walkLintType lists the fields of the object types within t.
func walkLintType(t tftypes.Type, prefix string, implicit, sensitive bool, out *[]LintField) {
	switch v := t.(type) {
	case tftypes.List:
		walkLintType(v.ElementType, prefix, implicit, sensitive, out)
	case tftypes.Set:
		walkLintType(v.ElementType, prefix, implicit, sensitive, out)
	case tftypes.Map:
		walkLintType(v.ElementType, prefix, implicit, sensitive, out)
	case tftypes.Object:
		for name, at := range v.AttributeTypes {
			*out = append(*out, LintField{
				Path: prefix + name, Name: name, Sensitive: sensitive,
				Implicit: implicit, TypeOnly: true,
			})
			walkLintType(at, prefix+name+".", implicit, sensitive, out)
		}
	}
}

// ImportableResourceTypes returns the type names of the resources that
// support terraform import (implement resource.ResourceWithImportState).
func ImportableResourceTypes(ctx context.Context) map[string]bool {
	out := map[string]bool{}
	p := NewFrameworkProvider()
	var meta provider.MetadataResponse
	p.Metadata(ctx, provider.MetadataRequest{}, &meta)
	for _, newResource := range p.Resources(ctx) {
		r := newResource()
		if _, ok := r.(resource.ResourceWithImportState); !ok {
			continue
		}
		var resp resource.MetadataResponse
		r.Metadata(ctx, resource.MetadataRequest{ProviderTypeName: meta.TypeName}, &resp)
		if strings.TrimSpace(resp.TypeName) != "" {
			out[resp.TypeName] = true
		}
	}
	return out
}
