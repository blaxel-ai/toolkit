package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("secret", func() *cobra.Command {
		return SecretCmd()
	})
}

func SecretCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secret",
		Short: "Manage workspace secrets",
		Long: `Manage workspace secrets.

A workspace secret is a named value that is encrypted at rest and can never be
read back once set. Reference it from a sandbox's agent-proxy routing rules with
` + "`{{SECRET:name}}`" + `: the proxy injects the latest value into outbound requests at
runtime, so the credential never reaches the sandbox itself.

Setting a secret that already exists replaces its value for new requests.`,
		Example: `  # Set a secret from a flag, an environment variable, a file or stdin
  bl secret set OPENAI_API_KEY --value sk-...
  bl secret set OPENAI_API_KEY --from-env OPENAI_API_KEY
  bl secret set TLS_CERT --from-file ./cert.pem
  cat token.txt | bl secret set GITHUB_TOKEN

  # List secret names (values are never shown)
  bl secret list

  # Delete a secret
  bl secret delete OPENAI_API_KEY`,
	}
	cmd.AddCommand(SecretSetCmd(), SecretListCmd(), SecretDeleteCmd())
	return cmd
}

func SecretSetCmd() *cobra.Command {
	var value, fromEnv, fromFile string
	cmd := &cobra.Command{
		Use:   "set NAME",
		Short: "Create or update a workspace secret",
		Long: `Create or update a workspace secret.

The value comes from exactly one of --value, --from-env, --from-file, or stdin
when none of the flags is given. Prefer --from-env, --from-file or stdin over
--value so the secret does not end up in your shell history.`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			v, err := readSecretValue(cmd.Flags().Changed("value"), value, fromEnv, fromFile, cmd.InOrStdin())
			if err != nil {
				core.PrintError("Secret", err)
				core.ExitWithError(core.MarkExpectedError(err, core.CLIErrorValidation))
			}
			c := core.GetClient()
			if c == nil {
				core.ExitWithError(fmt.Errorf("not logged in"))
			}
			out, err := core.SetWorkspaceSecret(context.Background(), c, args[0], v)
			if err != nil {
				core.PrintError("Secret", err)
				core.ExitWithError(err)
			}
			if f := core.GetOutputFormat(); f == "json" || f == "yaml" {
				outputDriveData(out, f)
				return
			}
			core.PrintSuccess(fmt.Sprintf("Secret %s set", out.Name))
		},
	}
	cmd.Flags().StringVar(&value, "value", "", "Secret value (visible in shell history; prefer --from-env, --from-file or stdin)")
	cmd.Flags().StringVar(&fromEnv, "from-env", "", "Read the value from this environment variable")
	cmd.Flags().StringVar(&fromFile, "from-file", "", "Read the value from this file")
	return cmd
}

func SecretListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List workspace secrets (names only, never values)",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			for _, r := range core.GetResources() {
				if r.Kind == "Secret" {
					ListFn(r)
					return
				}
			}
		},
	}
}

func SecretDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "delete NAME",
		Short: "Delete a workspace secret and all its versions",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			for _, r := range core.GetResources() {
				if r.Kind == "Secret" {
					if err := DeleteFn(r, args[0]); err != nil {
						core.ExitWithError(err)
					}
					return
				}
			}
		},
	}
}

// readSecretValue picks the single configured value source. Trailing newlines
// from files and pipes are trimmed so `echo value | bl secret set` works.
func readSecretValue(valueSet bool, value, fromEnv, fromFile string, stdin io.Reader) (string, error) {
	sources := 0
	for _, set := range []bool{valueSet, fromEnv != "", fromFile != ""} {
		if set {
			sources++
		}
	}
	if sources > 1 {
		return "", fmt.Errorf("use only one of --value, --from-env or --from-file")
	}
	var v string
	switch {
	case valueSet:
		v = value
	case fromEnv != "":
		var ok bool
		v, ok = os.LookupEnv(fromEnv)
		if !ok {
			return "", fmt.Errorf("environment variable %s is not set", fromEnv)
		}
	case fromFile != "":
		b, err := os.ReadFile(fromFile)
		if err != nil {
			return "", fmt.Errorf("read %s: %w", fromFile, err)
		}
		v = strings.TrimRight(string(b), "\r\n")
	default:
		b, err := io.ReadAll(stdin)
		if err != nil {
			return "", fmt.Errorf("read stdin: %w", err)
		}
		v = strings.TrimRight(string(b), "\r\n")
	}
	if v == "" {
		return "", fmt.Errorf("secret value is empty")
	}
	return v, nil
}
