package cli

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/blaxel-ai/toolkit/cli/core"
	"github.com/spf13/cobra"
)

func init() {
	core.RegisterCommand("secret", func() *cobra.Command {
		return SecretCmd()
	})
}

// workspaceSecret is the metadata the controlplane returns for a workspace
// secret; the value is write-only and never comes back.
type workspaceSecret struct {
	Name         string   `json:"name"`
	Destinations []string `json:"destinations"`
	CreatedAt    string   `json:"createdAt,omitempty"`
	UpdatedAt    string   `json:"updatedAt,omitempty"`
}

type workspaceSecretWrite struct {
	Name         string   `json:"name"`
	Value        string   `json:"value"`
	Destinations []string `json:"destinations"`
}

func secretResource() core.Resource {
	return core.Resource{
		Kind:     "Secret",
		Short:    "sec",
		Plural:   "secrets",
		Singular: "secret",
		Fields: []core.Field{
			{Key: "NAME", Value: "name"},
			{Key: "DESTINATIONS", Value: "destinations"},
			{Key: "CREATED_AT", Value: "createdAt", Special: "date"},
			{Key: "UPDATED_AT", Value: "updatedAt", Special: "date"},
		},
	}
}

func SecretCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "secret",
		Aliases: []string{"secrets"},
		Short:   "Manage workspace secrets injected by the sandbox proxy",
		Long: `Manage workspace secrets.

A workspace secret is referenced from a sandbox proxy routing rule as
{{SECRET:name}} and injected by the proxy, so sandbox code never sees the
value. Each secret lists the destinations it may be sent to (a host,
optionally followed by a path glob, same syntax as routing destinations);
it is never substituted toward any other destination. Values are write-only:
they can be set or rotated but never read back.`,
		Example: `  # List the workspace secrets (names and destinations, never values)
  bl secret list

  # Create or rotate a secret scoped to one host
  bl secret set stripe-key --value sk_live_... --destination api.stripe.com

  # Read the value from stdin, scope to several destinations
  cat key.txt | bl secret set github-token -d api.github.com -d "uploads.github.com/repos/*"

  # Delete a secret
  bl secret delete stripe-key`,
	}
	cmd.AddCommand(SecretListCmd(), SecretSetCmd(), SecretDeleteCmd())
	return cmd
}

func SecretListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Aliases: []string{"ls"},
		Short:   "List workspace secrets",
		Long:    `List the workspace secrets with their destinations and dates. Values are never returned.`,
		Example: `  bl secret list
  bl secret list -o json`,
		Run: func(cmd *cobra.Command, args []string) {
			client := core.GetClient()
			if client == nil {
				err := fmt.Errorf("client not initialized, please run 'bl login' first")
				core.PrintError("Secret", err)
				core.ExitWithError(err)
			}
			var secrets []workspaceSecret
			if err := client.Get(context.Background(), "secrets", nil, &secrets); err != nil {
				core.PrintError("Secret list", err)
				core.ExitWithError(err)
			}
			items := make([]interface{}, 0, len(secrets))
			for _, s := range secrets {
				items = append(items, map[string]interface{}{
					"name":         s.Name,
					"destinations": strings.Join(s.Destinations, ", "),
					"createdAt":    s.CreatedAt,
					"updatedAt":    s.UpdatedAt,
				})
			}
			core.Output(secretResource(), items, core.GetOutputFormat())
		},
	}
}

