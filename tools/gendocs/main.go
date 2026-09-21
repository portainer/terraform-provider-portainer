package main

// Generates a documentation skeleton for a data source straight from its
// schema, so the tables cannot drift from the fields on the day they are
// written. Descriptions come from the schema itself.

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
	"github.com/portainer/terraform-provider-portainer/internal"
)

// safeName is the shape a Terraform data source name has once its
// "portainer_" prefix is stripped.
var safeName = regexp.MustCompile(`^[a-z0-9_]+$`)

func typeName(s *schema.Schema) string {
	switch s.Type {
	case schema.TypeBool:
		return "bool"
	case schema.TypeInt:
		return "number"
	case schema.TypeFloat:
		return "number"
	case schema.TypeString:
		return "string"
	case schema.TypeList, schema.TypeSet:
		if _, ok := s.Elem.(*schema.Resource); ok {
			return "list(object)"
		}
		return "list(string)"
	case schema.TypeMap:
		return "map(string)"
	}
	return "string"
}

func rows(out *strings.Builder, fields map[string]*schema.Schema, args bool) {
	names := make([]string, 0, len(fields))
	for name, f := range fields {
		if (f.Required || f.Optional) == args {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	for _, name := range names {
		f := fields[name]
		if args {
			required := "❌ no"
			if f.Required {
				required = "✅ yes"
			}
			fmt.Fprintf(out, "| `%s` | %s | %s | %s |\n", name, typeName(f), required, f.Description)
		} else {
			fmt.Fprintf(out, "| `%s` | %s | %s |\n", name, typeName(f), f.Description)
		}
	}
}

func nested(out *strings.Builder, prefix string, fields map[string]*schema.Schema) {
	names := make([]string, 0, len(fields))
	for n := range fields {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, name := range names {
		res, ok := fields[name].Elem.(*schema.Resource)
		if !ok {
			continue
		}
		label := name
		if prefix != "" {
			label = prefix + "." + name
		}
		fmt.Fprintf(out, "\n### `%s`\n\n| Name | Type | Description |\n|------|------|-------------|\n", label)
		rows(out, res.Schema, false)
		nested(out, label, res.Schema)
	}
}

func main() {
	force := flag.Bool("force", false, "regenerate pages that already exist, discarding any prose in them")
	flag.Parse()

	p := internal.Provider()
	for _, name := range flag.Args() {
		ds, ok := p.DataSourcesMap[name]
		if !ok {
			fmt.Fprintf(os.Stderr, "unknown data source %s\n", name)
			os.Exit(1)
		}
		short := strings.TrimPrefix(name, "portainer_")

		var out strings.Builder
		fmt.Fprintf(&out, "# Data Source Documentation: `%s`\n\n# %s\n\n", name, name)
		// No edition banner: whether an endpoint is Business Edition only has to
		// be checked against the CE specification, not assumed.
		out.WriteString("DESCRIPTION\n\n")
		out.WriteString("## Example Usage\n\n```hcl\nEXAMPLE\n```\n\n")
		out.WriteString("## Arguments Reference\n\n| Name | Type | Required | Description |\n|------|------|----------|-------------|\n")
		rows(&out, ds.Schema, true)
		out.WriteString("\n## Attributes Reference\n\n| Name | Type | Description |\n|------|------|-------------|\n")
		rows(&out, ds.Schema, false)
		nested(&out, "", ds.Schema)

		// The name comes from the command line, so it is checked against the
		// shape a data source name actually has and then reduced to a bare
		// file name - an argument must not be able to write outside docs/.
		if !safeName.MatchString(short) {
			fmt.Fprintf(os.Stderr, "refusing to write a page for %q: not a plain data source name\n", name)
			os.Exit(1)
		}
		path := filepath.Join("docs", "data-sources", filepath.Base(short+".md"))

		// A generated page is a skeleton with prose filled in by hand
		// afterwards. Overwriting one silently would throw that prose away, so
		// it takes -force to do it.
		if _, err := os.Stat(path); err == nil && !*force {
			fmt.Printf("skipped %s (already exists; pass -force to regenerate)\n", path)
			continue
		}
		if err := os.WriteFile(path, []byte(out.String()), 0o600); err != nil {
			panic(err)
		}
		fmt.Println("wrote", path)
	}
}
