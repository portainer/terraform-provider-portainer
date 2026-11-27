package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

func TestVersionLess(t *testing.T) {
	t.Parallel()
	cases := []struct {
		a, b string
		want bool
	}{
		{"2.45.3", "3.0.0", true},
		{"3.0.0", "3.0.0", false},
		{"3.0.1", "3.0.0", false},
		{"3.0", "3.0.0", false},
		{"3.0.0-rc1", "3.0.0", false},
		{"2.9.0", "2.10.0", true},
	}
	for _, tc := range cases {
		if got := versionLess(tc.a, tc.b); got != tc.want {
			t.Errorf("versionLess(%q, %q) = %v, want %v", tc.a, tc.b, got, tc.want)
		}
	}
}

// TestExplainUnsupported: a 404 from a server too old or of the wrong edition
// names the requirement; on a server that meets it, the error is unchanged.
func TestExplainUnsupported(t *testing.T) {
	t.Parallel()
	notFound := &apiStatusError{Method: "POST", URL: "http://x/api/endpoints/d2k", StatusCode: http.StatusNotFound, Body: "404 page not found"}
	cases := map[string]struct {
		version, edition string
		explained        bool
	}{
		"community edition":  {"2.45.3", "CE", true},
		"old business":       {"2.45.3", "BE", true},
		"business 3.0":       {"3.0.0", "BE", false},
		"business 3.0 as EE": {"3.0.1", "EE", false},
		"old EE":             {"2.45.3", "EE", true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
				"ServerVersion": tc.version, "ServerEdition": tc.edition,
			}))
			err := explainUnsupported(context.Background(), mock.Client(), notFound, "portainer_d2k_environment", requiresBE30)
			explained := strings.Contains(err.Error(), "requires Portainer Business Edition 3.0 or later")
			if explained != tc.explained {
				t.Errorf("explained = %v, want %v: %v", explained, tc.explained, err)
			}
			if explained && !strings.Contains(err.Error(), tc.edition+" "+tc.version) {
				t.Errorf("the error should name the server, got %v", err)
			}
		})
	}

	t.Run("other errors untouched", func(t *testing.T) {
		mock := NewMockServer(t)
		forbidden := fmt.Errorf("wrapped: %w", &apiStatusError{StatusCode: http.StatusForbidden})
		if err := explainUnsupported(context.Background(), mock.Client(), forbidden, "x", requiresBE30); !errors.Is(err, forbidden) {
			t.Errorf("a 403 must be returned unchanged, got %v", err)
		}
		if len(mock.Requests()) != 0 {
			t.Error("the version must only be asked for after a 404")
		}
	})
}

// TestExplainUnsupported_AnyEdition: a requirement without an edition (the
// deployment scale API, 2.45) explains the 404 of the 2.39 LTS line on either
// edition and leaves a real 404 on a newer server alone.
func TestExplainUnsupported_AnyEdition(t *testing.T) {
	t.Parallel()
	notFound := &apiStatusError{Method: "PUT", URL: "http://x/api/kubernetes/4/namespaces/default/deployments/web/scale", StatusCode: http.StatusNotFound, Body: "404 page not found"}
	cases := map[string]struct {
		version, edition string
		explained        bool
	}{
		"community 2.39": {"2.39.9", "CE", true},
		"business 2.39":  {"2.39.9", "EE", true},
		"community 2.45": {"2.45.4", "CE", false},
		"business 3.1.0": {"3.1.0", "EE", false},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			mock := NewMockServer(t)
			mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{
				"ServerVersion": tc.version, "ServerEdition": tc.edition,
			}))
			err := explainUnsupported(context.Background(), mock.Client(), notFound, "portainer_kubernetes_deployment_scale", requires245)
			explained := strings.Contains(err.Error(), "requires Portainer 2.45 or later; this server is Portainer "+tc.edition+" "+tc.version)
			if explained != tc.explained {
				t.Errorf("explained = %v, want %v: %v", explained, tc.explained, err)
			}
		})
	}
}

// TestExplainUnsupported_Hint: a server that meets the requirement but whose
// router has no such route (an API behind a feature flag) gets the hint; an
// application 404 such as a deleted object does not.
func TestExplainUnsupported_Hint(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{"ServerVersion": "3.1.0", "ServerEdition": "EE"}))
	noRoute := &apiStatusError{Method: "GET", URL: "http://x/api/oidc_clients", StatusCode: http.StatusNotFound, Body: "404 page not found"}
	err := explainUnsupported(context.Background(), mock.Client(), noRoute, "portainer_oidc_client", requiresBE31OIDC)
	if !strings.Contains(err.Error(), "--feat=oidc") || !strings.Contains(err.Error(), "EE 3.1.0") {
		t.Errorf("expected the feature flag hint, got %v", err)
	}
	missing := &apiStatusError{Method: "GET", URL: "http://x/api/oidc_clients/7", StatusCode: http.StatusNotFound, Body: `{"message":"Unable to find an OIDC client"}`}
	if err := explainUnsupported(context.Background(), mock.Client(), missing, "portainer_oidc_client", requiresBE31OIDC); !errors.Is(err, missing) || strings.Contains(err.Error(), "feature flag") {
		t.Errorf("an application 404 must stay as it is, got %v", err)
	}
}

// TestExplainRemovedAPI: the router 404 of a removed API names the server and
// where the API still exists; an application 404 is left alone.
func TestExplainRemovedAPI(t *testing.T) {
	t.Parallel()
	mock := NewMockServer(t)
	mock.On("GET", "/system/version", RespondJSON(http.StatusOK, map[string]interface{}{"ServerVersion": "3.1.0", "ServerEdition": "EE"}))
	noRoute := &apiStatusError{Method: "POST", URL: "http://x/api/chat", StatusCode: http.StatusNotFound, Body: "404 page not found"}
	err := explainRemovedAPI(context.Background(), mock.Client(), noRoute, "portainer_chat", "gone")
	if !strings.Contains(err.Error(), "portainer_chat is not available on Portainer EE 3.1.0: gone") || !errors.Is(err, noRoute) {
		t.Errorf("unexpected error %v", err)
	}
	appErr := &apiStatusError{StatusCode: http.StatusNotFound, Body: `{"message":"not found"}`}
	if err := explainRemovedAPI(context.Background(), mock.Client(), appErr, "x", "gone"); err != appErr { //nolint:errorlint // the same value, unwrapped, is expected
		t.Errorf("an application 404 must stay as it is, got %v", err)
	}
}
