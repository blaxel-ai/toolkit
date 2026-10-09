package auth

import (
	"bytes"
	"context"
	"encoding/json"
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
	// deviceRequestTimeout bounds the request that starts a login.
	deviceRequestTimeout = 30 * time.Second
)

// LoginDevice logs in with the browser for bl login, and exits on failure.
func LoginDevice(workspace string) {
	if err := LoginWithDevice(workspace); err != nil {
		core.PrintError("Login", err)
		core.ExitWithError(err)
	}
}

// LoginWithDevice logs in with the browser and saves the credentials. With an
// empty workspace, the user picks one of their workspaces after signing in.
// Without a terminal (such as a coding agent running bl login) nothing is
// asked: the workspace is chosen for them and the login URL is printed on its
// own line. Failures are returned, so callers such as bl setup can carry on.
func LoginWithDevice(workspace string) error {
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()
	return loginWithDevice(ctx, workspace, core.IsTerminalInteractive())
}

func loginWithDevice(ctx context.Context, workspace string, interactive bool) error {
	deviceLogin, opened, err := StartDeviceLogin(ctx)
	if err != nil {
		return err
	}
	switch {
	case !interactive:
		if opened {
			core.PrintInfo("Opened the login page in your browser. If it did not open, visit this URL:")
		} else {
			core.PrintInfo("Visit this URL to finish logging in:")
		}
		core.Print(deviceLogin.VerificationURIComplete)
		core.PrintInfo(fmt.Sprintf("Waiting up to %s for you to confirm the login in your browser...", describeWait(devicePollInterval*time.Duration(devicePollAttempts))))
	case opened:
		core.PrintInfo(fmt.Sprintf("Opened URL in browser. If it's not working, please open it manually: %s", deviceLogin.VerificationURIComplete))
		core.PrintInfo("Waiting for you to confirm the login in your browser...")
	default:
		core.PrintInfo(fmt.Sprintf("Please visit the following URL to finish logging in: %s", deviceLogin.VerificationURIComplete))
		core.PrintInfo("Waiting for you to confirm the login in your browser...")
	}

	creds, err := WaitForDeviceLogin(ctx, deviceLogin.DeviceCode)
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
	return blaxel.Credentials{
		AccessToken:  token.AccessToken,
		RefreshToken: token.RefreshToken,
		ExpiresIn:    token.ExpiresIn,
		DeviceCode:   deviceCode,
	}, nil
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
	return DeviceLoginFinalizeResponse{}, core.MarkExpectedError(
		fmt.Errorf("timed out waiting for confirmation in the browser"),
		core.CLIErrorOperational,
	)
}

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
