package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
)

// fakeEndpointGroups is a stateful stand-in for Portainer's environment group
// API (groups, their access policies and the group of each environment),
// shared by the endpoint group, access, membership and association tests.
type fakeEndpointGroups struct {
	mu        sync.Mutex
	nextID    int64
	groups    map[int64]map[string]interface{}
	endpoints map[int64]int64 // environment ID -> group ID
	relations []map[string]interface{}
}

func newFakeEndpointGroups(m *MockServer) *fakeEndpointGroups {
	f := &fakeEndpointGroups{nextID: 1, groups: map[int64]map[string]interface{}{}, endpoints: map[int64]int64{}}
	f.register(m)
	return f
}

// seed adds a group with a fixed ID.
func (f *fakeEndpointGroups) seed(id int64, name string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.groups[id] = map[string]interface{}{
		"Id": id, "Name": name, "Description": "", "TagIds": []int64{},
		"UserAccessPolicies": map[string]interface{}{}, "TeamAccessPolicies": map[string]interface{}{},
	}
	if id >= f.nextID {
		f.nextID = id
	}
}

func (f *fakeEndpointGroups) group(id int64) map[string]interface{} {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.groups[id]
}

func (f *fakeEndpointGroups) remove(id int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.groups, id)
}

func (f *fakeEndpointGroups) setEndpointGroup(endpointID, groupID int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.endpoints[endpointID] = groupID
}

func (f *fakeEndpointGroups) removeEndpoint(endpointID int64) {
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.endpoints, endpointID)
}

func (f *fakeEndpointGroups) notFound(w http.ResponseWriter) {
	RespondString(http.StatusNotFound, "application/json", `{"message":"Unable to find an environment group with the specified identifier inside the database"}`)(w, nil)
}

func (f *fakeEndpointGroups) register(m *MockServer) {
	m.On("GET", "/endpoint_groups", func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		ids := make([]int64, 0, len(f.groups))
		for id := range f.groups {
			ids = append(ids, id)
		}
		sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
		out := make([]map[string]interface{}, 0, len(ids))
		for _, id := range ids {
			out = append(out, f.groups[id])
		}
		RespondJSON(http.StatusOK, out)(w, r)
	})
	m.On("POST", "/endpoint_groups", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name        string  `json:"name"`
			Description string  `json:"description"`
			TagIDs      []int64 `json:"tagIDs"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.nextID++
		id := f.nextID
		if body.TagIDs == nil {
			body.TagIDs = []int64{}
		}
		g := map[string]interface{}{
			"Id": id, "Name": body.Name, "Description": body.Description, "TagIds": body.TagIDs,
			"UserAccessPolicies": map[string]interface{}{}, "TeamAccessPolicies": map[string]interface{}{},
		}
		f.groups[id] = g
		f.mu.Unlock()
		RespondJSON(http.StatusOK, g)(w, r)
	})
	for id := int64(1); id <= 20; id++ {
		id := id
		p := fmt.Sprintf("/endpoint_groups/%d", id)
		m.On("GET", p, func(w http.ResponseWriter, r *http.Request) {
			g := f.group(id)
			if g == nil {
				f.notFound(w)
				return
			}
			f.mu.Lock()
			defer f.mu.Unlock()
			RespondJSON(http.StatusOK, g)(w, r)
		})
		m.On("PUT", p, func(w http.ResponseWriter, r *http.Request) {
			var body map[string]interface{}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			g := f.groups[id]
			if g == nil {
				f.notFound(w)
				return
			}
			// endpointGroupUpdate applies each field only when it is sent.
			for key, field := range map[string]string{"name": "Name", "description": "Description", "tagIDs": "TagIds"} {
				if v, ok := body[key]; ok && v != nil && v != "" {
					g[field] = v
				}
			}
			for _, field := range []string{"UserAccessPolicies", "TeamAccessPolicies"} {
				if v, ok := body[field]; ok && v != nil {
					g[field] = v
				}
			}
			RespondJSON(http.StatusOK, g)(w, r)
		})
		m.On("DELETE", p, func(w http.ResponseWriter, _ *http.Request) {
			if f.group(id) == nil {
				f.notFound(w)
				return
			}
			f.remove(id)
			w.WriteHeader(http.StatusNoContent)
		})
		// Environments 1..20 and their group.
		m.On("GET", fmt.Sprintf("/endpoints/%d", id), func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			groupID, ok := f.endpoints[id]
			f.mu.Unlock()
			if !ok {
				RespondString(http.StatusNotFound, "application/json", `{"message":"Unable to find an environment with the specified identifier inside the database"}`)(w, r)
				return
			}
			RespondJSON(http.StatusOK, map[string]interface{}{"Id": id, "Name": fmt.Sprintf("env-%d", id), "GroupId": groupID})(w, r)
		})
		m.On("PUT", fmt.Sprintf("/endpoints/%d/association", id), func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
		for endpointID := int64(1); endpointID <= 20; endpointID++ {
			endpointID := endpointID
			mp := fmt.Sprintf("/endpoint_groups/%d/endpoints/%d", id, endpointID)
			m.On("PUT", mp, func(w http.ResponseWriter, _ *http.Request) {
				f.setEndpointGroup(endpointID, id)
				w.WriteHeader(http.StatusNoContent)
			})
			m.On("DELETE", mp, func(w http.ResponseWriter, _ *http.Request) {
				f.setEndpointGroup(endpointID, 1)
				w.WriteHeader(http.StatusNoContent)
			})
		}
	}
	m.On("PUT", "/endpoints/relations", func(w http.ResponseWriter, r *http.Request) {
		var body map[string]interface{}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		f.relations = append(f.relations, body)
		f.mu.Unlock()
		w.WriteHeader(http.StatusNoContent)
	})
}

// a1LastBody decodes the body of the last request with that method and path.
func a1LastBody(m *MockServer, method, path string) map[string]interface{} {
	var out map[string]interface{}
	reqs := m.Requests()
	for i := len(reqs) - 1; i >= 0; i-- {
		if reqs[i].Method == method && reqs[i].Path == path {
			_ = reqs[i].DecodeJSON(&out)
			return out
		}
	}
	return nil
}

// a1CountRequests counts the requests with that method and path.
func a1CountRequests(m *MockServer, method, path string) int {
	n := 0
	for _, r := range m.Requests() {
		if r.Method == method && r.Path == path {
			n++
		}
	}
	return n
}
