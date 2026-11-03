package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"slices"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/charmbracelet/huh"
)

// DeviceLogin represents a device login request
type DeviceLogin struct {
	ClientID string `json:"client_id"`
	Scope    string `json:"scope"`
}

// DeviceLoginResponse represents the response from device login
type DeviceLoginResponse struct {
	DeviceCode              string `json:"device_code"`
	UserCode                string `json:"user_code"`
	VerificationURI         string `json:"verification_uri"`
	VerificationURIComplete string `json:"verification_uri_complete"`
	ExpiresIn               int    `json:"expires_in"`
	Interval                int    `json:"interval"`
}

// DeviceLoginFinalizeRequest represents a device login finalize request
type DeviceLoginFinalizeRequest struct {
	GrantType  string `json:"grant_type"`
	ClientID   string `json:"client_id"`
	DeviceCode string `json:"device_code"`
}

// DeviceLoginFinalizeResponse represents the response from device login finalize
type DeviceLoginFinalizeResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
	TokenType    string `json:"token_type"`
}

// AuthErrorResponse represents an error response from auth
type AuthErrorResponse struct {
	Error            string `json:"error"`
	ErrorDescription string `json:"error_description"`
}

// Device login polls for the browser confirmation every interval, for up to
// three minutes, which gives users time to review and confirm the login.
var (
	devicePollInterval = 3 * time.Second
	devicePollAttempts = 61
	// Without a terminal, bl login does not wait: an agent's shell often shows
	// output only once the command ends, so the URL would arrive after the wait.
	// It checks once, keeps the login pending, and the next bl login resumes it,
	// checking for up to deviceResumeAttempts intervals.
	deviceResumeAttempts = 10
	// pendingLoginLifetime is how long a pending login is kept when the server
	// does not say when its code expires.
	pendingLoginLifetime = 10 * time.Minute
	// deviceRequestTimeout bounds the request that starts a login.
	deviceRequestTimeout = 30 * time.Second
)

// LoginDevice logs in with the browser for bl login, and exits on failure.
func LoginDevice(workspace string) {
	err := LoginWithDevice(workspace)
	if errors.Is(err, ErrLoginPending) {
		// The URL and the step to resume are printed; it is not logged in yet.
		core.Exit(1)
	}
	if err != nil {
		core.PrintError("Login", err)
		core.ExitWithError(err)
	}
}

// LoginWithDevice logs in with the browser and saves the credentials. With an
// empty workspace, the user picks one of their workspaces after signing in.
// Without a terminal (such as a coding agent running bl login) nothing is
// asked and nothing waits: the login URL is printed on its own line, the login
// is kept pending and ErrLoginPending returned, and the next bl login finishes
// it once confirmed, choosing the workspace for the user. Failures are
// returned, so callers such as bl setup can carry on.
func LoginWithDevice(workspace string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	core.TrackCLILogin("started", nil)
	err := loginWithDevice(ctx, workspace, core.IsTerminalInteractive())
	// A pending login is neither a success nor a failure yet: the bl login
	// that resumes it records how it ends.
	if !errors.Is(err, ErrLoginPending) {
		core.TrackCLILoginResult(ctx, err)
	}
	return err
}

