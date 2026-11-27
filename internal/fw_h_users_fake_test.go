package internal

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
)

// fakeUsersAPI is a stateful stand-in for Portainer's user, team, team
// membership, API key and admin bootstrap endpoints, shared by the tests of
// portainer_user, portainer_user_admin, portainer_team,
// portainer_team_membership, portainer_user_memberships_sync and
// portainer_user_api_key.
type fakeUsersAPI struct {
	mu sync.Mutex

	nextID      int64
	users       map[int64]*fakeUser
	teams       map[int64]*portainerTeam
	memberships map[int64]*portainerTeamMembership
	keys        map[int]*portainerAPIKey

	// adminInitialized is what GET /users/admin/check reports.
	adminInitialized bool
	// initStatus, when set, is the status POST /users/admin/init answers.
	initStatus int
}

type fakeUser struct {
	portainerUser
	Password string
}

// fakeUsersMaxID bounds the IDs the fake serves: the mock server only routes
// exact paths, so every /x/{id} path is registered up front.
const fakeUsersMaxID = 30

func newFakeUsersAPI(m *MockServer) *fakeUsersAPI {
	f := &fakeUsersAPI{
		users:       map[int64]*fakeUser{},
		teams:       map[int64]*portainerTeam{},
		memberships: map[int64]*portainerTeamMembership{},
		keys:        map[int]*portainerAPIKey{},
	}
	f.register(m)
	return f
}

func (f *fakeUsersAPI) id() int64 {
	f.nextID++
	return f.nextID
}

// addUser adds a user directly, as if created outside Terraform.
func (f *fakeUsersAPI) addUser(username, password string, role int64) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.id()
	f.users[id] = &fakeUser{portainerUser: portainerUser{ID: id, Username: username, Role: role}, Password: password}
	return id
}

// addTeam adds a team directly.
func (f *fakeUsersAPI) addTeam(name string) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.id()
	f.teams[id] = &portainerTeam{ID: id, Name: name}
	return id
}

// addMembership adds a membership directly.
func (f *fakeUsersAPI) addMembership(userID, teamID, role int64) int64 {
	f.mu.Lock()
	defer f.mu.Unlock()
	id := f.id()
	f.memberships[id] = &portainerTeamMembership{ID: id, UserID: userID, TeamID: teamID, Role: role}
	return id
}

func (f *fakeUsersAPI) membershipCount(userID, teamID int64) int {
	f.mu.Lock()
	defer f.mu.Unlock()
	n := 0
	for _, m := range f.memberships {
		if m.UserID == userID && m.TeamID == teamID {
			n++
		}
	}
	return n
}

func fakeRespond(w http.ResponseWriter, status int, body interface{}) {
	RespondJSON(status, body)(w, nil)
}

func fakeNotFound(w http.ResponseWriter) {
	fakeRespond(w, http.StatusNotFound, map[string]string{"message": "Object not found inside the database"})
}

// lastPathID returns the numeric last segment of the path.
func lastPathID(r *http.Request) int64 {
	parts := strings.Split(strings.TrimSuffix(r.URL.Path, "/"), "/")
	id, _ := strconv.ParseInt(parts[len(parts)-1], 10, 64)
	return id
}

// pathIDAt returns the numeric path segment that follows the given one.
func pathIDAt(r *http.Request, after string) int64 {
	parts := strings.Split(r.URL.Path, "/")
	for i, p := range parts {
		if p == after && i+1 < len(parts) {
			id, _ := strconv.ParseInt(parts[i+1], 10, 64)
			return id
		}
	}
	return 0
}

