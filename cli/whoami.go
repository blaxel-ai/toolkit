package cli

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	blaxel "github.com/blaxel-ai/sdk-go"
	"github.com/blaxel-ai/sdk-go/option"
	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func init() {
	core.RegisterCommand("whoami", func() *cobra.Command {
		return WhoamiCmd()
	})
}

const (
	whoamiDeviceLogin       = "device_login"
	whoamiAPIKey            = "api_key"
	whoamiClientCredentials = "client_credentials"

	whoamiUser           = "user"
	whoamiServiceAccount = "service_account"

	whoamiConfigFile = "~/.blaxel/config.yaml"
)

// whoamiTimeout bounds the API check so bl whoami answers even when the API
// is unreachable.
var whoamiTimeout = 15 * time.Second

// whoamiReport is what bl whoami prints. It never holds a token or key.
type whoamiReport struct {
	// LoggedIn is false when there are no credentials or the API rejected them.
	LoggedIn bool `json:"logged_in" yaml:"logged_in"`
	// AccountType is user or service_account, empty when unknown.
	AccountType string `json:"account_type,omitempty" yaml:"account_type,omitempty"`
	Email       string `json:"email,omitempty" yaml:"email,omitempty"`
	Name        string `json:"name,omitempty" yaml:"name,omitempty"`
	ClientID    string `json:"client_id,omitempty" yaml:"client_id,omitempty"`
	Workspace   string `json:"workspace" yaml:"workspace"`
	// AuthMethod is device_login, api_key or client_credentials.
	AuthMethod        string `json:"auth_method,omitempty" yaml:"auth_method,omitempty"`
	CredentialsSource string `json:"credentials_source,omitempty" yaml:"credentials_source,omitempty"`
	APIURL            string `json:"api_url" yaml:"api_url"`
	// Verified is true when the API accepted the credentials.
	Verified bool   `json:"verified" yaml:"verified"`
	Error    string `json:"error,omitempty" yaml:"error,omitempty"`
	NextStep string `json:"next_step,omitempty" yaml:"next_step,omitempty"`
}

type whoamiProfile struct {
	Email      string `json:"email"`
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
}

func WhoamiCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show which account and workspace the CLI is logged in as",
		Long: `Show who the CLI is logged in as: the user or service account, the
current workspace, the authentication method (device login, API key or client
credentials) and the API URL.

bl whoami reads the saved login, or BL_API_KEY / BL_CLIENT_CREDENTIALS, and
makes one API call to confirm it and look up the account. If that call fails,
it still prints what it knows locally and says what failed. It never prints
tokens or keys.

It exits with status 1 when you are not logged in or the API could not confirm
the login, and tells you what to run next. With -o json the output is always a
JSON object with a logged_in field.`,
		Example: `  # Show the current login
  bl whoami

  # Machine-readable output for scripts and agents
  bl whoami -o json

  # Check the login for another workspace
  bl whoami -w my-workspace`,
		Args: cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			if code := runWhoami(ctx, cmd.OutOrStdout(), core.GetWorkspace(), core.GetOutputFormat()); code != 0 {
				core.Exit(code)
			}
		},
	}
}

// runWhoami prints the report and returns the exit code: 0 only when the API
// confirmed the login.
func runWhoami(ctx context.Context, out io.Writer, workspace, format string) int {
	report := resolveWhoami(ctx, workspace)
	if err := writeWhoami(out, report, format); err != nil {
		core.PrintError("whoami", err)
		return 1
	}
	if report.LoggedIn && report.Verified {
		return 0
	}
	return 1
}

func resolveWhoami(ctx context.Context, workspace string) whoamiReport {
	blaxel.InitializeEnvironment(workspace)
	report := whoamiReport{Workspace: workspace, APIURL: blaxel.GetBaseURL()}

	method, source, creds := whoamiCredentials(workspace)
	if method == "" {
		report.Error = "not logged in"
		report.NextStep = "bl login"
		if workspace != "" {
			report.Error = fmt.Sprintf("no saved login for workspace %s", workspace)
			report.NextStep = "bl login " + workspace
		}
		return report
	}
	report.LoggedIn = true
	report.AuthMethod = method
	report.CredentialsSource = source
	whoamiLocalIdentity(&report, creds)

	client, err := blaxel.NewClientFromConfig(workspace)
	if err != nil {
		report.Error = "could not create the API client: " + firstLine(err.Error())
		return report
	}
	ctx, cancel := context.WithTimeout(ctx, whoamiTimeout)
	defer cancel()
	verifyWhoami(ctx, client, &report)
	return report
}

