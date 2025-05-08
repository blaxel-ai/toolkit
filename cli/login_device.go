package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"time"

	"github.com/blaxel-ai/toolkit/sdk"
)

func (r *Operations) DeviceModeLogin(workspace string) {
	url := r.BaseURL + "/login/device"

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
	defer res.Body.Close()

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

	r.DeviceModeLoginFinalize(deviceLoginResponse.DeviceCode, workspace)
}

func (r *Operations) DeviceModeLoginFinalize(deviceCode string, workspace string) {
	time.Sleep(3 * time.Second)
	url := r.BaseURL + "/oauth/token"

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
	defer res.Body.Close()

	body, _ := io.ReadAll(res.Body)

	var finalizeResponse sdk.DeviceLoginFinalizeResponse
	if err := json.Unmarshal(body, &finalizeResponse); err != nil {
		panic(err)
	}

	if res.StatusCode != http.StatusOK {
		r.DeviceModeLoginFinalize(deviceCode, workspace)
	}

	creds := sdk.Credentials{
		AccessToken:  finalizeResponse.AccessToken,
		RefreshToken: finalizeResponse.RefreshToken,
		ExpiresIn:    finalizeResponse.ExpiresIn,
		DeviceCode:   deviceCode,
	}

	_, err = CheckWorkspaceAccess(workspace, creds)
	if err != nil {
		fmt.Printf("Error accessing workspace %s : %s\n", workspace, err)
	} else {
		sdk.SaveCredentials(workspace, creds)
		sdk.SetCurrentWorkspace(workspace)
		fmt.Println("Successfully logged in")
	}
}
