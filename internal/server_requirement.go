package internal

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
)

// serverRequirement is what a feature needs from the Portainer server: the
// Business Edition, a minimum version, or both.
type serverRequirement struct {
	BusinessEdition bool
	MinVersion      string // "3.0.0"; empty for any version
	// Hint explains a route that is missing although the server meets the
	// edition and version, such as an API behind a feature flag.
	Hint string
}

func (r serverRequirement) String() string {
	s := "Portainer"
	if r.BusinessEdition {
		s += " Business Edition"
	}
	if r.MinVersion != "" {
		s += " " + strings.TrimSuffix(r.MinVersion, ".0") + " or later"
	}
	return s
}

// requiresBE is the requirement of a Business Edition API that every
// supported release has, such as alerting.
var requiresBE = serverRequirement{BusinessEdition: true}

// requiresBE30 is the requirement of everything Portainer 3.0 added: D2K,
// KubeSolo, the reconcile engine, the Gateway API. 3.x is Business Edition
// only; Community Edition stays on 2.x.
var requiresBE30 = serverRequirement{BusinessEdition: true, MinVersion: "3.0.0"}

// requiresBE31OIDC is the requirement of Portainer's built-in OIDC provider:
// Business Edition 3.1.0 serves its API only when started with the "oidc"
// feature flag.
var requiresBE31OIDC = serverRequirement{
	BusinessEdition: true, MinVersion: "3.1.0",
	Hint: "Portainer serves the OIDC provider only when started with the oidc feature flag (--feat=oidc or FEATURE_FLAG=oidc; Helm chart: feature.flags[0]=--feat=oidc)",
}

// requiresBE31Addons is the requirement of the add-on API Business Edition 3.1
// added: add-ons run only when Portainer itself runs on Kubernetes.
var requiresBE31Addons = serverRequirement{
	BusinessEdition: true, MinVersion: "3.1.0",
	Hint: "add-ons need Portainer itself running on Kubernetes (GET /api/system/info reports addonsAvailable)",
}

// requiresBE245 is the requirement of what the Business Edition 2.45 API is
// the first published specification to have, such as the observability
// discovery endpoint.
var requiresBE245 = serverRequirement{BusinessEdition: true, MinVersion: "2.45.0"}

// requires241, requires242 and requires245 are the requirements of APIs both
// editions gained after the 2.39 LTS line, which the provider still supports:
// the environment summary (2.41), a user's effective access (2.42) and the
// deployment read, scale and rollback endpoints (2.45).
var (
	requires241 = serverRequirement{MinVersion: "2.41.0"}
	requires242 = serverRequirement{MinVersion: "2.42.0"}
	requires245 = serverRequirement{MinVersion: "2.45.0"}
)

// requiresBE2452 is the requirement of what Business Edition 2.45.2 LTS added
// and 3.0 kept: issuing, inspecting and revoking scoped kubeconfigs.
var requiresBE2452 = serverRequirement{BusinessEdition: true, MinVersion: "2.45.2"}

// explainUnsupported turns the 404 an older or Community Edition server
// answers for an endpoint it does not have into an error that names what the
// feature needs and what the server is. Only for endpoints that cannot 404
// for any other reason (creates, lists, singletons): on a Read, a 404 means the
// object is gone. Any other error is returned unchanged.
//
// The server is only asked for its version when the request already failed,
// so a supported server costs no extra call.
func explainUnsupported(ctx context.Context, client *APIClient, err error, feature string, req serverRequirement) error {
	if !isAPINotFound(err) && !isAPIStatus(err, http.StatusMethodNotAllowed) {
		return err
	}
	var version struct {
		ServerVersion string `json:"ServerVersion"`
		ServerEdition string `json:"ServerEdition"`
	}
	if verr := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/system/version", nil, &version); verr != nil || version.ServerVersion == "" {
		return err
	}
	edition := version.ServerEdition
	if edition == "" {
		edition = "CE"
	}
	// Business Edition reports itself as "EE" (the API's own name for it);
	// "BE" is accepted too.
	if (req.BusinessEdition && edition != "EE" && edition != "BE") || (req.MinVersion != "" && versionLess(version.ServerVersion, req.MinVersion)) {
		return fmt.Errorf("%s requires %s; this server is Portainer %s %s: %w", feature, req, edition, version.ServerVersion, err)
	}
	// The server meets the requirement, yet its router has no such route.
	var se *apiStatusError
	if req.Hint != "" && errors.As(err, &se) && strings.Contains(se.Body, "404 page not found") {
		return fmt.Errorf("%s is not available on this Portainer %s %s: %s: %w", feature, edition, version.ServerVersion, req.Hint, err)
	}
	return err
}

// isAPIStatus reports whether err is an API response with the given status.
func isAPIStatus(err error, status int) bool {
	var se *apiStatusError
	return errors.As(err, &se) && se.StatusCode == status
}

// versionLess compares dotted versions numerically ("2.45.3" < "3.0.0");
// anything after a "-" (a pre-release suffix) is ignored.
func versionLess(a, b string) bool {
	pa, pb := versionParts(a), versionParts(b)
	for i := 0; i < len(pa) || i < len(pb); i++ {
		var x, y int
		if i < len(pa) {
			x = pa[i]
		}
		if i < len(pb) {
			y = pb[i]
		}
		if x != y {
			return x < y
		}
	}
	return false
}

func versionParts(v string) []int {
	v, _, _ = strings.Cut(strings.TrimPrefix(v, "v"), "-")
	fields := strings.Split(v, ".")
	parts := make([]int, 0, len(fields))
	for _, f := range fields {
		n, _ := strconv.Atoi(f)
		parts = append(parts, n)
	}
	return parts
}

// explainRemovedAPI turns the router 404 ("404 page not found") of an API that
// Portainer removed into an error that says so: note says where the API still
// exists or what replaces it. Other errors are returned unchanged.
func explainRemovedAPI(ctx context.Context, client *APIClient, err error, feature, note string) error {
	var se *apiStatusError
	if !errors.As(err, &se) || se.StatusCode != http.StatusNotFound || !strings.Contains(se.Body, "404 page not found") {
		return err
	}
	var version struct {
		ServerVersion string `json:"ServerVersion"`
		ServerEdition string `json:"ServerEdition"`
	}
	server := "this Portainer"
	if verr := doJSON(ctx, client, http.MethodGet, client.Endpoint+"/system/version", nil, &version); verr == nil && version.ServerVersion != "" {
		edition := version.ServerEdition
		if edition == "" {
			edition = "CE"
		}
		server = fmt.Sprintf("Portainer %s %s", edition, version.ServerVersion)
	}
	return fmt.Errorf("%s is not available on %s: %s: %w", feature, server, note, err)
}