// whoamiCredentials finds the credentials the CLI uses, in the same order as
// core.ResolveAuthSource: the login saved for the workspace wins over
// BL_API_KEY and BL_CLIENT_CREDENTIALS.
func whoamiCredentials(workspace string) (method, source string, creds blaxel.Credentials) {
	if workspace != "" {
		saved, _ := blaxel.LoadCredentials(workspace)
		switch {
		case saved.APIKey != "":
			return whoamiAPIKey, whoamiConfigFile, saved
		case saved.AccessToken != "" || saved.RefreshToken != "":
			return whoamiDeviceLogin, whoamiConfigFile, saved
		case saved.ClientCredentials != "":
			return whoamiClientCredentials, whoamiConfigFile, saved
		}
	}
	if key := os.Getenv("BL_API_KEY"); key != "" {
		return whoamiAPIKey, "BL_API_KEY", blaxel.Credentials{APIKey: key}
	}
	if clientCredentials := os.Getenv("BL_CLIENT_CREDENTIALS"); clientCredentials != "" {
		return whoamiClientCredentials, "BL_CLIENT_CREDENTIALS", blaxel.Credentials{ClientCredentials: clientCredentials}
	}
	return "", "", blaxel.Credentials{}
}

// whoamiLocalIdentity fills in what the credentials say without the API: the
// claims of a device login token, or the client ID of client credentials.
// An API key says nothing about its owner.
func whoamiLocalIdentity(report *whoamiReport, creds blaxel.Credentials) {
	switch report.AuthMethod {
	case whoamiDeviceLogin:
		claims := jwtClaims(creds.AccessToken)
		if claims.SubType == whoamiServiceAccount {
			report.AccountType = whoamiServiceAccount
			report.ClientID = claims.Sub
			return
		}
		if claims.Email != "" {
			report.AccountType = whoamiUser
			report.Email = claims.Email
			report.Name = fullName(claims.GivenName, claims.FamilyName)
		}
	case whoamiClientCredentials:
		report.AccountType = whoamiServiceAccount
		report.ClientID = clientIDFromCredentials(creds.ClientCredentials)
	}
}

// verifyWhoami makes the API call. A user (or an API key, which may belong to
// one) reads its profile. A service account has no profile, so it checks that
// it can read its workspace instead.
func verifyWhoami(ctx context.Context, client *blaxel.Client, report *whoamiReport) {
	if report.AccountType == whoamiServiceAccount {
		verifyWhoamiWorkspace(ctx, client, report)
		return
	}

	var profile whoamiProfile
	err := client.Get(ctx, "profile", nil, &profile, option.WithMaxRetries(1))
	if err == nil {
		report.Verified = true
		report.AccountType = whoamiUser
		if profile.Email != "" {
			report.Email = profile.Email
			report.Name = fullName(profile.GivenName, profile.FamilyName)
		}
		return
	}
	// An API key that belongs to a service account cannot read a profile.
	// If it can read the workspace, the key is valid and is a service account's.
	if report.AuthMethod == whoamiAPIKey && whoamiRejected(err) {
		// Without a workspace there is nothing to check a service account's
		// key against, so the refusal does not prove the key is invalid.
		if report.Workspace == "" {
			report.Error = "the API key cannot read a user profile and no workspace is selected, so it could not be checked as a service account's key"
			report.NextStep = "bl whoami -w WORKSPACE"
			return
		}
		if _, wsErr := client.Workspaces.Get(ctx, report.Workspace, blaxel.WorkspaceGetParams{}, option.WithMaxRetries(1)); wsErr == nil {
			report.Verified = true
			report.AccountType = whoamiServiceAccount
			return
		}
	}
	whoamiFailed(report, err)
}

func verifyWhoamiWorkspace(ctx context.Context, client *blaxel.Client, report *whoamiReport) {
	if report.Workspace == "" {
		report.Error = "no workspace selected, so the service account could not be checked"
		report.NextStep = "bl whoami -w WORKSPACE"
		return
	}
	if _, err := client.Workspaces.Get(ctx, report.Workspace, blaxel.WorkspaceGetParams{}, option.WithMaxRetries(1)); err != nil {
		whoamiFailed(report, err)
		return
	}
	report.Verified = true
}

