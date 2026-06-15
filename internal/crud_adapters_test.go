package internal

import (
	"context"
	"errors"
	"strings"

	"github.com/hashicorp/terraform-plugin-sdk/v2/diag"
	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/schema"
)

// diagErr collapses a diag.Diagnostics into a single error so the existing
// table-driven tests — which assert on a returned error — keep working
// unchanged after the CRUD handlers were migrated from the legacy
// func(...) error signatures to the context-aware diag.Diagnostics ones.
//
// Only Error-severity diagnostics are folded in; warnings are ignored, matching
// the previous behaviour where handlers could only return an error or nil.
func diagErr(diags diag.Diagnostics) error {
	if !diags.HasError() {
		return nil
	}
	msgs := make([]string, 0, len(diags))
	for _, d := range diags {
		if d.Severity != diag.Error {
			continue
		}
		if d.Detail != "" {
			msgs = append(msgs, d.Summary+": "+d.Detail)
		} else {
			msgs = append(msgs, d.Summary)
		}
	}
	return errors.New(strings.Join(msgs, "; "))
}

// rcCreate/rcRead/rcUpdate/rcDelete invoke the context-aware CRUD handlers with
// a background context and return the result as an error. They let test
// call-sites stay close to their original r.Create(d, meta) form via a purely
// mechanical rewrite (r.Create( -> rcCreate(r, ).
func rcCreate(r *schema.Resource, d *schema.ResourceData, m interface{}) error {
	return diagErr(r.CreateContext(context.Background(), d, m))
}

func rcRead(r *schema.Resource, d *schema.ResourceData, m interface{}) error {
	return diagErr(r.ReadContext(context.Background(), d, m))
}

func rcUpdate(r *schema.Resource, d *schema.ResourceData, m interface{}) error {
	return diagErr(r.UpdateContext(context.Background(), d, m))
}

func rcDelete(r *schema.Resource, d *schema.ResourceData, m interface{}) error {
	return diagErr(r.DeleteContext(context.Background(), d, m))
}