func (f *fakeUsersAPI) register(m *MockServer) {
	m.On("GET", "/users/admin/check", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		if !f.adminInitialized {
			fakeRespond(w, http.StatusNotFound, map[string]string{"message": "No administrator account found inside the database"})
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
	m.On("POST", "/users/admin/init", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Username, Password string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		if f.initStatus != 0 {
			fakeRespond(w, f.initStatus, map[string]string{"message": "init refused"})
			return
		}
		if f.adminInitialized {
			fakeRespond(w, http.StatusConflict, map[string]string{"message": "An administrator user already exists"})
			return
		}
		f.adminInitialized = true
		id := f.id()
		u := &fakeUser{portainerUser: portainerUser{ID: id, Username: body.Username, Role: 1}, Password: body.Password}
		f.users[id] = u
		fakeRespond(w, http.StatusOK, u.portainerUser)
	})

	m.On("POST", "/auth", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Username, Password string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		for _, u := range f.users {
			if strings.EqualFold(u.Username, body.Username) && u.Password == body.Password {
				fakeRespond(w, http.StatusOK, map[string]string{"jwt": fmt.Sprintf("jwt-%d", u.ID)})
				return
			}
		}
		fakeRespond(w, http.StatusUnprocessableEntity, map[string]string{"message": "Invalid credentials"})
	})

	// Users.
	m.On("GET", "/users", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		list := []portainerUser{}
		for i := int64(1); i <= f.nextID; i++ {
			if u, ok := f.users[i]; ok {
				list = append(list, u.portainerUser)
			}
		}
		fakeRespond(w, http.StatusOK, list)
	})
	m.On("POST", "/users", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Username string
			Password *string
			Role     int64
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		id := f.id()
		u := &fakeUser{portainerUser: portainerUser{ID: id, Username: body.Username, Role: body.Role, CreatedAuthenticationMethod: 1}}
		if body.Password != nil {
			u.Password = *body.Password
		} else {
			u.CreatedAuthenticationMethod = 2
		}
		f.users[id] = u
		fakeRespond(w, http.StatusOK, u.portainerUser)
	})
	for i := 1; i <= fakeUsersMaxID; i++ {
		p := fmt.Sprintf("/users/%d", i)
		m.On("GET", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if u, ok := f.users[lastPathID(r)]; ok {
				fakeRespond(w, http.StatusOK, u.portainerUser)
				return
			}
			fakeNotFound(w)
		})
		m.On("PUT", p, func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				Username string
				Role     int64
			}
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			u, ok := f.users[lastPathID(r)]
			if !ok {
				fakeNotFound(w)
				return
			}
			u.Username, u.Role = body.Username, body.Role
			fakeRespond(w, http.StatusOK, u.portainerUser)
		})
		m.On("DELETE", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			id := lastPathID(r)
			if _, ok := f.users[id]; !ok {
				fakeNotFound(w)
				return
			}
			delete(f.users, id)
			w.WriteHeader(http.StatusNoContent)
		})
		m.On("PUT", p+"/passwd", func(w http.ResponseWriter, r *http.Request) {
			var body struct{ Password, NewPassword string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			u, ok := f.users[pathIDAt(r, "users")]
			if !ok {
				fakeNotFound(w)
				return
			}
			if u.Password != body.Password {
				fakeRespond(w, http.StatusForbidden, map[string]string{"message": "Current password doesn't match"})
				return
			}
			u.Password = body.NewPassword
			w.WriteHeader(http.StatusNoContent)
		})
		m.On("POST", p+"/memberships/sync", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNoContent)
		})
		m.On("GET", p+"/tokens", func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			userID := int(pathIDAt(r, "users"))
			if _, ok := f.users[int64(userID)]; !ok {
				fakeNotFound(w)
				return
			}
			list := []portainerAPIKey{}
			for k := 1; k <= int(f.nextID); k++ {
				if key, ok := f.keys[k]; ok && key.UserID == userID {
					list = append(list, *key)
				}
			}
			fakeRespond(w, http.StatusOK, list)
		})
		m.On("POST", p+"/tokens", func(w http.ResponseWriter, r *http.Request) {
			var body struct{ Description, Password string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			userID := pathIDAt(r, "users")
			u, ok := f.users[userID]
			if !ok || u.Password != body.Password {
				fakeRespond(w, http.StatusForbidden, map[string]string{"message": "Invalid password"})
				return
			}
			id := int(f.id())
			key := &portainerAPIKey{ID: id, UserID: int(userID), Description: body.Description, Prefix: "ptr_abc", DateCreated: 1700000000}
			f.keys[id] = key
			fakeRespond(w, http.StatusCreated, map[string]interface{}{"rawAPIKey": fmt.Sprintf("ptr_raw_%d", id), "apiKey": key})
		})
		for k := 1; k <= fakeUsersMaxID; k++ {
			m.On("DELETE", fmt.Sprintf("%s/tokens/%d", p, k), func(w http.ResponseWriter, r *http.Request) {
				f.mu.Lock()
				defer f.mu.Unlock()
				id := int(lastPathID(r))
				if _, ok := f.keys[id]; !ok {
					fakeNotFound(w)
					return
				}
				delete(f.keys, id)
				w.WriteHeader(http.StatusNoContent)
			})
		}
	}

	// Teams.
	m.On("GET", "/teams", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		list := []portainerTeam{}
		for i := int64(1); i <= f.nextID; i++ {
			if t, ok := f.teams[i]; ok {
				list = append(list, *t)
			}
		}
		fakeRespond(w, http.StatusOK, list)
	})
	m.On("POST", "/teams", func(w http.ResponseWriter, r *http.Request) {
		var body struct{ Name string }
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		id := f.id()
		f.teams[id] = &portainerTeam{ID: id, Name: body.Name}
		fakeRespond(w, http.StatusOK, f.teams[id])
	})
	for i := 1; i <= fakeUsersMaxID; i++ {
		p := fmt.Sprintf("/teams/%d", i)
		m.On("GET", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			if t, ok := f.teams[lastPathID(r)]; ok {
				fakeRespond(w, http.StatusOK, t)
				return
			}
			fakeNotFound(w)
		})
		m.On("PUT", p, func(w http.ResponseWriter, r *http.Request) {
			var body struct{ Name string }
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			t, ok := f.teams[lastPathID(r)]
			if !ok {
				fakeNotFound(w)
				return
			}
			if body.Name != "" {
				t.Name = body.Name
			}
			fakeRespond(w, http.StatusOK, t)
		})
		m.On("DELETE", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			id := lastPathID(r)
			if _, ok := f.teams[id]; !ok {
				fakeNotFound(w)
				return
			}
			delete(f.teams, id)
			w.WriteHeader(http.StatusNoContent)
		})
	}

	// Team memberships.
	m.On("GET", "/team_memberships", func(w http.ResponseWriter, _ *http.Request) {
		f.mu.Lock()
		defer f.mu.Unlock()
		list := []portainerTeamMembership{}
		for i := int64(1); i <= f.nextID; i++ {
			if tm, ok := f.memberships[i]; ok {
				list = append(list, *tm)
			}
		}
		fakeRespond(w, http.StatusOK, list)
	})
	m.On("POST", "/team_memberships", func(w http.ResponseWriter, r *http.Request) {
		var body teamMembershipPayload
		_ = json.NewDecoder(r.Body).Decode(&body)
		f.mu.Lock()
		defer f.mu.Unlock()
		id := f.id()
		f.memberships[id] = &portainerTeamMembership{ID: id, Role: body.Role, TeamID: body.TeamID, UserID: body.UserID}
		fakeRespond(w, http.StatusOK, f.memberships[id])
	})
	for i := 1; i <= fakeUsersMaxID; i++ {
		p := fmt.Sprintf("/team_memberships/%d", i)
		m.On("PUT", p, func(w http.ResponseWriter, r *http.Request) {
			var body teamMembershipPayload
			_ = json.NewDecoder(r.Body).Decode(&body)
			f.mu.Lock()
			defer f.mu.Unlock()
			tm, ok := f.memberships[lastPathID(r)]
			if !ok {
				fakeNotFound(w)
				return
			}
			tm.Role, tm.TeamID, tm.UserID = body.Role, body.TeamID, body.UserID
			fakeRespond(w, http.StatusOK, tm)
		})
		m.On("DELETE", p, func(w http.ResponseWriter, r *http.Request) {
			f.mu.Lock()
			defer f.mu.Unlock()
			id := lastPathID(r)
			if _, ok := f.memberships[id]; !ok {
				fakeNotFound(w)
				return
			}
			delete(f.memberships, id)
			w.WriteHeader(http.StatusNoContent)
		})
	}
}

// requestsTo returns the recorded requests with the given method and path.
func requestsTo(m *MockServer, method, path string) []*RecordedRequest {
	var out []*RecordedRequest
	for _, r := range m.Requests() {
		if r.Method == method && r.Path == path {
			out = append(out, r)
		}
	}
	return out
}
