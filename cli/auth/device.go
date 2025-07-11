package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/blaxel-ai/toolkit/sdk"
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
		fmt.Printf("Error making request: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = res.Body.Close() }()

	body, _ := io.ReadAll(res.Body)

	var deviceLoginResponse sdk.DeviceLoginResponse
	if err := json.Unmarshal(body, &deviceLoginResponse); err != nil {
		fmt.Printf("Error unmarshalling response: %v\n", err)
		os.Exit(1)
	}

	// Open the URL in the default browser
	err = exec.Command("open", deviceLoginResponse.VerificationURIComplete+"&workspace="+workspace).Start()
	if err != nil {
		fmt.Printf("Please visit the following URL to finish logging in: %s\n", deviceLoginResponse.VerificationURIComplete)
	} else {
		fmt.Println("Opened URL in browser. If it's not working, please open it manually: ", deviceLoginResponse.VerificationURIComplete)
	}
	fmt.Println("Waiting for user to finish login...")

	deviceModeLoginFinalize(deviceLoginResponse.DeviceCode, workspace, 3)
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
		fmt.Printf("Error making request: %v\n", err)
		os.Exit(1)
	}
	defer func() { _ = res.Body.Close() }()

	body, _ := io.ReadAll(res.Body)

	var finalizeResponse sdk.DeviceLoginFinalizeResponse
	if err := json.Unmarshal(body, &finalizeResponse); err != nil {
		panic(err)
	}

	if res.StatusCode != http.StatusOK {
		if retries > 0 {
			deviceModeLoginFinalize(deviceCode, workspace, retries-1)
		} else {
			fmt.Printf("Error logging in: %d -> %s\n", res.StatusCode, string(body))
			os.Exit(1)
		}
	}

	creds := sdk.Credentials{
		AccessToken:  finalizeResponse.AccessToken,
		RefreshToken: finalizeResponse.RefreshToken,
		ExpiresIn:    finalizeResponse.ExpiresIn,
		DeviceCode:   deviceCode,
	}

	err = validateWorkspace(workspace, creds)
	if err != nil {
		fmt.Printf("Error accessing workspace %s : %s\n", workspace, err)
	} else {
		sdk.SaveCredentials(workspace, creds)
		sdk.SetCurrentWorkspace(workspace)
		fmt.Println("Successfully logged in")
	}
}
