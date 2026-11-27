package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
)

// The schema of v3.0.0, the last release served by SDKv2 alone, as
// `terraform providers schema -json` printed it. Moving a resource to the
// plugin framework must not change what a configuration can say or what the
// state holds, so every resource and data source is compared against it.

type goldenSchemas struct {
	ResourceSchemas   map[string]goldenSchema `json:"resource_schemas"`
	DataSourceSchemas map[string]goldenSchema `json:"data_source_schemas"`
}

type goldenSchema struct {
	Block goldenBlock `json:"block"`
}

type goldenBlock struct {
	Attributes map[string]goldenAttribute `json:"attributes"`
	BlockTypes map[string]goldenBlockType `json:"block_types"`
}

type goldenAttribute struct {
	Type      json.RawMessage `json:"type"`
	Required  bool            `json:"required"`
	Optional  bool            `json:"optional"`
	Computed  bool            `json:"computed"`
	Sensitive bool            `json:"sensitive"`
	WriteOnly bool            `json:"write_only"`
}

type goldenBlockType struct {
	NestingMode string      `json:"nesting_mode"`
	Block       goldenBlock `json:"block"`
	MinItems    int64       `json:"min_items"`
	MaxItems    int64       `json:"max_items"`
}

// TestSchemaCompatibleWithV300 fails on any difference a user could notice:
// a missing resource, attribute or block, another type, a required attribute
// that turned optional or the other way round, an attribute that stopped
// being computed, sensitive or write-only. Accepted on purpose:
//   - "id": SDKv2 declares it optional+computed, the framework computed only;
//   - an optional attribute becoming optional+computed (the framework needs
//     that for a default);
//   - MaxItems/MinItems of a block, which the framework checks with
//     validators instead.
func TestSchemaCompatibleWithV300(t *testing.T) {
	t.Parallel()
	raw, err := os.ReadFile("testdata/schema_v3.0.0.json")
	if err != nil {
		t.Fatal(err)
	}
	var golden goldenSchemas
	if err := json.Unmarshal(raw, &golden); err != nil {
		t.Fatal(err)
	}

	server, err := NewProviderServer(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	resp, err := server.GetProviderSchema(context.Background(), &tfprotov5.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatal(err)
	}

	var problems []string
	compare := func(kind string, want map[string]goldenSchema, have map[string]*tfprotov5.Schema) {
		for name, w := range want {
			h, ok := have[name]
			if !ok {
				problems = append(problems, fmt.Sprintf("%s %s is gone", kind, name))
				continue
			}
			problems = append(problems, compareBlock(kind+" "+name, w.Block, h.Block)...)
		}
	}
	compare("resource", golden.ResourceSchemas, resp.ResourceSchemas)
	compare("data source", golden.DataSourceSchemas, resp.DataSourceSchemas)

	sort.Strings(problems)
	for _, p := range problems {
		t.Error(p)
	}
}

func compareBlock(where string, want goldenBlock, have *tfprotov5.SchemaBlock) []string {
	var problems []string
	haveAttrs := map[string]*tfprotov5.SchemaAttribute{}
	for _, a := range have.Attributes {
		haveAttrs[a.Name] = a
	}
	haveBlocks := map[string]*tfprotov5.SchemaNestedBlock{}
	for _, b := range have.BlockTypes {
		haveBlocks[b.TypeName] = b
	}

	for name, w := range want.Attributes {
		at := where + "." + name
		h, ok := haveAttrs[name]
		if !ok {
			problems = append(problems, at+": attribute is gone")
			continue
		}
		if !sameType(w.Type, h) {
			problems = append(problems, fmt.Sprintf("%s: type %s became %s", at, w.Type, typeJSON(h)))
		}
		if name == "id" {
			continue
		}
		if w.Required != h.Required {
			problems = append(problems, fmt.Sprintf("%s: required %v became %v", at, w.Required, h.Required))
		}
		if w.Optional != h.Optional {
			problems = append(problems, fmt.Sprintf("%s: optional %v became %v", at, w.Optional, h.Optional))
		}
		if w.Computed && !h.Computed {
			problems = append(problems, at+": no longer computed")
		}
		if w.Computed != h.Computed && !w.Optional {
			problems = append(problems, fmt.Sprintf("%s: computed %v became %v", at, w.Computed, h.Computed))
		}
		if w.Sensitive != h.Sensitive {
			problems = append(problems, fmt.Sprintf("%s: sensitive %v became %v", at, w.Sensitive, h.Sensitive))
		}
		if w.WriteOnly != h.WriteOnly {
			problems = append(problems, fmt.Sprintf("%s: write-only %v became %v", at, w.WriteOnly, h.WriteOnly))
		}
	}
	for name, w := range want.BlockTypes {
		at := where + "." + name
		h, ok := haveBlocks[name]
		if !ok {
			if _, isAttr := haveAttrs[name]; isAttr {
				problems = append(problems, at+": block became an attribute (changes the configuration syntax)")
			} else {
				problems = append(problems, at+": block is gone")
			}
			continue
		}
		if !strings.EqualFold(w.NestingMode, nestingName(h.Nesting)) {
			problems = append(problems, fmt.Sprintf("%s: nesting %s became %s", at, w.NestingMode, nestingName(h.Nesting)))
		}
		problems = append(problems, compareBlock(at, w.Block, h.Block)...)
	}
	return problems
}

func sameType(want json.RawMessage, h *tfprotov5.SchemaAttribute) bool {
	var a, b interface{}
	if json.Unmarshal(want, &a) != nil || json.Unmarshal([]byte(typeJSON(h)), &b) != nil {
		return false
	}
	return fmt.Sprint(a) == fmt.Sprint(b)
}

func typeJSON(h *tfprotov5.SchemaAttribute) string {
	if h.Type == nil {
		return "null"
	}
	b, err := h.Type.MarshalJSON() //nolint:staticcheck // the cty JSON form is what the golden file holds
	if err != nil {
		return err.Error()
	}
	return string(b)
}

func nestingName(n tfprotov5.SchemaNestedBlockNestingMode) string {
	switch n {
	case tfprotov5.SchemaNestedBlockNestingModeSingle:
		return "single"
	case tfprotov5.SchemaNestedBlockNestingModeList:
		return "list"
	case tfprotov5.SchemaNestedBlockNestingModeSet:
		return "set"
	case tfprotov5.SchemaNestedBlockNestingModeMap:
		return "map"
	case tfprotov5.SchemaNestedBlockNestingModeGroup:
		return "group"
	}
	return "invalid"
}
