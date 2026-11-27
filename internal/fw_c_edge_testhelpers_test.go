package internal

import (
	"regexp"
	"strings"
)

// edgeErrRe matches an error text as Terraform prints it, wrapped over
// several lines: every space may be any run of whitespace.
func edgeErrRe(text string) *regexp.Regexp {
	return regexp.MustCompile(strings.ReplaceAll(regexp.QuoteMeta(text), " ", `\s+`))
}