func SecretSetCmd() *cobra.Command {
	var value string
	var destinations []string
	cmd := &cobra.Command{
		Use:   "set <name>",
		Short: "Create or rotate a workspace secret",
		Long: `Create a workspace secret, or rotate the value of an existing one.

The value comes from --value, or from stdin when --value is omitted (a
trailing newline is dropped). At least one --destination is required: a host
such as api.stripe.com, optionally followed by a path glob such as
api.example.com/v1/*. The secret is only injected toward those destinations.`,
		Example: `  bl secret set stripe-key --value sk_live_... --destination api.stripe.com
  echo -n "$TOKEN" | bl secret set github-token -d api.github.com -d api.github.com/repos/*
  bl secret set multi -d "api.a.com,api.b.com/v2/*" --value ...`,
		Args: cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			name := args[0]
			dests := parseDestinations(destinations)
			if len(dests) == 0 {
				err := core.MarkExpectedError(fmt.Errorf("at least one --destination is required (e.g. --destination api.stripe.com)"), core.CLIErrorValidation)
				core.PrintError("Secret set", err)
				core.ExitWithError(err)
			}
			if value == "" {
				v, err := readSecretValue(cmd.InOrStdin())
				if err != nil {
					core.PrintError("Secret set", err)
					core.ExitWithError(err)
				}
				value = v
			}
			if value == "" {
				err := core.MarkExpectedError(fmt.Errorf("a value is required: pass --value or pipe it on stdin"), core.CLIErrorValidation)
				core.PrintError("Secret set", err)
				core.ExitWithError(err)
			}
			client := core.GetClient()
			if client == nil {
				err := fmt.Errorf("client not initialized, please run 'bl login' first")
				core.PrintError("Secret", err)
				core.ExitWithError(err)
			}
			var out workspaceSecret
			body := workspaceSecretWrite{Name: name, Value: value, Destinations: dests}
			if err := client.Post(context.Background(), "secrets", body, &out); err != nil {
				core.PrintError("Secret set", err)
				core.ExitWithError(err)
			}
			outputFormat := core.GetOutputFormat()
			if outputFormat == "json" || outputFormat == "yaml" {
				core.Output(secretResource(), []interface{}{map[string]interface{}{
					"name": out.Name, "destinations": out.Destinations, "createdAt": out.CreatedAt, "updatedAt": out.UpdatedAt,
				}}, outputFormat)
				return
			}
			core.PrintSuccess(fmt.Sprintf("Secret '%s' set for %s", out.Name, strings.Join(out.Destinations, ", ")))
		},
	}
	cmd.Flags().StringVar(&value, "value", "", "Secret value (read from stdin when omitted)")
	cmd.Flags().StringArrayVarP(&destinations, "destination", "d", nil, "Destination the secret may be sent to: host or host/path glob (repeatable, or comma-separated)")
	return cmd
}

func SecretDeleteCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "delete <name>",
		Aliases: []string{"rm"},
		Short:   "Delete a workspace secret",
		Long:    `Delete every version of a workspace secret and remove it from the proxy in every region.`,
		Example: `  bl secret delete stripe-key`,
		Args:    cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {
			client := core.GetClient()
			if client == nil {
				err := fmt.Errorf("client not initialized, please run 'bl login' first")
				core.PrintError("Secret", err)
				core.ExitWithError(err)
			}
			var out workspaceSecret
			if err := client.Delete(context.Background(), "secrets/"+args[0], nil, &out); err != nil {
				core.PrintError("Secret delete", err)
				core.ExitWithError(err)
			}
			core.PrintSuccess(fmt.Sprintf("Secret '%s' deleted", args[0]))
		},
	}
}

// parseDestinations accepts repeated flags and comma-separated lists, trims
// whitespace and drops empty entries.
func parseDestinations(flags []string) []string {
	out := make([]string, 0, len(flags))
	for _, f := range flags {
		for _, d := range strings.Split(f, ",") {
			if d = strings.TrimSpace(d); d != "" {
				out = append(out, d)
			}
		}
	}
	return out
}

// readSecretValue reads the whole of stdin and drops one trailing newline.
func readSecretValue(in io.Reader) (string, error) {
	b, err := io.ReadAll(bufio.NewReader(in))
	if err != nil {
		return "", fmt.Errorf("read value from stdin: %w", err)
	}
	return strings.TrimSuffix(strings.TrimSuffix(string(b), "\n"), "\r"), nil
}