func loginWithDevice(ctx context.Context, workspace string, interactive bool) error {
	var creds blaxel.Credentials
	var err error
	if interactive {
		creds, err = waitForBrowserLogin(ctx)
	} else {
		creds, workspace, err = handOffBrowserLogin(ctx, workspace)
	}
	if err != nil {
		return err
	}
	if workspace == "" {
		names, err := WaitForLoginWorkspaces(ctx, creds, func(note string) { core.Print(note) })
		if err != nil {
			return err
		}
		if workspace, err = chooseWorkspace(names, interactive); err != nil {
			return err
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := SaveDeviceLogin(workspace, creds); err != nil {
		return err
	}
	core.PrintSuccess(fmt.Sprintf("Successfully logged in to workspace %s", workspace))
	return nil
}

// waitForBrowserLogin opens the login page and waits in the terminal until the
// user confirms it.
func waitForBrowserLogin(ctx context.Context) (blaxel.Credentials, error) {
	deviceLogin, opened, err := StartDeviceLogin(ctx)
	if err != nil {
		return blaxel.Credentials{}, err
	}
	if opened {
		core.PrintInfo(fmt.Sprintf("Opened URL in browser. If it's not working, please open it manually: %s", deviceLogin.VerificationURIComplete))
	} else {
		core.PrintInfo(fmt.Sprintf("Please visit the following URL to finish logging in: %s", deviceLogin.VerificationURIComplete))
	}
	core.PrintInfo("Waiting for you to confirm the login in your browser...")
	return WaitForDeviceLogin(ctx, deviceLogin.DeviceCode)
}

// handOffBrowserLogin is the browser login without a terminal, such as a
// coding agent running bl login. It prints the login URL and returns
// ErrLoginPending at once instead of waiting, so the agent can show the URL;
// the next bl login resumes the pending login and finishes it once the user
// has confirmed. It returns the workspace the pending login was started for
// when workspace is empty.
func handOffBrowserLogin(ctx context.Context, workspace string) (blaxel.Credentials, string, error) {
	tokenURL := blaxel.BuildOAuthTokenURL()
	if pending, ok := loadPendingLogin(time.Now()); ok {
		if workspace == "" {
			workspace = pending.Workspace
		}
		core.PrintInfo("Checking the login started by the previous bl login...")
		token, err := pollDeviceToken(ctx, tokenURL, pending.DeviceCode, 0, 1)
		if errors.Is(err, errDeviceLoginTimeout) && deviceResumeAttempts > 1 {
			core.PrintInfo(fmt.Sprintf("Waiting up to %s for you to confirm the login in your browser...", describeWait(devicePollInterval*time.Duration(deviceResumeAttempts-1))))
			token, err = pollDeviceToken(ctx, tokenURL, pending.DeviceCode, devicePollInterval, deviceResumeAttempts-1)
		}
		switch {
		case err == nil:
			clearPendingLogin()
			return deviceCredentials(token, pending.DeviceCode), workspace, nil
		case errors.Is(err, errDeviceLoginTimeout):
			core.PrintInfo("Not confirmed yet. Visit this URL to finish logging in:")
			core.Print(pending.URL)
			printResumeStep(workspace, pending.Expires)
			return blaxel.Credentials{}, workspace, ErrLoginPending
		case ctx.Err() != nil:
			return blaxel.Credentials{}, workspace, err
		}
		clearPendingLogin()
		core.PrintInfo(fmt.Sprintf("The previous login link can no longer be used (%v). Starting a new login.", err))
	}

	deviceLogin, opened, err := StartDeviceLogin(ctx)
	if err != nil {
		return blaxel.Credentials{}, workspace, err
	}
	lifetime := pendingLoginLifetime
	if deviceLogin.ExpiresIn > 0 {
		lifetime = time.Duration(deviceLogin.ExpiresIn) * time.Second
	}
	pending := pendingLogin{DeviceCode: deviceLogin.DeviceCode, URL: deviceLogin.VerificationURIComplete, Workspace: workspace, Expires: time.Now().Add(lifetime)}
	if opened {
		core.PrintInfo("Opened the login page in your browser. If it did not open, visit this URL:")
	} else {
		core.PrintInfo("Visit this URL to finish logging in:")
	}
	core.Print(deviceLogin.VerificationURIComplete)
	if err := savePendingLogin(pending); err != nil {
		return blaxel.Credentials{}, workspace, fmt.Errorf("failed to keep the pending login: %w", err)
	}
	token, err := pollDeviceToken(ctx, tokenURL, deviceLogin.DeviceCode, devicePollInterval, 1)
	switch {
	case err == nil:
		clearPendingLogin()
		return deviceCredentials(token, deviceLogin.DeviceCode), workspace, nil
	case errors.Is(err, errDeviceLoginTimeout):
		printResumeStep(workspace, pending.Expires)
		return blaxel.Credentials{}, workspace, ErrLoginPending
	}
	clearPendingLogin()
	return blaxel.Credentials{}, workspace, err
}

// printResumeStep says how to finish a pending login.
func printResumeStep(workspace string, expires time.Time) {
	command := "bl login"
	if workspace != "" {
		command += " " + workspace
	}
	core.PrintInfo(fmt.Sprintf("Login pending. After you confirm it in the browser, run %s again to finish. The link expires in %s.", command, describeWait(time.Until(expires))))
}

// describeWait says how long a wait is in whole minutes, or seconds when short.
func describeWait(d time.Duration) string {
	if d < 90*time.Second {
		return fmt.Sprintf("%d seconds", int(d.Round(time.Second)/time.Second))
	}
	return fmt.Sprintf("%d minutes", int(d.Round(time.Minute)/time.Minute))
}

// StartDeviceLogin asks for a device login and opens its page in the
// browser. opened is false where no browser could be opened. Cancelling ctx
// (such as skipping the login in bl setup) stops the request.
func StartDeviceLogin(ctx context.Context) (login DeviceLoginResponse, opened bool, err error) {
	login, err = requestDeviceLogin(ctx, blaxel.BuildOAuthDeviceURL())
	if err != nil {
		return login, false, err
	}
	return login, openBrowser(login.VerificationURIComplete) == nil, nil
}

// WaitForDeviceLogin waits until the user confirms the login in the browser.
func WaitForDeviceLogin(ctx context.Context, deviceCode string) (blaxel.Credentials, error) {
	token, err := pollDeviceToken(ctx, blaxel.BuildOAuthTokenURL(), deviceCode, devicePollInterval, devicePollAttempts)
	if err != nil {
		return blaxel.Credentials{}, err
	}
	clearPendingLogin()
	return deviceCredentials(token, deviceCode), nil
}

func deviceCredentials(token DeviceLoginFinalizeResponse, deviceCode string) blaxel.Credentials {
	return blaxel.Credentials{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresIn:    token.ExpiresIn,
		DeviceCode:   deviceCode,
	}
}

// LoginWorkspaces lists the workspaces the new login can use.
func LoginWorkspaces(creds blaxel.Credentials) ([]string, error) {
	workspaces, err := listWorkspaces(creds)
	if err != nil {
		return nil, fmt.Errorf("failed to list workspaces: %w", err)
	}
	if len(workspaces) == 0 {
		return nil, core.MarkExpectedError(
			fmt.Errorf("no workspaces are available for your account.\nVisit %s to create one", blaxel.GetAppURL()),
			core.CLIErrorOperational,
		)
	}
	names := make([]string, 0, len(workspaces))
	for _, ws := range workspaces {
		names = append(names, ws.Name)
	}
	return names, nil
}

// SaveDeviceLogin checks the login can use the workspace, then saves it as
// the current workspace.
func SaveDeviceLogin(workspace string, creds blaxel.Credentials) error {
	if err := validateWorkspace(workspace, creds); err != nil {
		return fmt.Errorf("error accessing workspace %s : %w", workspace, err)
	}
	if err := blaxel.SaveCredentials(workspace, creds); err != nil {
		return fmt.Errorf("failed to save credentials: %w", err)
	}
	if err := blaxel.SetCurrentWorkspace(workspace); err != nil {
		return fmt.Errorf("failed to set workspace: %w", err)
	}
	return nil
}

func requestDeviceLogin(ctx context.Context, url string) (DeviceLoginResponse, error) {
	payloadBytes, err := json.Marshal(DeviceLogin{ClientID: "blaxel", Scope: "offline_access"})
	if err != nil {
		return DeviceLoginResponse{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, deviceRequestTimeout)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadBytes))
	if err != nil {
		return DeviceLoginResponse{}, err
	}
	request.Header.Set("Content-Type", "application/json")
	res, err := http.DefaultClient.Do(request)
	if err != nil {
		return DeviceLoginResponse{}, fmt.Errorf("error making request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, _ := io.ReadAll(res.Body)

	var deviceLoginResponse DeviceLoginResponse
	if err := json.Unmarshal(body, &deviceLoginResponse); err != nil {
		return DeviceLoginResponse{}, fmt.Errorf("error unmarshalling response: %w", err)
	}
	if deviceLoginResponse.DeviceCode == "" || deviceLoginResponse.VerificationURIComplete == "" {
		return DeviceLoginResponse{}, core.MarkExpectedError(
			fmt.Errorf("unexpected device login response with status %d", res.StatusCode),
			core.CLIErrorOperational,
		)
	}
	return deviceLoginResponse, nil
}

// pollDeviceToken waits until the user confirms the login in the browser and
// returns the issued token.
func pollDeviceToken(ctx context.Context, url, deviceCode string, interval time.Duration, attempts int) (DeviceLoginFinalizeResponse, error) {
	payloadBytes, err := json.Marshal(DeviceLoginFinalizeRequest{
		GrantType:  "urn:ietf:params:oauth:grant-type:device_code",
		ClientID:   "blaxel",
		DeviceCode: deviceCode,
	})
	if err != nil {
		return DeviceLoginFinalizeResponse{}, err
	}
	for attempt := 0; attempt < attempts; attempt++ {
		select {
		case <-ctx.Done():
			return DeviceLoginFinalizeResponse{}, ctx.Err()
		case <-time.After(interval):
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(payloadBytes))
		if err != nil {
			return DeviceLoginFinalizeResponse{}, err
		}
		request.Header.Set("Content-Type", "application/json")
		res, err := http.DefaultClient.Do(request)
		if err != nil {
			return DeviceLoginFinalizeResponse{}, fmt.Errorf("error making request: %w", err)
		}
		body, _ := io.ReadAll(res.Body)
		_ = res.Body.Close()

		// HTTP 202 or an authorization_pending error: the user has not confirmed yet.
		if res.StatusCode == http.StatusAccepted {
			continue
		}
		if res.StatusCode == http.StatusBadRequest {
			var errorResponse AuthErrorResponse
			if json.Unmarshal(body, &errorResponse) == nil && errorResponse.Error == "authorization_pending" {
				continue
			}
		}
		if res.StatusCode != http.StatusOK {
			return DeviceLoginFinalizeResponse{}, core.MarkExpectedError(
				fmt.Errorf("authentication failed with status %d: %s", res.StatusCode, string(body)),
				core.CLIErrorAuthentication,
			)
		}
		var finalizeResponse DeviceLoginFinalizeResponse
		if err := json.Unmarshal(body, &finalizeResponse); err != nil {
			return DeviceLoginFinalizeResponse{}, fmt.Errorf("error unmarshalling response: %w", err)
		}
		return finalizeResponse, nil
	}
	return DeviceLoginFinalizeResponse{}, core.MarkExpectedError(errDeviceLoginTimeout, core.CLIErrorOperational)
}

// errDeviceLoginTimeout means the user had not confirmed the login when
// pollDeviceToken stopped checking.
var errDeviceLoginTimeout = errors.New("timed out waiting for confirmation in the browser")

// currentWorkspaceIndex is where the workspace in use (context.workspace in
// the config) sits in names, or -1 when the new login cannot use it.
func currentWorkspaceIndex(names []string) int {
	current, err := blaxel.CurrentContext()
	if err != nil {
		return -1
	}
	return slices.Index(names, current.Workspace)
}

// chooseWorkspace returns the user's only workspace, or asks which one to use.
// Without a terminal it chooses one and says how to use another.
func chooseWorkspace(workspaces []string, interactive bool) (string, error) {
	if len(workspaces) == 1 {
		return workspaces[0], nil
	}
	if interactive {
		return askWorkspace(workspaces, nil, nil)
	}
	workspace, why := workspaceWithoutAsking(workspaces)
	core.PrintInfo(fmt.Sprintf("No terminal to ask in, so using workspace %s (%s).", workspace, why))
	core.PrintInfo("To use another one, run bl login <workspace>. To switch between workspaces you are already logged in to, run bl workspaces <workspace>.")
	return workspace, nil
}

// workspaceWithoutAsking is the current workspace when the new login can use
// it, else the first by name (the API lists workspaces in a varying order).
func workspaceWithoutAsking(names []string) (workspace, why string) {
	if index := currentWorkspaceIndex(names); index >= 0 {
		return names[index], "your current workspace"
	}
	return slices.Min(names), fmt.Sprintf("the first of your %d workspaces by name", len(names))
}

// askWorkspace asks which workspace to connect to, starting on the current
// one. in and out replace the terminal in tests.
func askWorkspace(workspaces []string, in io.Reader, out io.Writer) (string, error) {
	// Get workspaces the user is already connected to
	cfg, _ := blaxel.LoadConfig()
	connectedWorkspaceSet := make(map[string]bool)
	for _, ws := range cfg.Workspaces {
		connectedWorkspaceSet[ws.Name] = true
	}
	options := make([]huh.Option[string], 0, len(workspaces))
	for _, name := range workspaces {
		displayName := name
		if connectedWorkspaceSet[name] {
			displayName = fmt.Sprintf("%s (already connected)", name)
		}
		options = append(options, huh.NewOption(displayName, name))
	}

	workspace := workspaces[max(currentWorkspaceIndex(workspaces), 0)]
	form := huh.NewForm(
		huh.NewGroup(
			huh.NewSelect[string]().
				Title("Choose a workspace").
				Description("Select the workspace you want to connect to").
				Options(options...).
				Value(&workspace),
		),
	)
	form.WithTheme(core.GetHuhTheme())
	if in != nil {
		form.WithInput(in)
	}
	if out != nil {
		form.WithOutput(out)
	}
	if err := form.Run(); err != nil {
		return "", fmt.Errorf("error selecting workspace: %w", err)
	}
	return workspace, nil
}