// whoamiFailed records a failed API check. Rejected credentials mean the user
// is not logged in any more; any other failure leaves the local answer
// standing, unconfirmed.
func whoamiFailed(report *whoamiReport, err error) {
	var apiErr *blaxel.Error
	isAPIErr := errors.As(err, &apiErr)
	switch {
	case whoamiRejected(err):
		report.LoggedIn = false
		report.Error = "the API rejected the credentials"
		if isAPIErr {
			report.Error += fmt.Sprintf(" (%d %s)", apiErr.StatusCode, http.StatusText(apiErr.StatusCode))
		}
		report.NextStep = "bl login"
		if report.Workspace != "" {
			report.NextStep = "bl login " + report.Workspace
		}
	case isAPIErr:
		report.Error = fmt.Sprintf("the API returned %d %s", apiErr.StatusCode, http.StatusText(apiErr.StatusCode))
	default:
		report.Error = "the API check failed: " + firstLine(err.Error())
	}
}

func whoamiRejected(err error) bool {
	var apiErr *blaxel.Error
	if errors.As(err, &apiErr) {
		return apiErr.StatusCode == http.StatusUnauthorized || apiErr.StatusCode == http.StatusForbidden
	}
	// Token refresh and client credentials exchange fail with plain errors.
	msg := strings.ToLower(err.Error())
	for _, marker := range []string{"authentication failed", "status 401", "status 403", "http 401", "http 403"} {
		if strings.Contains(msg, marker) {
			return true
		}
	}
	return core.IsAuthError(err)
}

func writeWhoami(out io.Writer, report whoamiReport, format string) error {
	switch format {
	case "json":
		data, err := json.MarshalIndent(report, "", "  ")
		if err != nil {
			return err
		}
		_, err = fmt.Fprintln(out, string(data))
		return err
	case "yaml":
		data, err := yaml.Marshal(report)
		if err != nil {
			return err
		}
		_, err = out.Write(data)
		return err
	}

	var b strings.Builder
	line := func(label, value string) {
		fmt.Fprintf(&b, "%-17s %s\n", label+":", value)
	}
	switch {
	case report.LoggedIn && report.Verified:
		b.WriteString("Logged in to Blaxel.\n")
	case report.LoggedIn:
		b.WriteString("Logged in to Blaxel, but the API could not confirm it.\n")
	default:
		b.WriteString("Not logged in.\n")
	}

	if report.AuthMethod != "" {
		label, who := "Account", "unknown"
		switch report.AccountType {
		case whoamiUser:
			label, who = "User", report.Email
			if report.Name != "" {
				who += " (" + report.Name + ")"
			}
		case whoamiServiceAccount:
			label, who = "Service account", "client ID "+report.ClientID
			if report.ClientID == "" {
				who = "name not available with an API key"
			}
		}
		if who != "unknown" && !report.Verified {
			who += ", not confirmed by the API"
		}
		line(label, who)
	}

	workspace := report.Workspace
	if workspace == "" {
		workspace = "none selected"
	}
	line("Workspace", workspace)
	if report.AuthMethod != "" {
		line("Auth method", whoamiMethodLabel(report.AuthMethod)+" ("+whoamiSourceLabel(report.CredentialsSource)+")")
	}
	line("API URL", report.APIURL)
	if report.Error != "" && report.Error != "not logged in" {
		line("Error", report.Error)
	}
	if report.NextStep != "" {
		line("Next step", report.NextStep)
	}
	_, err := io.WriteString(out, b.String())
	return err
}

func whoamiMethodLabel(method string) string {
	switch method {
	case whoamiDeviceLogin:
		return "device login"
	case whoamiAPIKey:
		return "API key"
	case whoamiClientCredentials:
		return "client credentials"
	}
	return method
}

func whoamiSourceLabel(source string) string {
	if source == whoamiConfigFile {
		return "saved in " + source
	}
	return "from " + source
}

type tokenClaims struct {
	Sub        string `json:"sub"`
	SubType    string `json:"sub_type"`
	Email      string `json:"email"`
	GivenName  string `json:"given_name"`
	FamilyName string `json:"family_name"`
}

// jwtClaims reads a token's claims without verifying it; whoami only displays
// them and confirms the login with the API.
func jwtClaims(token string) tokenClaims {
	var claims tokenClaims
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return claims
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return claims
	}
	_ = json.Unmarshal(payload, &claims)
	return claims
}

// clientIDFromCredentials returns the public half of base64(clientId:secret).
func clientIDFromCredentials(credentials string) string {
	decoded, err := base64.StdEncoding.DecodeString(credentials)
	if err != nil {
		decoded = []byte(credentials)
	}
	clientID, _, found := strings.Cut(string(decoded), ":")
	if !found {
		return ""
	}
	return clientID
}

func fullName(given, family string) string {
	return strings.TrimSpace(given + " " + family)
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return strings.TrimSpace(line)
}
