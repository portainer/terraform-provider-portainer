package internal

import (
	"context"
	"testing"

	"github.com/hashicorp/terraform-plugin-go/tfprotov5"
)

// TestProviderServer_Schemas: the server reports every schema without an
// error, with or without PORTAINER_ENDPOINT set.
func TestProviderServer_Schemas(t *testing.T) {
	for _, env := range []string{"", "https://portainer.example.com"} {
		t.Run("PORTAINER_ENDPOINT="+env, func(t *testing.T) {
			t.Setenv("PORTAINER_ENDPOINT", env)
			checkServerSchemas(t)
		})
	}
}

func checkServerSchemas(t *testing.T) {
	server, err := NewProviderServer(context.Background())
	if err != nil {
		t.Fatalf("NewProviderServer: %v", err)
	}
	resp, err := server.GetProviderSchema(context.Background(), &tfprotov5.GetProviderSchemaRequest{})
	if err != nil {
		t.Fatalf("GetProviderSchema: %v", err)
	}
	for _, d := range resp.Diagnostics {
		if d.Severity == tfprotov5.DiagnosticSeverityError {
			t.Errorf("%s: %s", d.Summary, d.Detail)
		}
	}
	if len(resp.ResourceSchemas) == 0 || len(resp.DataSourceSchemas) == 0 {
		t.Errorf("expected resources and data sources, got %d and %d", len(resp.ResourceSchemas), len(resp.DataSourceSchemas))
	}
}
