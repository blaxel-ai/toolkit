package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
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

func LoginDevice(workspace string) {
	url := blaxel.BuildOAuthDeviceURL()

	payload := DeviceLogin{
		ClientID: "blaxel",
		Scope:    "offline_access",
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))

	req.Header.Add("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		err = fmt.Errorf("error making request: %w", err)
		core.PrintError("Login", err)
		core.ExitWithError(err)
	}
	defer func() { _ = res.Body.Close() }()

	body, _ := io.ReadAll(res.Body)

	var deviceLoginResponse DeviceLoginResponse
	if err := json.Unmarshal(body, &deviceLoginResponse); err != nil {
		err = fmt.Errorf("error unmarshalling response: %w", err)
		core.PrintError("Login", err)
		core.ExitWithError(err)
	}

	// Open the URL in the default browser
	err = exec.Command("open", deviceLoginResponse.VerificationURIComplete).Start()
	if err != nil {
		core.PrintInfo(fmt.Sprintf("Please visit the following URL to finish logging in: %s", deviceLoginResponse.VerificationURIComplete))
	} else {
		core.PrintInfo(fmt.Sprintf("Opened URL in browser. If it's not working, please open it manually: %s", deviceLoginResponse.VerificationURIComplete))
	}
	core.PrintInfo("Waiting for you to confirm the login in your browser...")
	// Increase retries to 60 (60 * 3 seconds = 3 minutes total timeout)
	// This gives users enough time to review and confirm the login
	deviceModeLoginFinalize(deviceLoginResponse.DeviceCode, workspace, 60)
}

func deviceModeLoginFinalize(deviceCode string, workspace string, retries int) {
	time.Sleep(3 * time.Second)
	url := blaxel.BuildOAuthTokenURL()

	payload := DeviceLoginFinalizeRequest{
		GrantType:  "urn:ietf:params:oauth:grant-type:device_code",
		ClientID:   "blaxel",
		DeviceCode: deviceCode,
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		panic(err)
	}

	req, _ := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	req.Header.Add("Content-Type", "application/json")

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		err = fmt.Errorf("error making request: %w", err)
		core.PrintError("Login", err)
		core.ExitWithError(err)
	}
	defer func() { _ = res.Body.Close() }()

	body, _ := io.ReadAll(res.Body)

	// Check for pending authorization (HTTP 202 or authorization_pending error)
	if res.StatusCode == http.StatusAccepted {
		// HTTP 202 Accepted means authorization is pending
		if retries > 0 {
			deviceModeLoginFinalize(deviceCode, workspace, retries-1)
			return
		} else {
			err := fmt.Errorf("login timed out waiting for confirmation")
			core.PrintError("Login", err)
			core.ExitWithError(err)
		}
	}

	// Check for authorization_pending in error response
	if res.StatusCode == http.StatusBadRequest {
		var errorResponse AuthErrorResponse
		if err := json.Unmarshal(body, &errorResponse); err == nil {
			if errorResponse.Error == "authorization_pending" {
				// Authorization is pending - keep polling
				if retries > 0 {
					deviceModeLoginFinalize(deviceCode, workspace, retries-1)
					return
				} else {
					err := fmt.Errorf("login timed out waiting for confirmation")
					core.PrintError("Login", err)
					core.ExitWithError(err)
				}
			}
		}
	}

	var finalizeResponse DeviceLoginFinalizeResponse
	if err := json.Unmarshal(body, &finalizeResponse); err != nil {
		panic(err)
	}

	if res.StatusCode != http.StatusOK {
		// This is a real error, not just pending
		err := fmt.Errorf("authentication failed with status %d: %s", res.StatusCode, string(body))
		core.PrintError("Login", err)
		core.ExitWithError(err)
	}

	creds := blaxel.Credentials{
		AccessToken:  finalizeResponse.AccessToken,
		RefreshToken: finalizeResponse.RefreshToken,
		ExpiresIn:    finalizeResponse.ExpiresIn,
		DeviceCode:   deviceCode,
	}

	// If no workspace is provided, show a menu to select a workspace
	if workspace == "" {
		workspaces, err := listWorkspaces(creds)
		if err != nil {
			err = fmt.Errorf("failed to list workspaces: %w", err)
			core.PrintError("Login", err)
			core.ExitWithError(err)
			return
		}
		if len(workspaces) == 0 {
			err := fmt.Errorf("no workspaces are available for your account.\nVisit %s to create one", blaxel.GetAppURL())
			core.PrintError("Login", err)
			core.ExitWithError(err)
			return
		}

		// Get workspaces the user is already connected to
		cfg, _ := blaxel.LoadConfig()
		connectedWorkspaceSet := make(map[string]bool)
		for _, ws := range cfg.Workspaces {
			connectedWorkspaceSet[ws.Name] = true
		}

		// If only one workspace, use it directly
		if len(workspaces) == 1 {
			workspace = workspaces[0].Name
		} else {
			// Create options for huh form
			options := make([]huh.Option[string], 0, len(workspaces))
			for _, ws := range workspaces {
				displayName := ws.Name
				if connectedWorkspaceSet[ws.Name] {
					displayName = fmt.Sprintf("%s (already connected)", ws.Name)
				}
				options = append(options, huh.NewOption(displayName, ws.Name))
			}

			// Create huh form for workspace selection
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

			err := form.Run()
			if err != nil {
				err = fmt.Errorf("error selecting workspace: %w", err)
				core.PrintError("Login", err)
				core.ExitWithError(err)
			}
		}

		blaxel.SaveCredentials(workspace, creds)
		blaxel.SetCurrentWorkspace(workspace)
		core.PrintSuccess(fmt.Sprintf("Successfully logged in to workspace %s", workspace))
		return
	}

	err = validateWorkspace(workspace, creds)
	if err != nil {
		core.PrintError("Login", fmt.Errorf("error accessing workspace %s : %w", workspace, err))
	} else {
		blaxel.SaveCredentials(workspace, creds)
		blaxel.SetCurrentWorkspace(workspace)
		core.PrintSuccess(fmt.Sprintf("Successfully logged in to workspace %s", workspace))
	}
}
