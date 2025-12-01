package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"time"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/sdk"
	"github.com/charmbracelet/huh"
)

func LoginDevice(workspace string) {
	url := core.GetBaseURL() + "/login/device"

	payload := sdk.DeviceLogin{
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

	var deviceLoginResponse sdk.DeviceLoginResponse
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
	url := core.GetBaseURL() + "/oauth/token"

	payload := sdk.DeviceLoginFinalizeRequest{
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
		var errorResponse sdk.AuthErrorResponse
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

	var finalizeResponse sdk.DeviceLoginFinalizeResponse
	if err := json.Unmarshal(body, &finalizeResponse); err != nil {
		panic(err)
	}

	if res.StatusCode != http.StatusOK {
		// This is a real error, not just pending
		err := fmt.Errorf("authentication failed with status %d: %s", res.StatusCode, string(body))
		core.PrintError("Login", err)
		core.ExitWithError(err)
	}

	creds := sdk.Credentials{
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
			err := fmt.Errorf("no workspaces are available for your account.\nVisit https://app.blaxel.ai to create one")
			core.PrintError("Login", err)
			core.ExitWithError(err)
			return
		}

		// Get workspaces the user is already connected to
		connectedWorkspaces := sdk.ListWorkspaces()
		connectedWorkspaceSet := make(map[string]bool)
		for _, ws := range connectedWorkspaces {
			connectedWorkspaceSet[ws] = true
		}

		// If only one workspace, use it directly
		if len(workspaces) == 1 {
			workspace = *workspaces[0].Name
		} else {
			// Create options for huh form
			options := make([]huh.Option[string], 0, len(workspaces))
			for _, ws := range workspaces {
				if ws.Name == nil {
					continue
				}
				displayName := *ws.Name
				if connectedWorkspaceSet[*ws.Name] {
					displayName = fmt.Sprintf("%s (already connected)", *ws.Name)
				}
				options = append(options, huh.NewOption(displayName, *ws.Name))
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

		sdk.SaveCredentials(workspace, creds)
		sdk.SetCurrentWorkspace(workspace)
		core.PrintSuccess(fmt.Sprintf("Successfully logged in to workspace %s", workspace))
		return
	}

	err = validateWorkspace(workspace, creds)
	if err != nil {
		core.PrintError("Login", fmt.Errorf("error accessing workspace %s : %w", workspace, err))
	} else {
		sdk.SaveCredentials(workspace, creds)
		sdk.SetCurrentWorkspace(workspace)
		core.PrintSuccess(fmt.Sprintf("Successfully logged in to workspace %s", workspace))
	}
}
