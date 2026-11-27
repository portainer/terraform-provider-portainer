package internal

import (
	"sort"
)

// Portainer reports authorizations as an open-ended map of authorization name
// to a boolean, and Portainer adds names to it between releases. The
// portainer_user_namespaces and portainer_current_user_authorizations data
// sources therefore surface the sorted list of granted names, which is what a
// configuration can act on, alongside the raw response for anything else.

// grantedAuthorizations picks out the authorization names that are switched
// on, sorted so the output is stable between two identical plans.
func grantedAuthorizations(authorizations map[string]bool) []string {
	granted := make([]string, 0, len(authorizations))
	for name, allowed := range authorizations {
		if allowed {
			granted = append(granted, name)
		}
	}
	sort.Strings(granted)
	return granted
}
