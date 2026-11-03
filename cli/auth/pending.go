package auth

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
)

// ErrLoginPending means a login without a terminal printed its URL and
// stopped before the user confirmed it. Running bl login again resumes it.
var ErrLoginPending = errors.New("login pending: confirm it in the browser, then run bl login again")

// pendingLogin is a device login started without a terminal and not yet
// confirmed, kept so the next bl login resumes it instead of starting over.
type pendingLogin struct {
	DeviceCode string    `json:"device_code"`
	URL        string    `json:"url"`
	Workspace  string    `json:"workspace,omitempty"`
	Expires    time.Time `json:"expires"`
	// TokenURL is where the device code is redeemed. A login started against
	// one API is never finished against another, such as one a project's
	// BL_API_URL selects later.
	TokenURL string `json:"token_url"`
}

func pendingLoginPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".blaxel", "pending-login.json"), nil
}

// loadPendingLogin returns the pending login, if one exists, has not expired
// and was started against the API in use now.
func loadPendingLogin(now time.Time) (pendingLogin, bool) {
	path, err := pendingLoginPath()
	if err != nil {
		return pendingLogin{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return pendingLogin{}, false
	}
	var pending pendingLogin
	if json.Unmarshal(data, &pending) != nil || pending.DeviceCode == "" || pending.URL == "" || !now.Before(pending.Expires) ||
		pending.TokenURL != blaxel.BuildOAuthTokenURL() {
		clearPendingLogin()
		return pendingLogin{}, false
	}
	return pending, true
}

func savePendingLogin(pending pendingLogin) error {
	path, err := pendingLoginPath()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	data, err := json.Marshal(pending)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o600)
}

func clearPendingLogin() {
	if path, err := pendingLoginPath(); err == nil {
		_ = os.Remove(path)
	}
}
