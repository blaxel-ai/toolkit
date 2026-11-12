package core

import (
	"context"
	"strings"
	"sync"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
)

// Resources record their creator in metadata.createdBy as "<subType>:<sub>",
// e.g. "user:6b2f5df3-..." or "service_account:<client id>". The table view
// shows the creator's email instead, joined from the workspace member list.
// The control plane already serves that list to every member, so one extra
// request per command is enough; JSON and YAML output keep the raw id.

const creatorLookupTimeout = 10 * time.Second

// The member and service account lists are read once per client and
// workspace, so `bl get sandboxes --watch` does not repeat them (or wait out a
// failing lookup) on every refresh. A failed lookup is kept as an empty list:
// the column then shows raw ids for the rest of the command.
var creatorCache struct {
	sync.Mutex
	client    *blaxel.Client
	workspace string
	users     map[string]string
	accounts  map[string]string
}

func cachedCreatorNames(ctx context.Context, wantUsers, wantAccounts bool) (users, accounts map[string]string) {
	creatorCache.Lock()
	defer creatorCache.Unlock()
	if creatorCache.client != GetClient() || creatorCache.workspace != GetWorkspace() {
		creatorCache.client, creatorCache.workspace = GetClient(), GetWorkspace()
		creatorCache.users, creatorCache.accounts = nil, nil
	}
	if wantUsers && creatorCache.users == nil {
		creatorCache.users = map[string]string{}
		if list, err := fetchWorkspaceUsers(ctx); err == nil {
			for _, u := range list {
				if name := userDisplayName(u); name != "" {
					creatorCache.users[u.Sub] = name
				}
			}
		}
	}
	if wantAccounts && creatorCache.accounts == nil {
		creatorCache.accounts = map[string]string{}
		if list, err := fetchWorkspaceServiceAccounts(ctx); err == nil {
			for _, sa := range list {
				if sa.Name != "" {
					creatorCache.accounts[sa.ClientID] = sa.Name
				}
			}
		}
	}
	return creatorCache.users, creatorCache.accounts
}

type workspaceUser struct {
	Sub        string `json:"sub"`
	Email      string `json:"email"`
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
}

type workspaceServiceAccount struct {
	ClientID string `json:"client_id"`
	Name     string `json:"name"`
}

// GET /users is open to every workspace member. It is not in the generated
// SDK, so it is called raw.
func fetchWorkspaceUsers(ctx context.Context) ([]workspaceUser, error) {
	var users []workspaceUser
	err := GetClient().Get(ctx, "users", nil, &users)
	return users, err
}

// GET /service_accounts is admin-only; callers treat any error as "unknown".
func fetchWorkspaceServiceAccounts(ctx context.Context) ([]workspaceServiceAccount, error) {
	var accounts []workspaceServiceAccount
	err := GetClient().Get(ctx, "service_accounts", nil, &accounts)
	return accounts, err
}

// splitCreator returns the subject type and subject of a createdBy value.
// A value without a known type prefix is treated as a user subject.
func splitCreator(createdBy string) (subType, sub string) {
	if kind, rest, ok := strings.Cut(createdBy, ":"); ok {
		switch kind {
		case "user":
			return "user", rest
		case "service_account", "serviceaccount":
			return "service_account", rest
		}
	}
	return "user", createdBy
}

// resolveCreators maps each createdBy value to a readable name: the user's
// email, or the service account's name. It makes at most one users request
// and, only when a service account created something, one service accounts
// request (admin-only, so a viewer falls back to the raw id), and reuses both
// for later calls with the same client. Any value that cannot be resolved maps
// to itself.
func resolveCreators(createdBy []string) map[string]string {
	resolved := make(map[string]string, len(createdBy))
	var hasUser, hasServiceAccount bool
	for _, value := range createdBy {
		if value == "" || value == "-" {
			continue
		}
		resolved[value] = value
		if kind, _ := splitCreator(value); kind == "service_account" {
			hasServiceAccount = true
		} else {
			hasUser = true
		}
	}
	if len(resolved) == 0 || GetClient() == nil {
		return resolved
	}

	ctx, cancel := context.WithTimeout(context.Background(), creatorLookupTimeout)
	defer cancel()

	users, accounts := cachedCreatorNames(ctx, hasUser, hasServiceAccount)

	for value := range resolved {
		kind, sub := splitCreator(value)
		lookup := users
		if kind == "service_account" {
			lookup = accounts
		}
		if name, ok := lookup[sub]; ok {
			resolved[value] = name
		}
	}
	return resolved
}

func userDisplayName(u workspaceUser) string {
	if u.Email != "" {
		return u.Email
	}
	return strings.TrimSpace(u.GivenName + " " + u.FamilyName)
}
